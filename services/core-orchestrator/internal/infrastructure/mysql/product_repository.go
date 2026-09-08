package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrProductNotFound = errors.New("product not found")

type ProductDTO struct {
	ID               int64      `json:"id"`
	SKU              string     `json:"sku"`
	PartNumber       string     `json:"partNumber"`
	Name             string     `json:"name"`
	Description      *string    `json:"description,omitempty"`
	ShortDescription *string    `json:"shortDescription,omitempty"`
	BrandID          *int64     `json:"brandId,omitempty"`
	CategoryID       *int64     `json:"categoryId,omitempty"`
	ProductType      string     `json:"productType"`
	Status           *string    `json:"status,omitempty"`
	IsSellable       bool       `json:"isSellable"`
	IsStockable      bool       `json:"isStockable"`
	SourceID         int64      `json:"sourceId"`
	CreatedBy        int64      `json:"createdBy"`
	UpdatedBy        *int64     `json:"updatedBy,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	DeletedAt        *time.Time `json:"deletedAt,omitempty"`
	// ImageURL is the product's cover image (ecom_product_images.is_first),
	// resolved to a browser-usable URL. Only FindPaginated populates it —
	// nil when the product has no image marked as first, or on FindByID.
	ImageURL *string `json:"imageUrl,omitempty"`
	// BrandName is ecom_brands.name for the product's brand_id (nil when the
	// product has no brand). Only FindPaginated fills it.
	BrandName *string `json:"brandName,omitempty"`
	// TotalStock is the sum of available_qty across every ecom_product_stock
	// row for the product (0 when it has none). Only FindPaginated fills it.
	TotalStock int64 `json:"totalStock"`
	// Price is the product's price from the highest-priority active/valid
	// price list (same rule as ProductPricesRepository.FindEffectivePrice),
	// with the currency that price is stored in. nil when the product has no
	// such price. Only FindPaginated fills it.
	Price *ProductListPrice `json:"price,omitempty"`
	// Channels lists the distinct channel connections (ecom_channel_connections)
	// the product is mapped to in ecom_channel_product_map — deduplicated by
	// connection, so a connection with allows_multiple_listings and several
	// listings for this product still appears once. Excludes 'closed' and
	// soft-deleted mappings. Always a slice (empty when the product is in no
	// channel). FindPaginated only.
	Channels []ProductChannelBadge `json:"channels"`
}

type ProductListPrice struct {
	Amount         string `json:"amount"`
	CurrencyCode   string `json:"currencyCode"`
	CurrencySymbol string `json:"currencySymbol"`
}

type ProductChannelBadge struct {
	ConnectionID int64  `json:"connectionId"`
	Code         string `json:"code"`
	Name         string `json:"name"`
}

type PaginatedProducts struct {
	Total    int64        `json:"total"`
	Offset   int          `json:"offset"`
	PageSize int          `json:"pageSize"`
	Products []ProductDTO `json:"products"`
}

type CreateProductInput struct {
	SKU              string
	PartNumber       string
	Name             string
	Description      *string
	ShortDescription *string
	BrandID          *int64
	CategoryID       *int64
	ProductType      string
	Status           *string
	IsSellable       bool
	IsStockable      bool
	SourceID         int64
	CreatedBy        int64
}

type UpdateProductInput struct {
	SKU              string
	PartNumber       string
	Name             string
	Description      *string
	ShortDescription *string
	BrandID          *int64
	CategoryID       *int64
	ProductType      string
	Status           *string
	IsSellable       bool
	IsStockable      bool
	SourceID         int64
	UpdatedBy        int64
}

type ProductRepository struct {
	db Querier
}

func NewProductRepository(db Querier) *ProductRepository {
	return &ProductRepository{db: db}
}

const productColumns = `
	id, sku, part_number, name, description, short_description, brand_id, category_id,
	product_type, status, is_sellable, is_stockable, source_id,
	created_by, updated_by, created_at, updated_at, deleted_at
`

// productSortColumns whitelists the API-facing sort keys FindPaginated
// accepts and maps each to the column (or SELECT-list alias) ORDER BY uses.
// Column names/directions can't be parameterized with `?`, so ORDER BY is
// built from this fixed map instead of the caller's raw input — never
// interpolate an unvalidated sortBy/sortDir into the query string.
// stock/price/brand sort by the computed columns added in FindPaginated.
var productSortColumns = map[string]string{
	"sku":        "sku",
	"partNumber": "part_number",
	"name":       "name",
	"brand":      "brand_name",
	"stock":      "total_stock",
	"price":      "price_amount",
}

func (r *ProductRepository) FindPaginated(ctx context.Context, offset, pageSize int, sortBy, sortDir, search string, connectionID *int64, pendingOnly bool) (*PaginatedProducts, error) {
	// search matches sku, part_number or name (case-insensitive via the
	// column collation). % and _ in the term are escaped so they're treated
	// as literals, not wildcards.
	filterClause := "WHERE deleted_at IS NULL"
	var filterArgs []any
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(trimmed)
		like := "%" + escaped + "%"
		filterClause += " AND (sku LIKE ? OR part_number LIKE ? OR name LIKE ?)"
		filterArgs = append(filterArgs, like, like, like)
	}
	if connectionID != nil {
		filterClause += ` AND EXISTS (
			SELECT 1 FROM ecom_channel_product_map cpm2
			WHERE cpm2.product_id = ecom_products.id
			  AND cpm2.connection_id = ?
			  AND cpm2.deleted_at IS NULL
			  AND cpm2.status <> 'closed'
		)`
		filterArgs = append(filterArgs, *connectionID)
	}
	if pendingOnly {
		// Pending = ready to be prepared for a channel sync: has stock and at
		// least one image. Not tied to any particular connection or mapping
		// status.
		filterClause += ` AND COALESCE((
			SELECT SUM(ps2.available_qty) FROM ecom_product_stock ps2
			WHERE ps2.product_id = ecom_products.id
		), 0) > 0
		AND EXISTS (
			SELECT 1 FROM ecom_product_images pi2
			WHERE pi2.product_id = ecom_products.id AND pi2.deleted_at IS NULL
		)`
	}

	var total int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ecom_products "+filterClause, filterArgs...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting products: %w", err)
	}

	orderBy := "id ASC"
	if column, ok := productSortColumns[sortBy]; ok {
		direction := "ASC"
		if strings.EqualFold(sortDir, "desc") {
			direction = "DESC"
		}
		// Tie-break on id so pagination stays stable across pages when the
		// sorted column has duplicate values.
		orderBy = column + " " + direction + ", id ASC"
	}

	// The extra columns (cover image, brand name, total stock, effective
	// price) are resolved with correlated subqueries rather than JOINs so a
	// data bug (e.g. two images flagged is_first for one product) can never
	// duplicate the product row — each is bounded to a single result. The
	// three price_* columns all pick the same row: the highest-priority
	// active/valid price list (same rule as
	// ProductPricesRepository.FindEffectivePrice). Their aliases (brand_name,
	// total_stock, price_amount) are what productSortColumns sorts on.
	priceRow := `FROM ecom_product_prices pp
			   JOIN ecom_price_list pl ON pl.id = pp.price_list_id AND pl.deleted_at IS NULL
			   JOIN ecom_currencies cur ON cur.id = pp.currency
			   WHERE pp.product_id = ecom_products.id
			     AND pl.status = 'active' AND CURDATE() BETWEEN pl.valid_from AND pl.valid_to
			   ORDER BY pl.priority DESC, pp.updated_at DESC LIMIT 1`

	query := `
		SELECT ` + productColumns + `,
			(SELECT f.path
			   FROM ecom_product_images pi
			   JOIN ecom_files f ON f.id = pi.file_id AND f.deleted_at IS NULL
			   WHERE pi.product_id = ecom_products.id AND pi.is_first = 1 AND pi.deleted_at IS NULL
			   ORDER BY pi.id ASC LIMIT 1) AS image_path,
			(SELECT sd.base_url
			   FROM ecom_product_images pi
			   JOIN ecom_files f ON f.id = pi.file_id AND f.deleted_at IS NULL
			   JOIN ecom_storage_disks sd ON sd.id = f.disk_id AND sd.deleted_at IS NULL
			   WHERE pi.product_id = ecom_products.id AND pi.is_first = 1 AND pi.deleted_at IS NULL
			   ORDER BY pi.id ASC LIMIT 1) AS image_base_url,
			(SELECT b.name FROM ecom_brands b
			   WHERE b.id = ecom_products.brand_id AND b.deleted_at IS NULL) AS brand_name,
			CAST(COALESCE((SELECT SUM(ps.available_qty)
			   FROM ecom_product_stock ps
			   WHERE ps.product_id = ecom_products.id), 0) AS SIGNED) AS total_stock,
			(SELECT pp.price ` + priceRow + `) AS price_amount,
			(SELECT cur.code ` + priceRow + `) AS price_currency_code,
			(SELECT cur.symbol ` + priceRow + `) AS price_currency_symbol,
			(SELECT CONCAT('[', GROUP_CONCAT(DISTINCT JSON_OBJECT(
			       'connectionId', cc.id, 'code', ch.code, 'name', cc.name)), ']')
			   FROM ecom_channel_product_map cpm
			   JOIN ecom_channel_connections cc ON cc.id = cpm.connection_id AND cc.deleted_at IS NULL
			   JOIN ecom_channels ch ON ch.id = cc.channel_id AND ch.deleted_at IS NULL
			   WHERE cpm.product_id = ecom_products.id AND cpm.deleted_at IS NULL
			     AND cpm.status <> 'closed') AS channels_json
		FROM ecom_products
		` + filterClause + `
		ORDER BY ` + orderBy + `
		LIMIT ? OFFSET ?
	`

	queryArgs := append(append([]any{}, filterArgs...), pageSize, offset)
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("error querying products: %w", err)
	}
	defer rows.Close()

	products := make([]ProductDTO, 0)
	for rows.Next() {
		product, scanErr := scanProduct(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		products = append(products, product)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating products: %w", err)
	}

	return &PaginatedProducts{
		Total:    total,
		Offset:   offset,
		PageSize: pageSize,
		Products: products,
	}, nil
}

func (r *ProductRepository) FindByID(ctx context.Context, id int64) (*ProductDTO, error) {
	query := `
		SELECT ` + productColumns + `
		FROM ecom_products
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return scanProductRow(row)
}

