package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	channelAttributeValuesApp "core-orchestrator/internal/application/channel_attribute_values"
	syncApp "core-orchestrator/internal/application/sync"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	odooInfra "core-orchestrator/internal/infrastructure/marketplace/odoo"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// VecomSyncProductMigrationService is a TEMPORARY, one-off migration helper:
// it reads the previous system's legacy tables (vecom_sync_product joined to
// vecom_products) and, for each row, creates/matches the local ecom_products
// row, records the ecom_channel_product_map entry, and completes the missing
// data (category, status, description, dimensions, category attributes,
// brand) by querying the real Odoo / MercadoLibre connections.
//
// Nothing is ever written back to the marketplaces — this only reads listings
// and writes locally, same as sync.MercadoLibreListingsAuditService. Remove
// this service, its handler method and its route once the migration is done.
type VecomSyncProductMigrationService struct {
	db                            *sql.DB
	productRepository             *mysqlInfra.ProductRepository
	brandsRepository              *mysqlInfra.BrandsRepository
	productDimensionsRepository   *mysqlInfra.ProductDimensionsRepository
	channelProductMapRepository   *mysqlInfra.ChannelProductMapRepository
	channelCategoryMapRepository  *mysqlInfra.ChannelCategoryMapRepository
	categoriesRepository          *mysqlInfra.CategoriesRepository
	credentialsRepository         *mysqlInfra.ConnectionCredentialsRepository
	settingsRepository            *mysqlInfra.ConnectionSettingsRepository
	tokenService                  *syncApp.MercadoLibreTokenService
	channelAttributeValuesService *channelAttributeValuesApp.Service
	itemsHandler                  *mercadoLibreInfra.ItemsHandler
	odooRateLimiter               *odooInfra.RateLimiter
}

func NewVecomSyncProductMigrationService(
	db *sql.DB,
	productRepository *mysqlInfra.ProductRepository,
	brandsRepository *mysqlInfra.BrandsRepository,
	productDimensionsRepository *mysqlInfra.ProductDimensionsRepository,
	channelProductMapRepository *mysqlInfra.ChannelProductMapRepository,
	channelCategoryMapRepository *mysqlInfra.ChannelCategoryMapRepository,
	categoriesRepository *mysqlInfra.CategoriesRepository,
	credentialsRepository *mysqlInfra.ConnectionCredentialsRepository,
	settingsRepository *mysqlInfra.ConnectionSettingsRepository,
	tokenService *syncApp.MercadoLibreTokenService,
	channelAttributeValuesService *channelAttributeValuesApp.Service,
	mercadoLibreRateLimiter *mercadoLibreInfra.RateLimiter,
	odooRateLimiter *odooInfra.RateLimiter,
) *VecomSyncProductMigrationService {
	return &VecomSyncProductMigrationService{
		db:                            db,
		productRepository:             productRepository,
		brandsRepository:              brandsRepository,
		productDimensionsRepository:   productDimensionsRepository,
		channelProductMapRepository:   channelProductMapRepository,
		channelCategoryMapRepository:  channelCategoryMapRepository,
		categoriesRepository:          categoriesRepository,
		credentialsRepository:         credentialsRepository,
		settingsRepository:            settingsRepository,
		tokenService:                  tokenService,
		channelAttributeValuesService: channelAttributeValuesService,
		itemsHandler:                  mercadoLibreInfra.NewItemsHandler(mercadoLibreInfra.NewClient(nil, "", mercadoLibreRateLimiter)),
		odooRateLimiter:               odooRateLimiter,
	}
}

const (
	// integration_id values in the legacy vecom_sync_product table and the
	// ecom_channel_connections they map to. Any other integration_id is
	// skipped entirely.
	vecomOdooIntegrationID int64 = 4
	vecomMeliIntegrationID int64 = 5
	vecomOdooConnectionID  int64 = 3
	vecomMeliConnectionID  int64 = 1

	// company_id (vecom_products) -> ecom_products.source_id / brand_id.
	vecomNissanCompanyID  int64 = 5
	vecomNissanSourceID   int64 = 13 // Nissan
	vecomDynamicsSourceID int64 = 14 // Dynamics
	vecomNissanBrandID    int64 = 1

	vecomProductType = "part"

	vecomMapStatusPending = "pending"
	vecomMapStatusSynced  = "synced"

	// MercadoLibre raw item statuses / folded ecom_channel_product_map values
	// — duplicated from sync_mercadolibre_products.go (unexported there) with
	// the same rationale as foldSyncItemMeliMigrationStatus in
	// mercadolibre_migrate_syncitemmeli_handler.go.
	vecomMeliSubStatusForbidden = "forbidden"
	vecomMeliStatusUnderReview  = "under_review"
	vecomMeliStatusPaused       = "paused"
)

// MigrateVecomInput narrows a run down to a subset of vecom_sync_product rows
// so a large migration can be done in batches (MercadoLibre is rate limited).
// A zero value processes every row.
type MigrateVecomInput struct {
	IntegrationID *int64
	Limit         int
	Offset        int
}

