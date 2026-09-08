package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ProductDetailsRepository is the read/write model behind the product detail
// page: one purpose-built query per section, each already resolving the foreign
// keys (brand, category, warehouse, currency, attribute, file...) into the names
// and URLs the UI shows. It deliberately does NOT reuse the write-path repos
// (ProductRepository, ProductPricesRepository, ...) — their DTOs and scan
// orders are consumed by the marketplace sync flows, and widening them for a
// UI concern would put those at risk. The only writes are the inline edits the
// detail page allows (UpdateGeneral); everything else is read-only.
type ProductDetailsRepository struct {
	db Querier
}

func NewProductDetailsRepository(db Querier) *ProductDetailsRepository {
	return &ProductDetailsRepository{db: db}
}

// --- General -----------------------------------------------------------------

type ProductGeneralDTO struct {
	ID               int64   `json:"id"`
	SKU              string  `json:"sku"`
	PartNumber       string  `json:"partNumber"`
	Name             string  `json:"name"`
	Description      *string `json:"description,omitempty"`
	ShortDescription *string `json:"shortDescription,omitempty"`
	ProductType      string  `json:"productType"`
	Status           *string `json:"status,omitempty"`
	IsSellable       bool    `json:"isSellable"`
	IsStockable      bool    `json:"isStockable"`
	BrandID          *int64  `json:"brandId,omitempty"`
	BrandName        *string `json:"brandName,omitempty"`
	CategoryID       *int64  `json:"categoryId,omitempty"`
	CategoryName     *string `json:"categoryName,omitempty"`
	// CategoryPath is the local category's full ancestry, root first
	// ("Refacciones / Motor / Filtros"), so the page can show where the
	// category sits in the tree instead of just its leaf name.
	CategoryPath *string   `json:"categoryPath,omitempty"`
	SourceID     int64     `json:"sourceId"`
	SourceName   *string   `json:"sourceName,omitempty"`
	CoverImage   *string   `json:"coverImage,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	CreatedBy    *string   `json:"createdBy,omitempty"`
	UpdatedBy    *string   `json:"updatedBy,omitempty"`
}

func (r *ProductDetailsRepository) FindGeneral(ctx context.Context, productID int64) (*ProductGeneralDTO, error) {
	query := `
		SELECT
			p.id, p.sku, p.part_number, p.name, p.description, p.short_description,
			p.product_type, p.status, p.is_sellable, p.is_stockable,
			p.brand_id, b.name AS brand_name,
			p.category_id, c.name AS category_name,
			p.source_id, s.name AS source_name,
			(SELECT f.path FROM ecom_product_images pi
			   JOIN ecom_files f ON f.id = pi.file_id AND f.deleted_at IS NULL
			   WHERE pi.product_id = p.id AND pi.is_first = 1 AND pi.deleted_at IS NULL
			   ORDER BY pi.id ASC LIMIT 1) AS image_path,
			(SELECT sd.base_url FROM ecom_product_images pi
			   JOIN ecom_files f ON f.id = pi.file_id AND f.deleted_at IS NULL
			   JOIN ecom_storage_disks sd ON sd.id = f.disk_id AND sd.deleted_at IS NULL
			   WHERE pi.product_id = p.id AND pi.is_first = 1 AND pi.deleted_at IS NULL
			   ORDER BY pi.id ASC LIMIT 1) AS image_base_url,
			p.created_at, p.updated_at,
			cu.username AS created_by_name, uu.username AS updated_by_name
		FROM ecom_products p
		LEFT JOIN ecom_brands b ON b.id = p.brand_id AND b.deleted_at IS NULL
		LEFT JOIN ecom_categories c ON c.id = p.category_id AND c.deleted_at IS NULL
		LEFT JOIN ecom_sources s ON s.id = p.source_id AND s.deleted_at IS NULL
		LEFT JOIN ecom_api_user cu ON cu.id = p.created_by
		LEFT JOIN ecom_api_user uu ON uu.id = p.updated_by
		WHERE p.id = ? AND p.deleted_at IS NULL
		LIMIT 1
	`

	var (
		d                            ProductGeneralDTO
		description, shortDesc       sql.NullString
		status                       sql.NullString
		brandID, categoryID          sql.NullInt64
		brandName, categoryName      sql.NullString
		sourceName                   sql.NullString
		imagePath, imageBaseURL      sql.NullString
		createdByName, updatedByName sql.NullString
	)

	err := r.db.QueryRowContext(ctx, query, productID).Scan(
		&d.ID, &d.SKU, &d.PartNumber, &d.Name, &description, &shortDesc,
		&d.ProductType, &status, &d.IsSellable, &d.IsStockable,
		&brandID, &brandName,
		&categoryID, &categoryName,
		&d.SourceID, &sourceName,
		&imagePath, &imageBaseURL,
		&d.CreatedAt, &d.UpdatedAt,
		&createdByName, &updatedByName,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("error querying product general details: %w", err)
	}

	d.Description = nullStringPtr(description)
	d.ShortDescription = nullStringPtr(shortDesc)
	d.Status = nullStringPtr(status)
	d.BrandName = nullStringPtr(brandName)
	d.CategoryName = nullStringPtr(categoryName)
	d.SourceName = nullStringPtr(sourceName)
	d.CreatedBy = nullStringPtr(createdByName)
	d.UpdatedBy = nullStringPtr(updatedByName)
	if brandID.Valid {
		d.BrandID = &brandID.Int64
	}
	if categoryID.Valid {
		d.CategoryID = &categoryID.Int64
	}
	d.CoverImage = buildFileURL(imageBaseURL.String, imagePath.String)

	if categoryID.Valid {
		path, pathErr := r.categoryPath(ctx, categoryID.Int64)
		if pathErr != nil {
			return nil, pathErr
		}
		d.CategoryPath = &path
	}

	return &d, nil
}

// UpdateGeneralFields carries the columns of the General section that the detail
// page lets an admin edit inline. A nil pointer means "not sent — leave the
// column untouched", so the same PATCH endpoint serves each editable card
// (Identificación → Name, Clasificación → BrandID/CategoryID, Descripción →
// Description/ShortDescription). For the nullable FKs a non-nil pointer whose
// value is <= 0 clears the column (SET NULL). Widen this struct AND the builder
// below together when enabling more fields (status, is_sellable, ...).
type UpdateGeneralFields struct {
	Name             *string
	Description      *string
	ShortDescription *string
	BrandID          *int64
	CategoryID       *int64
}

// UpdateGeneral applies a partial update to ecom_products: one statement whose
// SET list is built from whichever fields are present. Column names come from a
// fixed whitelist here, never from the caller. It returns the affected row
// count without translating 0 into ErrProductNotFound — MySQL also reports 0
// when the values are unchanged — so the service checks existence via
// FindGeneral instead.
func (r *ProductDetailsRepository) UpdateGeneral(ctx context.Context, productID, actorID int64, in UpdateGeneralFields) (int64, error) {
	setClauses := make([]string, 0, 7)
	args := make([]any, 0, 8)

	addString := func(column string, value *string) {
		if value == nil {
			return
		}
		setClauses = append(setClauses, column+" = ?")
		args = append(args, *value)
	}
	// FK anulable: puntero nil = no tocar; valor <= 0 = SET NULL; > 0 = ese id.
	addNullableID := func(column string, value *int64) {
		if value == nil {
			return
		}
		setClauses = append(setClauses, column+" = ?")
		if *value > 0 {
			args = append(args, *value)
		} else {
			args = append(args, nil)
		}
	}

	addString("name", in.Name)
	addString("description", in.Description)
	addString("short_description", in.ShortDescription)
	addNullableID("brand_id", in.BrandID)
	addNullableID("category_id", in.CategoryID)

	if len(setClauses) == 0 {
		return 0, nil
	}

	setClauses = append(setClauses, "updated_by = ?", "updated_at = NOW()")
	args = append(args, actorID, productID)

	query := "UPDATE ecom_products SET " + strings.Join(setClauses, ", ") +
		" WHERE id = ? AND deleted_at IS NULL"

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("error updating product general details: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error getting rows affected: %w", err)
	}

	return affected, nil
}

// categoryPath walks ecom_categories.parent_id upwards to build the full
// "root / .. / leaf" label. The loop is bounded by maxCategoryDepth so a
// cyclic parent_id in the data can't hang the request.
const maxCategoryDepth = 20

func (r *ProductDetailsRepository) categoryPath(ctx context.Context, categoryID int64) (string, error) {
	names := make([]string, 0, 4)
	currentID := categoryID

	for i := 0; i < maxCategoryDepth; i++ {
		var name string
		var parentID sql.NullInt64
		err := r.db.QueryRowContext(ctx,
			"SELECT name, parent_id FROM ecom_categories WHERE id = ? AND deleted_at IS NULL",
			currentID,
		).Scan(&name, &parentID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				break
			}
			return "", fmt.Errorf("error resolving category path: %w", err)
		}

		names = append([]string{name}, names...)
		if !parentID.Valid {
			break
		}
		currentID = parentID.Int64
	}

	path := ""
	for i, name := range names {
		if i > 0 {
			path += " / "
		}
		path += name
	}
	return path, nil
}

// --- Multimedia --------------------------------------------------------------

// ProductMediaItemDTO is one file attached to the product, whatever the table
// it came from (images, videos or media/attachments) — the UI renders the
// three lists the same way.
type ProductMediaItemDTO struct {
	ID        int64     `json:"id"`
	FileID    int64     `json:"fileId"`
	URL       *string   `json:"url,omitempty"`
	Filename  string    `json:"filename"`
	MimeType  string    `json:"mimeType"`
	Extension string    `json:"extension"`
	Size      int64     `json:"size"`
	Width     *int64    `json:"width,omitempty"`
	Height    *int64    `json:"height,omitempty"`
	IsFirst   bool      `json:"isFirst,omitempty"`
	Type      string    `json:"type,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type ProductMediaSectionDTO struct {
	Images      []ProductMediaItemDTO `json:"images"`
	Videos      []ProductMediaItemDTO `json:"videos"`
	Attachments []ProductMediaItemDTO `json:"attachments"`
}