func (r *ProductRepository) FindBySKU(ctx context.Context, sku string) (*ProductDTO, error) {
	query := `
		SELECT ` + productColumns + `
		FROM ecom_products
		WHERE sku = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, sku)
	return scanProductRow(row)
}

// FindBySourceAndPartNumber busca por part_number dentro de una fuente, no por sku — sku y
// part_number pueden ser distintos fuera del flujo de Nissan (donde hoy son iguales, ver
// createNissanProduct en sync_nissan.go), y la resolución de ecom_part_number_supersessions
// está definida en términos de part_number, no de sku. ecom_products ya NO tiene un unique key
// sobre (source_id, part_number) — dos productos pueden compartir part_number dentro del mismo
// source (sku sigue siendo el único identificador realmente único) — así que esto puede
// devolver cualquiera de varios; ORDER BY id ASC lo hace al menos determinístico (gana el más
// antiguo) en vez de depender del orden físico de InnoDB. Los llamadores que necesiten
// considerar a todos los productos que comparten el part_number deben usar
// FindAllBySourceAndPartNumber en su lugar.
func (r *ProductRepository) FindBySourceAndPartNumber(ctx context.Context, sourceID int64, partNumber string) (*ProductDTO, error) {
	query := `
		SELECT ` + productColumns + `
		FROM ecom_products
		WHERE source_id = ? AND part_number = ? AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, sourceID, partNumber)
	return scanProductRow(row)
}