// VecomAttributeOutcome reports what happened for one MercadoLibre category
// attribute this migration tried to mirror into the local attribute tables.
type VecomAttributeOutcome struct {
	ExternalKey string `json:"externalKey"`
	Written     bool   `json:"written,omitempty"`
	Error       string `json:"error,omitempty"`
}

// VecomRowOutcome is the per-row report. A row failing never stops the batch.
type VecomRowOutcome struct {
	SyncProductID      int64                   `json:"syncProductId"`
	IntegrationID      int64                   `json:"integrationId"`
	ConnectionID       int64                   `json:"connectionId,omitempty"`
	Code               string                  `json:"code"`
	ExternalID         string                  `json:"externalId"`
	ProductID          *int64                  `json:"productId,omitempty"`
	ProductCreated     bool                    `json:"productCreated,omitempty"`
	BrandAssigned      string                  `json:"brandAssigned,omitempty"`
	CategoryLocalID    *int64                  `json:"categoryLocalId,omitempty"`
	ExternalCategoryID string                  `json:"externalCategoryId,omitempty"`
	Status             string                  `json:"status,omitempty"`
	DescriptionUpdated bool                    `json:"descriptionUpdated,omitempty"`
	DimensionsUpdated  bool                    `json:"dimensionsUpdated,omitempty"`
	Attributes         []VecomAttributeOutcome `json:"attributes,omitempty"`
	// CategoryMapping reports what happened with the reuse mapping
	// (ecom_channel_category_map) that links the product's local category to
	// this connection's external category: "created", "exists" (already
	// mapped to the same external id), "conflict" (already mapped to a
	// different one — left untouched, first mapping wins) or "" (nothing to
	// map, e.g. the product has no local category / no external category).
	CategoryMapping string `json:"categoryMapping,omitempty"`
	Skipped         bool   `json:"skipped,omitempty"`
	SkipReason      string `json:"skipReason,omitempty"`
	Error           string `json:"error,omitempty"`
}

type MigrateVecomResult struct {
	TotalRows        int               `json:"totalRows"`
	ProductsCreated  int               `json:"productsCreated"`
	MappingsUpserted int               `json:"mappingsUpserted"`
	Skipped          int               `json:"skipped"`
	Errored          int               `json:"errored"`
	Results          []VecomRowOutcome `json:"results"`
}

type vecomRow struct {
	SyncProductID int64
	IntegrationID int64
	ExternalID    string
	Name          string
	Code          string
	PartNumber    string
	CompanyID     int64
}

// Migrate walks the selected vecom_sync_product rows. actorID is recorded as
// created_by/updated_by on every write.
func (s *VecomSyncProductMigrationService) Migrate(ctx context.Context, input MigrateVecomInput, actorID int64) (*MigrateVecomResult, error) {
	if actorID <= 0 {
		return nil, fmt.Errorf("actorID is required")
	}

	rows, err := s.loadRows(ctx, input)
	if err != nil {
		return nil, err
	}

	result := &MigrateVecomResult{TotalRows: len(rows), Results: make([]VecomRowOutcome, 0, len(rows))}

	// Odoo API context (client + handlers + category resolver) is built once
	// and lazily — the first Odoo row triggers it, and a build failure is
	// reported per row rather than failing the whole batch.
	var odooCtx *vecomOdooContext
	var odooErr error
	odooBuilt := false
	ensureOdoo := func() (*vecomOdooContext, error) {
		if !odooBuilt {
			odooBuilt = true
			odooCtx, odooErr = s.buildOdooContext(ctx, actorID)
		}
		return odooCtx, odooErr
	}

	for _, row := range rows {
		outcome := s.migrateRow(ctx, row, actorID, ensureOdoo)
		result.Results = append(result.Results, outcome)

		switch {
		case outcome.Skipped:
			result.Skipped++
		case outcome.Error != "":
			result.Errored++
		}
		if outcome.ProductCreated {
			result.ProductsCreated++
		}
		if outcome.ProductID != nil && outcome.Error == "" && !outcome.Skipped {
			result.MappingsUpserted++
		}
	}

	return result, nil
}