// mediaFileColumns is the file half every media query selects, so the three
// lists share scanMediaRows below.
const mediaFileColumns = `
	f.id, f.filename, f.mime_type, f.extension, f.size, f.width, f.height,
	f.path, sd.base_url
`

func (r *ProductDetailsRepository) FindMedia(ctx context.Context, productID int64) (*ProductMediaSectionDTO, error) {
	section := &ProductMediaSectionDTO{
		Images:      make([]ProductMediaItemDTO, 0),
		Videos:      make([]ProductMediaItemDTO, 0),
		Attachments: make([]ProductMediaItemDTO, 0),
	}

	// Images — cover first (is_first), then by id, matching the order the
	// marketplace publishers use.
	imagesQuery := `
		SELECT pi.id, pi.is_first, '' AS media_type, pi.created_at, ` + mediaFileColumns + `
		FROM ecom_product_images pi
		JOIN ecom_files f ON f.id = pi.file_id AND f.deleted_at IS NULL
		LEFT JOIN ecom_storage_disks sd ON sd.id = f.disk_id AND sd.deleted_at IS NULL
		WHERE pi.product_id = ? AND pi.deleted_at IS NULL
		ORDER BY pi.is_first DESC, pi.id ASC
	`
	images, err := r.scanMediaRows(ctx, imagesQuery, productID)
	if err != nil {
		return nil, fmt.Errorf("error querying product images: %w", err)
	}
	section.Images = images

	videosQuery := `
		SELECT pv.id, 0 AS is_first, '' AS media_type, pv.created_at, ` + mediaFileColumns + `
		FROM ecom_product_videos pv
		JOIN ecom_files f ON f.id = pv.file_id AND f.deleted_at IS NULL
		LEFT JOIN ecom_storage_disks sd ON sd.id = f.disk_id AND sd.deleted_at IS NULL
		WHERE pv.product_id = ? AND pv.deleted_at IS NULL
		ORDER BY pv.id ASC
	`
	videos, err := r.scanMediaRows(ctx, videosQuery, productID)
	if err != nil {
		return nil, fmt.Errorf("error querying product videos: %w", err)
	}
	section.Videos = videos

	// ecom_product_media has no surrogate key — file_id stands in as the row
	// id, which is unique per product in practice (a file is attached once).
	attachmentsQuery := `
		SELECT pm.file_id AS id, 0 AS is_first, pm.type AS media_type, pm.created_at, ` + mediaFileColumns + `
		FROM ecom_product_media pm
		JOIN ecom_files f ON f.id = pm.file_id AND f.deleted_at IS NULL
		LEFT JOIN ecom_storage_disks sd ON sd.id = f.disk_id AND sd.deleted_at IS NULL
		WHERE pm.product_id = ? AND pm.deleted_at IS NULL
		ORDER BY pm.type ASC, pm.file_id ASC
	`
	attachments, err := r.scanMediaRows(ctx, attachmentsQuery, productID)
	if err != nil {
		return nil, fmt.Errorf("error querying product media: %w", err)
	}
	section.Attachments = attachments

	return section, nil
}