// FindAllBySourceAndPartNumber returns every ecom_products row for a given
// (source_id, part_number) pair, oldest first — unlike FindBySourceAndPartNumber,
// which only ever returns one. Use this where more than one product sharing
// the part_number within the source must all be taken into account (e.g.
// resolveSuccessionChain in channel_listings, which enumerates every product
// linked through a part number supersession chain). Always a slice, never
// nil, empty when nothing matches.
func (r *ProductRepository) FindAllBySourceAndPartNumber(ctx context.Context, sourceID int64, partNumber string) ([]ProductDTO, error) {
	query := `
		SELECT ` + productColumns + `
		FROM ecom_products
		WHERE source_id = ? AND part_number = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, sourceID, partNumber)
	if err != nil {
		return nil, fmt.Errorf("error querying products by source and part number: %w", err)
	}
	defer rows.Close()

	products := make([]ProductDTO, 0)
	for rows.Next() {
		var p ProductDTO
		if err := rows.Scan(
			&p.ID, &p.SKU, &p.PartNumber, &p.Name, &p.Description, &p.ShortDescription,
			&p.BrandID, &p.CategoryID, &p.ProductType, &p.Status, &p.IsSellable, &p.IsStockable,
			&p.SourceID, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning product: %w", err)
		}
		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating products: %w", err)
	}

	return products, nil
}

// ReadyForListingProduct is the minimal projection ListingDiscoveryScheduler
// needs per product: its id, plus the category and brand ids it already
// resolved as non-null in the query (both are guaranteed present — the WHERE
// clause filters out products missing either).
type ReadyForListingProduct struct {
	ID         int64
	CategoryID int64
	BrandID    int64
}

// FindReadyForListingPage returns one keyset page (id > afterID, oldest first)
// of products that pass every "ready to publish" gate that isn't
// connection-specific: not soft-deleted, status 'active', sellable, has a
// category and a brand assigned, has total available stock > 0, and has at
// least one cover image (ecom_product_images.is_first = 1). The price gate
// (effective price > 0 in the connection's currency) is left to the caller —
// it depends on the connection's pricing formula. Pass afterID = 0 for the
// first page; keep calling with the last returned id until fewer than limit
// rows come back.
func (r *ProductRepository) FindReadyForListingPage(ctx context.Context, afterID int64, limit int) ([]ReadyForListingProduct, error) {
	query := `
		SELECT p.id, p.category_id, p.brand_id
		FROM ecom_products p
		WHERE p.deleted_at IS NULL
		  AND p.status = 'active'
		  AND p.is_sellable = 1
		  AND p.category_id IS NOT NULL
		  AND p.brand_id IS NOT NULL
		  AND p.id > ?
		  AND COALESCE((
		        SELECT SUM(ps.available_qty) FROM ecom_product_stock ps
		        WHERE ps.product_id = p.id
		      ), 0) > 0
		  AND EXISTS (
		        SELECT 1 FROM ecom_product_images pi
		        WHERE pi.product_id = p.id AND pi.is_first = 1 AND pi.deleted_at IS NULL
		      )
		ORDER BY p.id ASC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("error querying products ready for listing: %w", err)
	}
	defer rows.Close()

	ready := make([]ReadyForListingProduct, 0, limit)
	for rows.Next() {
		var p ReadyForListingProduct
		if err := rows.Scan(&p.ID, &p.CategoryID, &p.BrandID); err != nil {
			return nil, fmt.Errorf("error scanning product ready for listing: %w", err)
		}
		ready = append(ready, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating products ready for listing: %w", err)
	}

	return ready, nil
}