func (s *VecomSyncProductMigrationService) loadRows(ctx context.Context, input MigrateVecomInput) ([]vecomRow, error) {
	query := `
		SELECT sp.id, sp.integration_id, sp.external_id, sp.name,
		       p.code, p.sku, p.company_id
		FROM vecom_sync_product sp
		JOIN vecom_products p ON p.id = sp.product_id
	`
	args := make([]any, 0, 3)
	if input.IntegrationID != nil {
		query += " WHERE sp.integration_id = ?"
		args = append(args, *input.IntegrationID)
	}
	query += " ORDER BY sp.id"
	if input.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, input.Limit)
		if input.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, input.Offset)
		}
	}

	dbRows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("error querying vecom_sync_product: %w", err)
	}
	defer dbRows.Close()

	out := make([]vecomRow, 0)
	for dbRows.Next() {
		var (
			r          vecomRow
			externalID sql.NullString
			name       sql.NullString
			code       sql.NullString
			partNumber sql.NullString
			companyID  sql.NullInt64
		)
		if err := dbRows.Scan(&r.SyncProductID, &r.IntegrationID, &externalID, &name, &code, &partNumber, &companyID); err != nil {
			return nil, fmt.Errorf("error scanning vecom_sync_product row: %w", err)
		}
		r.ExternalID = strings.TrimSpace(externalID.String)
		r.Name = strings.TrimSpace(name.String)
		r.Code = strings.TrimSpace(code.String)
		r.PartNumber = strings.TrimSpace(partNumber.String)
		r.CompanyID = companyID.Int64
		out = append(out, r)
	}
	if err := dbRows.Err(); err != nil {
		return nil, fmt.Errorf("error reading vecom_sync_product rows: %w", err)
	}

	return out, nil
}

func (s *VecomSyncProductMigrationService) migrateRow(
	ctx context.Context,
	row vecomRow,
	actorID int64,
	ensureOdoo func() (*vecomOdooContext, error),
) VecomRowOutcome {
	outcome := VecomRowOutcome{
		SyncProductID: row.SyncProductID,
		IntegrationID: row.IntegrationID,
		Code:          row.Code,
		ExternalID:    row.ExternalID,
	}

	connectionID, ok := connectionIDForIntegration(row.IntegrationID)
	if !ok {
		outcome.Skipped = true
		outcome.SkipReason = fmt.Sprintf("integration_id %d no soportado", row.IntegrationID)
		return outcome
	}
	outcome.ConnectionID = connectionID

	if row.Code == "" {
		outcome.Error = "vecom_products.code vacío"
		return outcome
	}
	if row.ExternalID == "" {
		outcome.Error = "vecom_sync_product.external_id vacío"
		return outcome
	}

	product, created, err := s.resolveOrCreateProduct(ctx, row, actorID)
	if err != nil {
		outcome.Error = err.Error()
		return outcome
	}
	outcome.ProductID = &product.ID
	outcome.ProductCreated = created

	// Base mapping row — the enrichment below re-upserts it with the real
	// category/status.
	if _, err := s.channelProductMapRepository.Upsert(ctx, mysqlInfra.UpsertChannelProductMapInput{
		ProductID:    product.ID,
		ConnectionID: connectionID,
		ListingTitle: row.Name,
		ExternalID:   row.ExternalID,
		Status:       vecomMapStatusPending,
		ActorID:      actorID,
	}); err != nil {
		outcome.Error = fmt.Sprintf("error creando channel product map: %v", err)
		return outcome
	}

	switch connectionID {
	case vecomOdooConnectionID:
		odooCtx, err := ensureOdoo()
		if err != nil {
			outcome.Error = err.Error()
			return outcome
		}
		s.enrichOdoo(ctx, row, product, connectionID, odooCtx, actorID, &outcome)
	case vecomMeliConnectionID:
		s.enrichMercadoLibre(ctx, row, product, connectionID, actorID, &outcome)
	}

	return outcome
}

// resolveOrCreateProduct returns the ecom_products row for row.Code (matched
// on sku), creating it from the legacy row when absent. Mirrors
// resolveOrCreateProduct in sync_mercadolibre_listings_audit.go, including the
// unique-key race recovery.
func (s *VecomSyncProductMigrationService) resolveOrCreateProduct(ctx context.Context, row vecomRow, actorID int64) (product *mysqlInfra.ProductDTO, created bool, err error) {
	existing, findErr := s.productRepository.FindBySKU(ctx, row.Code)
	if findErr == nil {
		return existing, false, nil
	}
	if !errors.Is(findErr, mysqlInfra.ErrProductNotFound) {
		return nil, false, fmt.Errorf("error buscando producto por sku %s: %w", row.Code, findErr)
	}

	sourceID, ok := sourceIDForCompany(row.CompanyID)
	if !ok {
		return nil, false, fmt.Errorf("company_id %d sin source_id definido", row.CompanyID)
	}

	var brandID *int64
	if row.CompanyID == vecomNissanCompanyID {
		id := vecomNissanBrandID
		brandID = &id
	}

	partNumber := row.PartNumber
	if partNumber == "" {
		partNumber = row.Code
	}

	newProduct, createErr := s.productRepository.Create(ctx, mysqlInfra.CreateProductInput{
		SKU:         row.Code,
		PartNumber:  partNumber,
		Name:        row.Name,
		BrandID:     brandID,
		ProductType: vecomProductType,
		IsSellable:  true,
		IsStockable: true,
		SourceID:    sourceID,
		CreatedBy:   actorID,
	})
	if createErr != nil {
		// Another row for the same source_id + part_number may have inserted
		// it already (unique key) — re-resolve instead of failing.
		if recovered, recErr := s.productRepository.FindBySourceAndPartNumber(ctx, sourceID, partNumber); recErr == nil {
			return recovered, false, nil
		}
		if recovered, recErr := s.productRepository.FindBySKU(ctx, row.Code); recErr == nil {
			return recovered, false, nil
		}
		return nil, false, fmt.Errorf("error creando producto para sku %s: %w", row.Code, createErr)
	}

	return newProduct, true, nil
}