func (r *ProductDetailsRepository) scanMediaRows(ctx context.Context, query string, productID int64) ([]ProductMediaItemDTO, error) {
	rows, err := r.db.QueryContext(ctx, query, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ProductMediaItemDTO, 0)
	for rows.Next() {
		var (
			item          ProductMediaItemDTO
			mediaType     sql.NullString
			width, height sql.NullInt64
			path, baseURL sql.NullString
		)
		if err := rows.Scan(
			&item.ID, &item.IsFirst, &mediaType, &item.CreatedAt,
			&item.FileID, &item.Filename, &item.MimeType, &item.Extension, &item.Size,
			&width, &height, &path, &baseURL,
		); err != nil {
			return nil, err
		}

		item.Type = mediaType.String
		if width.Valid {
			item.Width = &width.Int64
		}
		if height.Valid {
			item.Height = &height.Int64
		}
		item.URL = buildFileURL(baseURL.String, path.String)
		items = append(items, item)
	}

	return items, rows.Err()
}

// --- Precios -----------------------------------------------------------------

type ProductPriceDetailDTO struct {
	ID             int64     `json:"id"`
	PriceListID    int64     `json:"priceListId"`
	PriceListName  string    `json:"priceListName"`
	PriceListPrio  int       `json:"priceListPriority"`
	Status         string    `json:"status"`
	ValidFrom      time.Time `json:"validFrom"`
	ValidTo        time.Time `json:"validTo"`
	Price          string    `json:"price"`
	CurrencyID     int64     `json:"currencyId"`
	CurrencyCode   string    `json:"currencyCode"`
	CurrencySymbol string    `json:"currencySymbol"`
	Margin         string    `json:"margin"`
	TaxIncluded    bool      `json:"taxIncluded"`
	UpdatedAt      time.Time `json:"updatedAt"`
	UpdatedBy      *string   `json:"updatedBy,omitempty"`
	// IsEffective marks the row FindEffectivePrice would pick: the
	// highest-priority active list whose validity window covers today.
	IsEffective bool `json:"isEffective"`
}