func (r *ProductRepository) Create(ctx context.Context, input CreateProductInput) (*ProductDTO, error) {
	query := `
		INSERT INTO ecom_products (
			sku, part_number, name, description, short_description, brand_id, category_id,
			product_type, status, is_sellable, is_stockable, source_id, created_by, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx,
		query,
		input.SKU, input.PartNumber, input.Name, input.Description, input.ShortDescription,
		input.BrandID, input.CategoryID, input.ProductType, input.Status,
		input.IsSellable, input.IsStockable, input.SourceID, input.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating product: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *ProductRepository) Update(ctx context.Context, id int64, input UpdateProductInput) (*ProductDTO, error) {
	query := `
		UPDATE ecom_products
		SET sku = ?, part_number = ?, name = ?, description = ?, short_description = ?,
			brand_id = ?, category_id = ?, product_type = ?, status = ?,
			is_sellable = ?, is_stockable = ?, source_id = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx,
		query,
		input.SKU, input.PartNumber, input.Name, input.Description, input.ShortDescription,
		input.BrandID, input.CategoryID, input.ProductType, input.Status,
		input.IsSellable, input.IsStockable, input.SourceID, input.UpdatedBy, id,
	)
	if err != nil {
		return nil, fmt.Errorf("error updating product: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return nil, ErrProductNotFound
	}

	return r.FindByID(ctx, id)
}

// UpdateCategoryID sets category_id alone, leaving every other column
// untouched — used when a marketplace sync assigns or replaces a product's
// category from the channel's own taxonomy (see
// sync.MercadoLibreProductSyncService.syncCategoryMapping) instead of a full
// product edit.
func (r *ProductRepository) UpdateCategoryID(ctx context.Context, id, categoryID, updatedBy int64) error {
	query := `
		UPDATE ecom_products
		SET category_id = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, categoryID, updatedBy, id)
	if err != nil {
		return fmt.Errorf("error updating product category: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrProductNotFound
	}

	return nil
}

// UpdateBrandID sets brand_id alone, leaving every other column untouched —
// used by brand bulk-assignment imports (see brands.BrandService.BulkAssignBrands).
func (r *ProductRepository) UpdateBrandID(ctx context.Context, id, brandID, updatedBy int64) error {
	query := `
		UPDATE ecom_products
		SET brand_id = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, brandID, updatedBy, id)
	if err != nil {
		return fmt.Errorf("error updating product brand: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrProductNotFound
	}

	return nil
}