// --- MercadoLibre enrichment -------------------------------------------------

func (s *VecomSyncProductMigrationService) enrichMercadoLibre(
	ctx context.Context,
	row vecomRow,
	product *mysqlInfra.ProductDTO,
	connectionID int64,
	actorID int64,
	outcome *VecomRowOutcome,
) {
	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		outcome.Error = fmt.Sprintf("error obteniendo token mercadolibre: %v", err)
		return
	}

	detail, err := s.itemsHandler.GetItem(ctx, accessToken, row.ExternalID)
	if err != nil {
		outcome.Error = fmt.Sprintf("error consultando item mercadolibre: %v", err)
		return
	}

	// Brand — only when the product still has none.
	if product.BrandID == nil {
		if brandName := meliAttributeValue(detail.Attributes, "BRAND"); brandName != "" {
			if brandID, berr := s.resolveBrandID(ctx, strings.ToUpper(brandName), actorID); berr != nil {
				outcome.Error = fmt.Sprintf("error resolviendo marca %q: %v", brandName, berr)
				return
			} else if uerr := s.productRepository.UpdateBrandID(ctx, product.ID, brandID, actorID); uerr != nil {
				outcome.Error = fmt.Sprintf("error asignando marca al producto: %v", uerr)
				return
			} else {
				outcome.BrandAssigned = strings.ToUpper(brandName)
			}
		}
	}

	// Status.
	status := vecomMapStatusSynced
	if statuses, serr := s.itemsHandler.GetItemsStatus(ctx, accessToken, []string{row.ExternalID}); serr == nil && len(statuses) > 0 {
		status = foldMeliStatus(statuses[0].Status, statuses[0].SubStatus)
	} else if serr != nil {
		log.Printf("vecom migration: item %s status fetch failed: %v", row.ExternalID, serr)
	}
	outcome.Status = status
	outcome.ExternalCategoryID = detail.CategoryID

	if _, err := s.channelProductMapRepository.Upsert(ctx, mysqlInfra.UpsertChannelProductMapInput{
		ProductID:          product.ID,
		ConnectionID:       connectionID,
		ListingTitle:       row.Name,
		ExternalID:         row.ExternalID,
		ExternalCategoryID: detail.CategoryID,
		Status:             status,
		ActorID:            actorID,
	}); err != nil {
		outcome.Error = fmt.Sprintf("error actualizando channel product map: %v", err)
		return
	}

	// Description — never overwrite an existing one.
	if product.Description == nil || strings.TrimSpace(*product.Description) == "" {
		if text := s.fetchMeliDescription(ctx, accessToken, detail); text != "" {
			if err := s.productRepository.UpdateDescription(ctx, product.ID, text, actorID); err != nil {
				outcome.Error = fmt.Sprintf("error actualizando descripción: %v", err)
				return
			}
			outcome.DescriptionUpdated = true
		}
	}

	// Package dimensions.
	if updated, err := s.writeMeliDimensions(ctx, detail, product.ID, actorID); err != nil {
		outcome.Error = fmt.Sprintf("error guardando dimensiones: %v", err)
		return
	} else if updated {
		outcome.DimensionsUpdated = true
	}

	// Category attributes.
	outcome.Attributes = s.writeMeliCustomAttributes(ctx, row.Code, detail, connectionID, actorID)

	// Local category id assigned by ProvisionCategoryAttributes (if any), then
	// record the reuse mapping (product's local category -> MercadoLibre
	// category) so a later publish of any product on that same local category
	// resolves it straight from ecom_channel_category_map instead of hitting
	// the category predictor.
	if refreshed, err := s.productRepository.FindByID(ctx, product.ID); err == nil && refreshed.CategoryID != nil {
		outcome.CategoryLocalID = refreshed.CategoryID
		outcome.CategoryMapping = s.recordChannelCategory(ctx, *refreshed.CategoryID, connectionID, detail.CategoryID, nil, actorID)
	}
}

// fetchMeliDescription mirrors fetchListingDescription in
// sync_mercadolibre_listings_audit.go: the per-item description first, then the
// catalog product's short_description as a fallback for catalog/user-product
// linked listings.
func (s *VecomSyncProductMigrationService) fetchMeliDescription(ctx context.Context, accessToken string, detail *mercadoLibreInfra.ItemDetail) string {
	if desc, err := s.itemsHandler.GetItemDescription(ctx, accessToken, detail.ID); err == nil {
		if text := pickMeliDescription(desc); text != "" {
			return text
		}
	} else if !errors.Is(err, mercadoLibreInfra.ErrItemDescriptionNotFound) {
		log.Printf("vecom migration: item %s description fetch failed: %v", detail.ID, err)
	}

	catalogID := strings.TrimSpace(detail.CatalogProductID)
	if catalogID == "" {
		return ""
	}
	catalog, err := s.itemsHandler.GetCatalogProduct(ctx, accessToken, catalogID)
	if err != nil {
		log.Printf("vecom migration: item %s catalog product %s fetch failed: %v", detail.ID, catalogID, err)
		return ""
	}
	return strings.TrimSpace(catalog.ShortDescription.Content)
}