type ProductPriceHistoryDetailDTO struct {
	ID             int64     `json:"id"`
	PriceListID    int64     `json:"priceListId"`
	PriceListName  *string   `json:"priceListName,omitempty"`
	OldPrice       string    `json:"oldPrice"`
	NewPrice       string    `json:"newPrice"`
	CurrencyCode   *string   `json:"currencyCode,omitempty"`
	CurrencySymbol *string   `json:"currencySymbol,omitempty"`
	UpdatedBy      *string   `json:"updatedBy,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// ProductPricingSectionDTO no longer embeds the price-change history: it can run
// to thousands of rows on a long-lived product, so the UI pages it separately via
// FindPriceHistory / GET .../details/pricing/history.
type ProductPricingSectionDTO struct {
	Current []ProductPriceDetailDTO `json:"current"`
}

func (r *ProductDetailsRepository) FindPricing(ctx context.Context, productID int64) (*ProductPricingSectionDTO, error) {
	section := &ProductPricingSectionDTO{
		Current: make([]ProductPriceDetailDTO, 0),
	}

	currentQuery := `
		SELECT
			pp.id, pp.price_list_id, pl.name, pl.priority, pl.status, pl.valid_from, pl.valid_to,
			pp.price, pp.currency, cur.code, cur.symbol,
			pp.margin, pp.tax_included, pp.updated_at, u.username
		FROM ecom_product_prices pp
		JOIN ecom_price_list pl ON pl.id = pp.price_list_id AND pl.deleted_at IS NULL
		JOIN ecom_currencies cur ON cur.id = pp.currency
		LEFT JOIN ecom_api_user u ON u.id = pp.updated_by
		WHERE pp.product_id = ?
		ORDER BY pl.priority DESC, pp.updated_at DESC
	`
	rows, err := r.db.QueryContext(ctx, currentQuery, productID)
	if err != nil {
		return nil, fmt.Errorf("error querying product prices: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			p         ProductPriceDetailDTO
			updatedBy sql.NullString
		)
		if err := rows.Scan(
			&p.ID, &p.PriceListID, &p.PriceListName, &p.PriceListPrio, &p.Status, &p.ValidFrom, &p.ValidTo,
			&p.Price, &p.CurrencyID, &p.CurrencyCode, &p.CurrencySymbol,
			&p.Margin, &p.TaxIncluded, &p.UpdatedAt, &updatedBy,
		); err != nil {
			return nil, fmt.Errorf("error scanning product price: %w", err)
		}
		p.UpdatedBy = nullStringPtr(updatedBy)
		section.Current = append(section.Current, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating product prices: %w", err)
	}

	// The list is already ordered by priority DESC, so the first row that is
	// active and currently valid is the effective one (same rule as
	// ProductPricesRepository.FindEffectivePrice).
	today := time.Now()
	for i := range section.Current {
		p := &section.Current[i]
		if p.Status == "active" && !today.Before(p.ValidFrom) && !today.After(p.ValidTo.AddDate(0, 0, 1)) {
			p.IsEffective = true
			break
		}
	}

	return section, nil
}

// PaginatedProductPriceHistory is one page of ecom_price_history for the product
// detail page's "Historial de cambios" table.
type PaginatedProductPriceHistory struct {
	Data     []ProductPriceHistoryDetailDTO `json:"data"`
	Total    int64                          `json:"total"`
	Offset   int                            `json:"offset"`
	PageSize int                            `json:"pageSize"`
}

func (r *ProductDetailsRepository) FindPriceHistory(ctx context.Context, productID int64, offset, pageSize int) (*PaginatedProductPriceHistory, error) {
	page := &PaginatedProductPriceHistory{
		Data:     make([]ProductPriceHistoryDetailDTO, 0),
		Offset:   offset,
		PageSize: pageSize,
	}

	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM ecom_price_history WHERE product_id = ?",
		productID,
	).Scan(&page.Total); err != nil {
		return nil, fmt.Errorf("error counting price history: %w", err)
	}

	const historyQuery = `
		SELECT ph.id, ph.price_list_id, pl.name, ph.old_price, ph.new_price,
		       cur.code, cur.symbol, u.username, ph.created_at
		FROM ecom_price_history ph
		LEFT JOIN ecom_price_list pl ON pl.id = ph.price_list_id
		LEFT JOIN ecom_currencies cur ON cur.id = ph.currency_id
		LEFT JOIN ecom_api_user u ON u.id = ph.updated_by
		WHERE ph.product_id = ?
		ORDER BY ph.created_at DESC, ph.id DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.QueryContext(ctx, historyQuery, productID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying price history: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			h                            ProductPriceHistoryDetailDTO
			listName, code, symbol, user sql.NullString
		)
		if err := rows.Scan(
			&h.ID, &h.PriceListID, &listName, &h.OldPrice, &h.NewPrice,
			&code, &symbol, &user, &h.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning price history: %w", err)
		}
		h.PriceListName = nullStringPtr(listName)
		h.CurrencyCode = nullStringPtr(code)
		h.CurrencySymbol = nullStringPtr(symbol)
		h.UpdatedBy = nullStringPtr(user)
		page.Data = append(page.Data, h)
	}

	return page, rows.Err()
}