// UpdateDescription sets description alone, leaving every other column
// untouched — used when a marketplace sync (see
// sync.MercadoLibreCompatibilityService's description sync) pulls a
// listing's description into the local product on the caller's explicit
// request, same convention as UpdateName.
func (r *ProductRepository) UpdateDescription(ctx context.Context, id int64, description string, updatedBy int64) error {
	query := `
		UPDATE ecom_products
		SET description = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, description, updatedBy, id)
	if err != nil {
		return fmt.Errorf("error updating product description: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrProductNotFound
	}

	return nil
}

// UpdateName sets name alone, leaving every other column untouched — used
// when a marketplace sync (see sync.MercadoLibreListingsAuditService)
// replaces a product's name with the marketplace's own listing title.
func (r *ProductRepository) UpdateName(ctx context.Context, id int64, name string, updatedBy int64) error {
	query := `
		UPDATE ecom_products
		SET name = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, name, updatedBy, id)
	if err != nil {
		return fmt.Errorf("error updating product name: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrProductNotFound
	}

	return nil
}

func (r *ProductRepository) SoftDelete(ctx context.Context, id int64) error {
	query := `
		UPDATE ecom_products
		SET deleted_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting product: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrProductNotFound
	}

	return nil
}

func scanProduct(rows *sql.Rows) (ProductDTO, error) {
	var p ProductDTO
	var imagePath, imageBaseURL, brandName sql.NullString
	var priceAmount, priceCurrencyCode, priceCurrencySymbol sql.NullString
	var channelsJSON sql.NullString
	err := rows.Scan(
		&p.ID, &p.SKU, &p.PartNumber, &p.Name, &p.Description, &p.ShortDescription,
		&p.BrandID, &p.CategoryID, &p.ProductType, &p.Status, &p.IsSellable, &p.IsStockable,
		&p.SourceID, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
		&imagePath, &imageBaseURL, &brandName, &p.TotalStock,
		&priceAmount, &priceCurrencyCode, &priceCurrencySymbol, &channelsJSON,
	)
	if err != nil {
		return ProductDTO{}, fmt.Errorf("error scanning product: %w", err)
	}
	p.ImageURL = buildFileURL(imageBaseURL.String, imagePath.String)
	if brandName.Valid {
		p.BrandName = &brandName.String
	}
	if priceAmount.Valid {
		p.Price = &ProductListPrice{
			Amount:         priceAmount.String,
			CurrencyCode:   priceCurrencyCode.String,
			CurrencySymbol: priceCurrencySymbol.String,
		}
	}
	p.Channels = make([]ProductChannelBadge, 0)
	if channelsJSON.Valid && channelsJSON.String != "" {
		// A malformed aggregate (e.g. group_concat_max_len truncation on a
		// product in an unusually large number of channels) is tolerated as
		// "no channels" rather than failing the whole listing.
		var parsed []ProductChannelBadge
		if json.Unmarshal([]byte(channelsJSON.String), &parsed) == nil {
			p.Channels = parsed
		}
	}
	return p, nil
}

// buildFileURL derives a browser-usable URL for an ecom_files row from its
// storage disk's base_url and its own path. product_image_import — today's
// source of product images, synced by URL from a marketplace/ERP — stores
// the complete external URL directly in path, so that case is returned as-is;
// anything else is treated as a path relative to the disk's base_url. Returns
// nil when there is no path (no cover image) or, for the relative case, no
// base_url to resolve it against.
func buildFileURL(baseURL, path string) *string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return &path
	}

	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return nil
	}

	url := base + "/" + strings.TrimLeft(path, "/")
	return &url
}

func scanProductRow(row *sql.Row) (*ProductDTO, error) {
	var p ProductDTO
	err := row.Scan(
		&p.ID, &p.SKU, &p.PartNumber, &p.Name, &p.Description, &p.ShortDescription,
		&p.BrandID, &p.CategoryID, &p.ProductType, &p.Status, &p.IsSellable, &p.IsStockable,
		&p.SourceID, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("error scanning product: %w", err)
	}
	return &p, nil
}