// writeMeliDimensions reads the SELLER_PACKAGE_* attributes off the item and
// upserts ecom_product_dimensions (weight converted from grams to kg, sides
// kept in cm). It's the inverse of resolvePackageAttributes in
// sync_mercadolibre_products.go. Returns false (no error) when the item
// carries none of them.
func (s *VecomSyncProductMigrationService) writeMeliDimensions(ctx context.Context, detail *mercadoLibreInfra.ItemDetail, productID, actorID int64) (bool, error) {
	var height, length, width, weight string
	for _, attr := range detail.Attributes {
		num := parseLeadingNumber(attr.ValueName)
		if num == "" {
			continue
		}
		switch attr.ID {
		case "SELLER_PACKAGE_HEIGHT":
			height = num
		case "SELLER_PACKAGE_LENGTH":
			length = num
		case "SELLER_PACKAGE_WIDTH":
			width = num
		case "SELLER_PACKAGE_WEIGHT":
			if grams, err := strconv.ParseFloat(num, 64); err == nil {
				weight = strconv.FormatFloat(grams/1000.0, 'f', 3, 64)
			}
		}
	}
	if height == "" && length == "" && width == "" && weight == "" {
		return false, nil
	}

	existing, err := s.productDimensionsRepository.FindByProductID(ctx, productID)
	if errors.Is(err, mysqlInfra.ErrProductDimensionsNotFound) {
		_, cerr := s.productDimensionsRepository.Create(ctx, mysqlInfra.CreateProductDimensionsInput{
			ProductID: productID,
			Weight:    firstNonEmpty(weight, "1.00"),
			Length:    firstNonEmpty(length, "1.00"),
			Width:     firstNonEmpty(width, "1.00"),
			Height:    firstNonEmpty(height, "1.00"),
			Diameter:  "1.00",
			CreatedBy: actorID,
		})
		return cerr == nil, cerr
	}
	if err != nil {
		return false, err
	}

	_, uerr := s.productDimensionsRepository.Update(ctx, productID, mysqlInfra.UpdateProductDimensionsInput{
		Weight:    firstNonEmpty(weight, existing.Weight),
		Length:    firstNonEmpty(length, existing.Length),
		Width:     firstNonEmpty(width, existing.Width),
		Height:    firstNonEmpty(height, existing.Height),
		Diameter:  existing.Diameter,
		UpdatedBy: actorID,
	})
	return uerr == nil, uerr
}

// writeMeliCustomAttributes provisions the required-attribute slots for the
// item's MercadoLibre category (reusing
// channelAttributeValuesService.ProvisionCategoryAttributes, which also builds
// the local category and assigns it to the product) and then writes whatever
// value the item already carries for each custom_attribute slot via SetValue.
// system_field slots (BRAND, PART_NUMBER, ...) are skipped — their values come
// from the product/brand/dimensions tables. A single attribute failing is
// reported and never stops the rest.
func (s *VecomSyncProductMigrationService) writeMeliCustomAttributes(
	ctx context.Context,
	code string,
	detail *mercadoLibreInfra.ItemDetail,
	connectionID int64,
	actorID int64,
) []VecomAttributeOutcome {
	if strings.TrimSpace(detail.CategoryID) == "" {
		return nil
	}

	provisioned, err := s.channelAttributeValuesService.ProvisionCategoryAttributes(ctx, channelAttributeValuesApp.ProvisionCategoryAttributesInput{
		SKU:          code,
		CategoryID:   detail.CategoryID,
		ConnectionID: &connectionID,
		ActorID:      actorID,
	})
	if err != nil {
		return []VecomAttributeOutcome{{ExternalKey: detail.CategoryID, Error: fmt.Sprintf("provisión de atributos: %v", err)}}
	}

	byKey := make(map[string]mercadoLibreInfra.ItemAttribute, len(detail.Attributes))
	for _, attr := range detail.Attributes {
		byKey[attr.ID] = attr
	}

	outcomes := make([]VecomAttributeOutcome, 0)
	for _, provisionedAttr := range provisioned.Results {
		if provisionedAttr.Skipped || provisionedAttr.SourceType != "custom_attribute" || provisionedAttr.AttributeID == nil {
			continue
		}

		itemAttr, ok := byKey[provisionedAttr.ExternalKey]
		if !ok {
			continue
		}
		value := strings.TrimSpace(itemAttr.ValueName)
		if value == "" {
			value = strings.TrimSpace(itemAttr.ValueID)
		}
		if value == "" {
			continue
		}

		setInput := channelAttributeValuesApp.SetValueInput{
			SKU:         code,
			ExternalKey: provisionedAttr.ExternalKey,
			DataType:    provisionedAttr.DataType,
			ActorID:     actorID,
		}
		switch provisionedAttr.DataType {
		case "number":
			parsed, perr := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
			if perr != nil {
				outcomes = append(outcomes, VecomAttributeOutcome{ExternalKey: provisionedAttr.ExternalKey, Error: "valor numérico inválido: " + value})
				continue
			}
			setInput.ValueNumber = &parsed
		case "boolean":
			parsed := parseMeliBoolean(value)
			setInput.ValueBool = &parsed
		case "date":
			parsed, perr := time.Parse(time.DateOnly, value)
			if perr != nil {
				outcomes = append(outcomes, VecomAttributeOutcome{ExternalKey: provisionedAttr.ExternalKey, Error: "fecha inválida: " + value})
				continue
			}
			setInput.ValueDate = &parsed
		case "enum":
			v := value
			setInput.EnumValue = &v
		default:
			v := value
			setInput.ValueText = &v
		}

		if _, serr := s.channelAttributeValuesService.SetValue(ctx, setInput); serr != nil {
			outcomes = append(outcomes, VecomAttributeOutcome{ExternalKey: provisionedAttr.ExternalKey, Error: serr.Error()})
			continue
		}
		outcomes = append(outcomes, VecomAttributeOutcome{ExternalKey: provisionedAttr.ExternalKey, Written: true})
	}

	return outcomes
}