// --- Inventario ---------------------------------------------------------------

type ProductStockDetailDTO struct {
	ID            int64      `json:"id"`
	BranchID      int64      `json:"branchId"`
	BranchName    string     `json:"branchName"`
	WarehouseID   int64      `json:"warehouseId"`
	WarehouseName string     `json:"warehouseName"`
	AvailableQty  int        `json:"availableQty"`
	LastSyncAt    *time.Time `json:"lastSyncAt,omitempty"`
}

type ProductStockMovementDetailDTO struct {
	ID             int64     `json:"id"`
	BranchName     *string   `json:"branchName,omitempty"`
	WarehouseName  *string   `json:"warehouseName,omitempty"`
	MovementType   string    `json:"movementType"`
	QuantityBefore int       `json:"quantityBefore"`
	QuantityChange int       `json:"quantityChange"`
	QuantityAfter  int       `json:"quantityAfter"`
	UpdatedBy      *string   `json:"updatedBy,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// ProductInventorySectionDTO no longer embeds the movement log: a synced product
// gets a movement row per ERP sync that changed its quantity, so the UI pages it
// separately via FindStockMovements / GET .../details/inventory/movements.
type ProductInventorySectionDTO struct {
	TotalStock int                     `json:"totalStock"`
	ByLocation []ProductStockDetailDTO `json:"byLocation"`
}

func (r *ProductDetailsRepository) FindInventory(ctx context.Context, productID int64) (*ProductInventorySectionDTO, error) {
	section := &ProductInventorySectionDTO{
		ByLocation: make([]ProductStockDetailDTO, 0),
	}

	stockQuery := `
		SELECT ps.id, ps.branch_id, b.name, ps.warehouse_id, w.name,
		       ps.available_qty, ps.last_sync_at
		FROM ecom_product_stock ps
		JOIN ecom_branches b ON b.id = ps.branch_id
		JOIN ecom_warehouses w ON w.id = ps.warehouse_id
		WHERE ps.product_id = ?
		ORDER BY b.name ASC, w.name ASC
	`
	rows, err := r.db.QueryContext(ctx, stockQuery, productID)
	if err != nil {
		return nil, fmt.Errorf("error querying product stock: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			s          ProductStockDetailDTO
			lastSyncAt sql.NullTime
		)
		if err := rows.Scan(
			&s.ID, &s.BranchID, &s.BranchName, &s.WarehouseID, &s.WarehouseName,
			&s.AvailableQty, &lastSyncAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning product stock: %w", err)
		}
		if lastSyncAt.Valid {
			s.LastSyncAt = &lastSyncAt.Time
		}
		section.TotalStock += s.AvailableQty
		section.ByLocation = append(section.ByLocation, s)
	}
	return section, rows.Err()
}

// PaginatedProductStockMovements is one page of ecom_stock_movements for the
// product detail page's "Historial de movimientos" table.
type PaginatedProductStockMovements struct {
	Data     []ProductStockMovementDetailDTO `json:"data"`
	Total    int64                           `json:"total"`
	Offset   int                             `json:"offset"`
	PageSize int                             `json:"pageSize"`
}

func (r *ProductDetailsRepository) FindStockMovements(ctx context.Context, productID int64, offset, pageSize int) (*PaginatedProductStockMovements, error) {
	page := &PaginatedProductStockMovements{
		Data:     make([]ProductStockMovementDetailDTO, 0),
		Offset:   offset,
		PageSize: pageSize,
	}

	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM ecom_stock_movements WHERE product_id = ? AND deleted_at IS NULL",
		productID,
	).Scan(&page.Total); err != nil {
		return nil, fmt.Errorf("error counting stock movements: %w", err)
	}

	const movementsQuery = `
		SELECT sm.id, b.name, w.name, sm.movement_type,
		       sm.quantity_before, sm.quantity_change, sm.quantity_after,
		       u.username, sm.created_at
		FROM ecom_stock_movements sm
		LEFT JOIN ecom_branches b ON b.id = sm.branch_id
		LEFT JOIN ecom_warehouses w ON w.id = sm.warehouse_id
		LEFT JOIN ecom_api_user u ON u.id = sm.updated_by
		WHERE sm.product_id = ? AND sm.deleted_at IS NULL
		ORDER BY sm.created_at DESC, sm.id DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.QueryContext(ctx, movementsQuery, productID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying stock movements: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			m                      ProductStockMovementDetailDTO
			branch, warehouse, who sql.NullString
		)
		if err := rows.Scan(
			&m.ID, &branch, &warehouse, &m.MovementType,
			&m.QuantityBefore, &m.QuantityChange, &m.QuantityAfter,
			&who, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning stock movement: %w", err)
		}
		m.BranchName = nullStringPtr(branch)
		m.WarehouseName = nullStringPtr(warehouse)
		m.UpdatedBy = nullStringPtr(who)
		page.Data = append(page.Data, m)
	}

	return page, rows.Err()
}

// --- Números de parte ----------------------------------------------------------

type ProductPartNumberDetailDTO struct {
	PartNumber string    `json:"partNumber"`
	Type       string    `json:"type"`
	BrandID    int64     `json:"brandId"`
	BrandName  *string   `json:"brandName,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// ProductSupersessionDetailDTO is one link of the part-number succession
// chain. Direction says how it relates to the product being viewed:
// "supersedes" (this product replaces OtherPartNumber) or "superseded_by"
// (this product was replaced by it).
type ProductSupersessionDetailDTO struct {
	ID              int64      `json:"id"`
	Direction       string     `json:"direction"`
	OldPartNumber   string     `json:"oldPartNumber"`
	NewPartNumber   string     `json:"newPartNumber"`
	OtherPartNumber string     `json:"otherPartNumber"`
	OtherProductID  *int64     `json:"otherProductId,omitempty"`
	OtherProductSKU *string    `json:"otherProductSku,omitempty"`
	Resolved        bool       `json:"resolved"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       *time.Time `json:"updatedAt,omitempty"`
}

type ProductPartNumbersSectionDTO struct {
	PartNumber    string                         `json:"partNumber"`
	Alternates    []ProductPartNumberDetailDTO   `json:"alternates"`
	Supersessions []ProductSupersessionDetailDTO `json:"supersessions"`
}