// resolveBrandID finds the brand by its (already uppercased) name, creating it
// when missing — same rule as brands.BrandService.resolveBrandID.
func (s *VecomSyncProductMigrationService) resolveBrandID(ctx context.Context, brandName string, actorID int64) (int64, error) {
	brand, err := s.brandsRepository.FindByName(ctx, brandName)
	if err == nil {
		return brand.ID, nil
	}
	if !errors.Is(err, mysqlInfra.ErrBrandNotFound) {
		return 0, err
	}
	created, err := s.brandsRepository.Create(ctx, mysqlInfra.CreateBrandInput{Name: brandName, CreatedBy: actorID})
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

// recordChannelCategory links a product's local category to the external
// category id of a given connection in ecom_channel_category_map, so a later
// publish of any product on that same local category resolves the external
// category straight from the map (see resolveExternalCategoryID /
// resolveOdooCategory) instead of calling the category predictor. It never
// overwrites an existing (category_id, connection_id) mapping — the first one
// recorded wins, keeping re-runs and unrelated rows stable. The returned
// string ("created" / "exists" / "conflict:<old>" / "error: ...") is only for
// the per-row report.
func (s *VecomSyncProductMigrationService) recordChannelCategory(ctx context.Context, localCategoryID, connectionID int64, externalCategoryID string, name *string, actorID int64) string {
	externalCategoryID = strings.TrimSpace(externalCategoryID)
	if localCategoryID <= 0 || externalCategoryID == "" {
		return ""
	}

	existing, err := s.channelCategoryMapRepository.FindByCategoryAndConnection(ctx, localCategoryID, connectionID)
	if err == nil {
		if strings.TrimSpace(existing.ExternalCategoryID) == externalCategoryID {
			return "exists"
		}
		return "conflict:" + strings.TrimSpace(existing.ExternalCategoryID)
	}
	if !errors.Is(err, mysqlInfra.ErrChannelCategoryMapNotFound) {
		return "error: " + err.Error()
	}

	if _, err := s.channelCategoryMapRepository.Upsert(ctx, mysqlInfra.UpsertChannelCategoryMapInput{
		CategoryID:           localCategoryID,
		ConnectionID:         connectionID,
		ExternalCategoryID:   externalCategoryID,
		ExternalCategoryName: name,
		ActorID:              actorID,
	}); err != nil {
		return "error: " + err.Error()
	}
	return "created"
}

// --- Odoo enrichment -------------------------------------------------------

// vecomOdooContext holds the per-run Odoo API client, handlers and the lazy
// category-tree resolver — built once (see buildOdooContext).
type vecomOdooContext struct {
	credentials      odooInfra.Credentials
	productsHandler  *odooInfra.ProductsHandler
	categoryResolver *vecomOdooCategoryResolver
}

func (s *VecomSyncProductMigrationService) buildOdooContext(ctx context.Context, actorID int64) (*vecomOdooContext, error) {
	values, err := syncApp.LoadOdooConnectionValues(ctx, s.credentialsRepository, s.settingsRepository, vecomOdooConnectionID)
	if err != nil {
		return nil, fmt.Errorf("error cargando conexión odoo %d: %w", vecomOdooConnectionID, err)
	}
	odooURL := values["odoo_url"]
	apiKey := values["apikey"]
	if odooURL == "" || apiKey == "" {
		return nil, fmt.Errorf("conexión odoo %d sin odoo_url / apikey", vecomOdooConnectionID)
	}
	credentials := odooInfra.Credentials{APIKey: apiKey, Database: values["x-odoo-database"]}

	client := odooInfra.NewClient(nil, odooURL, s.odooRateLimiter)
	return &vecomOdooContext{
		credentials:     credentials,
		productsHandler: odooInfra.NewProductsHandler(client),
		categoryResolver: &vecomOdooCategoryResolver{
			svc:               s,
			categoriesHandler: odooInfra.NewCategoriesHandler(client),
			credentials:       credentials,
			connectionID:      vecomOdooConnectionID,
			actorID:           actorID,
			local:             make(map[int64]int64),
		},
	}, nil
}

func (s *VecomSyncProductMigrationService) enrichOdoo(
	ctx context.Context,
	row vecomRow,
	product *mysqlInfra.ProductDTO,
	connectionID int64,
	odooCtx *vecomOdooContext,
	actorID int64,
	outcome *VecomRowOutcome,
) {
	externalID, err := strconv.ParseInt(row.ExternalID, 10, 64)
	if err != nil {
		outcome.Error = fmt.Sprintf("external_id odoo inválido %q: %v", row.ExternalID, err)
		return
	}

	odooProduct, err := odooCtx.productsHandler.GetProductByID(ctx, odooCtx.credentials, externalID)
	if err != nil {
		outcome.Error = fmt.Sprintf("error consultando producto odoo %d: %v", externalID, err)
		return
	}
	if odooProduct == nil {
		outcome.Error = fmt.Sprintf("producto odoo %d no encontrado", externalID)
		return
	}

	status := vecomMapStatusSynced
	externalCategoryID := ""
	if len(odooProduct.PublicCategIDs) > 0 {
		odooCategoryID := odooProduct.PublicCategIDs[0]
		externalCategoryID = strconv.FormatInt(odooCategoryID, 10)

		localCategoryID, err := odooCtx.categoryResolver.resolve(ctx, odooCategoryID)
		if err != nil {
			outcome.Error = fmt.Sprintf("error resolviendo categoría odoo %d: %v", odooCategoryID, err)
			return
		}

		if product.CategoryID == nil {
			if err := s.productRepository.UpdateCategoryID(ctx, product.ID, localCategoryID, actorID); err != nil {
				outcome.Error = fmt.Sprintf("error asignando categoría al producto: %v", err)
				return
			}
		}

		// Reuse mapping: the product's effective local category (its
		// pre-existing one, or the Odoo-resolved leaf just assigned) linked to
		// this connection's external category, so a later publish of any
		// product on that same local category reuses it directly.
		effectiveLocalCategoryID := localCategoryID
		if product.CategoryID != nil {
			effectiveLocalCategoryID = *product.CategoryID
		}
		outcome.CategoryLocalID = &effectiveLocalCategoryID
		outcome.CategoryMapping = s.recordChannelCategory(ctx, effectiveLocalCategoryID, connectionID, externalCategoryID, nil, actorID)
	}
	outcome.Status = status
	outcome.ExternalCategoryID = externalCategoryID

	if _, err := s.channelProductMapRepository.Upsert(ctx, mysqlInfra.UpsertChannelProductMapInput{
		ProductID:          product.ID,
		ConnectionID:       connectionID,
		ListingTitle:       row.Name,
		ExternalID:         row.ExternalID,
		ExternalCategoryID: externalCategoryID,
		Status:             status,
		ActorID:            actorID,
	}); err != nil {
		outcome.Error = fmt.Sprintf("error actualizando channel product map: %v", err)
	}
}

// vecomOdooCategoryResolver resolves an Odoo product.public.category id into a
// local ecom_categories id, replicating the branch (and its ancestors) into
// ecom_categories + ecom_channel_category_map the first time it's seen. Same
// shape as OdooCategoryMigrationService.upsertHierarchy — duplicated
// deliberately so this one-off migration stays self-contained and only
// downloads the Odoo category tree once per run.
type vecomOdooCategoryResolver struct {
	svc               *VecomSyncProductMigrationService
	categoriesHandler *odooInfra.CategoriesHandler
	credentials       odooInfra.Credentials
	connectionID      int64
	actorID           int64
	tree              map[int64]odooInfra.PublicCategory
	local             map[int64]int64
}

func (r *vecomOdooCategoryResolver) resolve(ctx context.Context, odooCategoryID int64) (int64, error) {
	if id, ok := r.local[odooCategoryID]; ok {
		return id, nil
	}

	existing, err := r.svc.channelCategoryMapRepository.FindByExternalCategoryAndConnection(ctx, strconv.FormatInt(odooCategoryID, 10), r.connectionID)
	if err == nil {
		r.local[odooCategoryID] = existing.CategoryID
		return existing.CategoryID, nil
	}
	if !errors.Is(err, mysqlInfra.ErrChannelCategoryMapNotFound) {
		return 0, err
	}

	if r.tree == nil {
		categories, err := r.categoriesHandler.SearchReadPublicCategories(ctx, odooInfra.SearchReadPublicCategoriesRequest{Credentials: r.credentials})
		if err != nil {
			return 0, fmt.Errorf("error listando categorías odoo: %w", err)
		}
		r.tree = make(map[int64]odooInfra.PublicCategory, len(categories))
		for _, category := range categories {
			r.tree[category.ID] = category
		}
	}

	return r.resolveNode(ctx, odooCategoryID, map[int64]bool{})
}

func (r *vecomOdooCategoryResolver) resolveNode(ctx context.Context, odooCategoryID int64, visiting map[int64]bool) (int64, error) {
	if id, ok := r.local[odooCategoryID]; ok {
		return id, nil
	}
	if visiting[odooCategoryID] {
		return 0, fmt.Errorf("jerarquía cíclica en categoría odoo %d", odooCategoryID)
	}
	visiting[odooCategoryID] = true
	defer delete(visiting, odooCategoryID)

	node, ok := r.tree[odooCategoryID]
	if !ok {
		return 0, fmt.Errorf("categoría odoo %d no está en el árbol descargado", odooCategoryID)
	}
	name := strings.TrimSpace(node.Name)
	if name == "" {
		return 0, fmt.Errorf("categoría odoo %d sin nombre", odooCategoryID)
	}

	var parentLocalID *int64
	if node.ParentID.Valid {
		resolvedParent, err := r.resolveNode(ctx, node.ParentID.ID, visiting)
		if err != nil {
			return 0, err
		}
		parentLocalID = &resolvedParent
	}

	var localID int64
	existing, err := r.svc.categoriesRepository.FindByNameAndParentID(ctx, name, parentLocalID)
	if err != nil && !errors.Is(err, mysqlInfra.ErrCategoryNotFound) {
		return 0, fmt.Errorf("error buscando categoría %q: %w", name, err)
	}
	if existing != nil {
		localID = existing.ID
	} else {
		created, err := r.svc.categoriesRepository.Create(ctx, mysqlInfra.CreateCategoryInput{Name: name, ParentID: parentLocalID, CreatedBy: r.actorID})
		if err != nil {
			return 0, fmt.Errorf("error creando categoría %q: %w", name, err)
		}
		localID = created.ID
	}

	nameCopy := name
	if _, err := r.svc.channelCategoryMapRepository.Upsert(ctx, mysqlInfra.UpsertChannelCategoryMapInput{
		CategoryID:           localID,
		ConnectionID:         r.connectionID,
		ExternalCategoryID:   strconv.FormatInt(odooCategoryID, 10),
		ExternalCategoryName: &nameCopy,
		ActorID:              r.actorID,
	}); err != nil {
		return 0, fmt.Errorf("error registrando channel category map para %q: %w", name, err)
	}

	r.local[odooCategoryID] = localID
	return localID, nil
}

// --- helpers -------------------------------------------------------------

func connectionIDForIntegration(integrationID int64) (int64, bool) {
	switch integrationID {
	case vecomOdooIntegrationID:
		return vecomOdooConnectionID, true
	case vecomMeliIntegrationID:
		return vecomMeliConnectionID, true
	default:
		return 0, false
	}
}

func sourceIDForCompany(companyID int64) (int64, bool) {
	switch companyID {
	case vecomNissanCompanyID:
		return vecomNissanSourceID, true
	case 4, 7:
		return vecomDynamicsSourceID, true
	default:
		return 0, false
	}
}

// foldMeliStatus mirrors foldMercadoLibreStatus in sync_mercadolibre_products.go
// (unexported there) / foldSyncItemMeliMigrationStatus.
func foldMeliStatus(rawStatus string, subStatus []string) string {
	for _, s := range subStatus {
		if strings.EqualFold(strings.TrimSpace(s), vecomMeliSubStatusForbidden) {
			return "closed"
		}
	}
	switch strings.ToLower(strings.TrimSpace(rawStatus)) {
	case vecomMeliStatusUnderReview:
		return "under_review"
	case vecomMeliStatusPaused:
		return "paused"
	default:
		return "synced"
	}
}

func meliAttributeValue(attributes []mercadoLibreInfra.ItemAttribute, id string) string {
	for _, attr := range attributes {
		if attr.ID == id {
			if v := strings.TrimSpace(attr.ValueName); v != "" {
				return v
			}
		}
	}
	return ""
}

func pickMeliDescription(desc *mercadoLibreInfra.ItemDescription) string {
	if desc == nil {
		return ""
	}
	if text := strings.TrimSpace(desc.PlainText); text != "" {
		return text
	}
	return strings.TrimSpace(desc.Text)
}

func parseMeliBoolean(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "sí", "si", "yes", "true", "1":
		return true
	default:
		return false
	}
}

// parseLeadingNumber extracts the leading numeric part of a MercadoLibre
// package attribute value like "10 cm" / "1000 g" / "12.5 cm" as a plain
// decimal string ("" when there is none).
func parseLeadingNumber(raw string) string {
	raw = strings.TrimSpace(raw)
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == ',':
			b.WriteRune('.')
		case r == ' ':
			if b.Len() == 0 {
				continue
			}
			return b.String()
		default:
			return b.String()
		}
	}
	return b.String()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