func (r *ProductDetailsRepository) FindPartNumbers(ctx context.Context, productID int64) (*ProductPartNumbersSectionDTO, error) {
	section := &ProductPartNumbersSectionDTO{
		Alternates:    make([]ProductPartNumberDetailDTO, 0),
		Supersessions: make([]ProductSupersessionDetailDTO, 0),
	}

	var sourceID int64
	err := r.db.QueryRowContext(ctx,
		"SELECT part_number, source_id FROM ecom_products WHERE id = ? AND deleted_at IS NULL",
		productID,
	).Scan(&section.PartNumber, &sourceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("error resolving product part number: %w", err)
	}

	alternatesQuery := `
		SELECT ppn.part_number, ppn.type, ppn.brand_id, b.name, ppn.created_at
		FROM ecom_product_part_numbers ppn
		LEFT JOIN ecom_brands b ON b.id = ppn.brand_id AND b.deleted_at IS NULL
		WHERE ppn.product_id = ? AND ppn.deleted_at IS NULL
		ORDER BY ppn.type ASC, ppn.part_number ASC
	`
	rows, err := r.db.QueryContext(ctx, alternatesQuery, productID)
	if err != nil {
		return nil, fmt.Errorf("error querying product part numbers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			pn        ProductPartNumberDetailDTO
			brandName sql.NullString
		)
		if err := rows.Scan(&pn.PartNumber, &pn.Type, &pn.BrandID, &brandName, &pn.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning product part number: %w", err)
		}
		pn.BrandName = nullStringPtr(brandName)
		section.Alternates = append(section.Alternates, pn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating product part numbers: %w", err)
	}

	// A supersession row is linked to this product either by the resolved FK
	// (old_product_id / new_product_id) or, when still unresolved, by matching
	// its part number within the same source — both sides are listed.
	supersessionsQuery := `
		SELECT pns.id,
		       CASE WHEN pns.old_part_number = ? THEN 'superseded_by' ELSE 'supersedes' END AS direction,
		       pns.old_part_number, pns.new_part_number,
		       CASE WHEN pns.old_part_number = ? THEN pns.new_part_number ELSE pns.old_part_number END AS other_part_number,
		       CASE WHEN pns.old_part_number = ? THEN pns.new_product_id ELSE pns.old_product_id END AS other_product_id,
		       op.sku, np.sku,
		       pns.old_resolved_at, pns.new_resolved_at,
		       pns.created_at, pns.updated_at
		FROM ecom_part_number_supersessions pns
		LEFT JOIN ecom_products op ON op.id = pns.old_product_id AND op.deleted_at IS NULL
		LEFT JOIN ecom_products np ON np.id = pns.new_product_id AND np.deleted_at IS NULL
		WHERE pns.deleted_at IS NULL
		  AND (
		        pns.old_product_id = ? OR pns.new_product_id = ?
		        OR (pns.source_id = ? AND (pns.old_part_number = ? OR pns.new_part_number = ?))
		      )
		ORDER BY pns.created_at DESC, pns.id DESC
	`
	supersessionRows, err := r.db.QueryContext(ctx, supersessionsQuery,
		section.PartNumber, section.PartNumber, section.PartNumber,
		productID, productID, sourceID, section.PartNumber, section.PartNumber,
	)
	if err != nil {
		return nil, fmt.Errorf("error querying part number supersessions: %w", err)
	}
	defer supersessionRows.Close()

	for supersessionRows.Next() {
		var (
			s                            ProductSupersessionDetailDTO
			otherProductID               sql.NullInt64
			oldSKU, newSKU               sql.NullString
			oldResolvedAt, newResolvedAt sql.NullTime
			updatedAt                    sql.NullTime
		)
		if err := supersessionRows.Scan(
			&s.ID, &s.Direction, &s.OldPartNumber, &s.NewPartNumber, &s.OtherPartNumber,
			&otherProductID, &oldSKU, &newSKU,
			&oldResolvedAt, &newResolvedAt, &s.CreatedAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning part number supersession: %w", err)
		}

		if otherProductID.Valid {
			s.OtherProductID = &otherProductID.Int64
		}
		// The "other" side's sku comes from whichever join matched it.
		if s.Direction == "superseded_by" {
			s.OtherProductSKU = nullStringPtr(newSKU)
			s.Resolved = newResolvedAt.Valid
		} else {
			s.OtherProductSKU = nullStringPtr(oldSKU)
			s.Resolved = oldResolvedAt.Valid
		}
		if updatedAt.Valid {
			s.UpdatedAt = &updatedAt.Time
		}
		section.Supersessions = append(section.Supersessions, s)
	}

	return section, supersessionRows.Err()
}

// --- Atributos ------------------------------------------------------------------

type ProductDimensionsDetailDTO struct {
	Weight   string `json:"weight"`
	Length   string `json:"length"`
	Width    string `json:"width"`
	Height   string `json:"height"`
	Diameter string `json:"diameter"`
	Volume   string `json:"volume"`
}

type ProductSEODetailDTO struct {
	MetaTitle       *string `json:"metaTitle,omitempty"`
	MetaDescription *string `json:"metaDescription,omitempty"`
	Keywords        *string `json:"keywords,omitempty"`
}

// ProductAttributeDetailDTO carries the attribute's definition (name, code,
// data type, unit) alongside the value, with Value already rendered as the
// display string for whichever value_* column the data type uses.
type ProductAttributeDetailDTO struct {
	ID            int64   `json:"id"`
	AttributeID   int64   `json:"attributeId"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	DataType      string  `json:"dataType"`
	Unit          *string `json:"unit,omitempty"`
	Value         string  `json:"value"`
	OptionID      *int64  `json:"optionId,omitempty"`
	UpdatedByName *string `json:"updatedBy,omitempty"`
}

type ProductAttributesSectionDTO struct {
	Dimensions *ProductDimensionsDetailDTO `json:"dimensions,omitempty"`
	SEO        *ProductSEODetailDTO        `json:"seo,omitempty"`
	Attributes []ProductAttributeDetailDTO `json:"attributes"`
}

func (r *ProductDetailsRepository) FindAttributes(ctx context.Context, productID int64) (*ProductAttributesSectionDTO, error) {
	section := &ProductAttributesSectionDTO{
		Attributes: make([]ProductAttributeDetailDTO, 0),
	}

	var dimensions ProductDimensionsDetailDTO
	err := r.db.QueryRowContext(ctx, `
		SELECT weight, length, width, height, diameter, volume
		FROM ecom_product_dimensions
		WHERE product_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, productID).Scan(
		&dimensions.Weight, &dimensions.Length, &dimensions.Width,
		&dimensions.Height, &dimensions.Diameter, &dimensions.Volume,
	)
	switch {
	case err == nil:
		section.Dimensions = &dimensions
	case errors.Is(err, sql.ErrNoRows):
		// El producto no tiene dimensiones cargadas: la sección lo muestra vacío.
	default:
		return nil, fmt.Errorf("error querying product dimensions: %w", err)
	}

	var (
		seo                             ProductSEODetailDTO
		metaTitle, metaDesc, seoKeyword sql.NullString
	)
	err = r.db.QueryRowContext(ctx, `
		SELECT meta_title, meta_description, keywords
		FROM ecom_product_seo
		WHERE product_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, productID).Scan(&metaTitle, &metaDesc, &seoKeyword)
	switch {
	case err == nil:
		seo.MetaTitle = nullStringPtr(metaTitle)
		seo.MetaDescription = nullStringPtr(metaDesc)
		seo.Keywords = nullStringPtr(seoKeyword)
		section.SEO = &seo
	case errors.Is(err, sql.ErrNoRows):
		// Sin fila de SEO — igual que dimensiones, no es un error.
	default:
		return nil, fmt.Errorf("error querying product seo: %w", err)
	}

	attributesQuery := `
		SELECT pa.id, pa.attribute_id, a.code, a.name, a.data_type, a.unit,
		       pa.value_text, pa.value_number, pa.value_boolean, pa.value_date,
		       pa.option_id, ao.value AS option_value, u.username
		FROM ecom_product_attributes pa
		JOIN ecom_attributes a ON a.id = pa.attribute_id AND a.deleted_at IS NULL
		LEFT JOIN ecom_attribute_options ao ON ao.id = pa.option_id AND ao.deleted_at IS NULL
		LEFT JOIN ecom_api_user u ON u.id = pa.updated_by
		WHERE pa.product_id = ? AND pa.deleted_at IS NULL
		ORDER BY a.name ASC
	`
	rows, err := r.db.QueryContext(ctx, attributesQuery, productID)
	if err != nil {
		return nil, fmt.Errorf("error querying product attributes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			a                     ProductAttributeDetailDTO
			unit                  sql.NullString
			valueText             sql.NullString
			valueNumber           sql.NullFloat64
			valueBool             sql.NullBool
			valueDate             sql.NullTime
			optionID              sql.NullInt64
			optionValue, whoValue sql.NullString
		)
		if err := rows.Scan(
			&a.ID, &a.AttributeID, &a.Code, &a.Name, &a.DataType, &unit,
			&valueText, &valueNumber, &valueBool, &valueDate,
			&optionID, &optionValue, &whoValue,
		); err != nil {
			return nil, fmt.Errorf("error scanning product attribute: %w", err)
		}

		a.Unit = nullStringPtr(unit)
		a.UpdatedByName = nullStringPtr(whoValue)
		if optionID.Valid {
			a.OptionID = &optionID.Int64
		}

		switch {
		case optionValue.Valid:
			a.Value = optionValue.String
		case valueText.Valid:
			a.Value = valueText.String
		case valueNumber.Valid:
			a.Value = fmt.Sprintf("%g", valueNumber.Float64)
		case valueBool.Valid:
			if valueBool.Bool {
				a.Value = "Sí"
			} else {
				a.Value = "No"
			}
		case valueDate.Valid:
			a.Value = valueDate.Time.Format(time.DateOnly)
		}

		section.Attributes = append(section.Attributes, a)
	}

	return section, rows.Err()
}

// nullStringPtr returns nil for a NULL/empty column and a pointer to its value
// otherwise, so optional text fields serialize as absent instead of "".
func nullStringPtr(value sql.NullString) *string {
	if !value.Valid || value.String == "" {
		return nil
	}
	v := value.String
	return &v
}
