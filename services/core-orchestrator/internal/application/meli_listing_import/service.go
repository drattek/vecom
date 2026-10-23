// Package meli_listing_import links a listing that already exists on
// MercadoLibre to a local product, from the product detail page's
// "Sincronización" tab ("Importar una publicación existente").
//
// Given a connection and a MercadoLibre listing id, Import checks that the
// listing belongs to the connection's account and that its PART_NUMBER
// attribute matches the product's part number; if so it copies the listing's
// data into the local product (name, category link, description, dimensions and
// category attributes, vehicle compatibilities), records the listing in
// ecom_channel_product_map, and — only when the product currently has stock and
// a price — pushes the current price/stock back and appends the brand to the
// title (see sync.MercadoLibreProductSyncService.PushImportedListing).
//
// Nothing is written to MercadoLibre before every local step has succeeded.
package meli_listing_import

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	channelAttributeValuesApp "core-orchestrator/internal/application/channel_attribute_values"
	productCategorySelectionApp "core-orchestrator/internal/application/product_category_selection"
	productImageImportApp "core-orchestrator/internal/application/product_image_import"
	syncApp "core-orchestrator/internal/application/sync"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// importTimeout bounds a whole import, including pictures. It runs in the
// background (see Start), not tied to the HTTP request that started it.
const importTimeout = 35 * time.Minute

// compatibilitiesTimeout bounds the compatibilities step on its own. It is the
// slowest step by far: every compatibility is resolved with one MercadoLibre
// catalog request, and those share the platform-wide budget of
// MercadoLibreRateLimit calls per minute (90), so a part that fits 1,300 vehicles
// needs at least ~15 minutes. It runs last, with its own budget, so it can never
// starve the steps that matter more (linking, price/stock, images).
const compatibilitiesTimeout = 20 * time.Minute

const (
	mercadoLibreChannelCode = "MERCADOLIBRE"
	partNumberAttributeID   = "PART_NUMBER"
	meliStatusClosed        = "closed"
	meliMapStatusPaused     = "paused"
	meliMapStatusClosed     = "closed"
	pausedReasonUser        = "user"
)

// Step codes and statuses reported in Result.Steps. The dashboard maps the
// codes to Spanish labels.
const (
	StepName            = "name"
	StepCategory        = "category"
	StepDescription     = "description"
	StepAttributes      = "attributes"
	StepCompatibilities = "compatibilities"
	StepImages          = "images"
	StepLink            = "link"
	StepPriceStock      = "price_stock"
	StepListingType     = "listing_type"
	StepOfficialStore   = "official_store"
	StepTitle           = "title"

	StatusOK      = "ok"
	StatusSkipped = "skipped"
	StatusWarning = "warning"
	StatusError   = "error"
)

var (
	ErrInvalidInput       = errors.New("productId, connectionId and externalId are required")
	ErrInvalidExternalID  = errors.New("the MercadoLibre listing id is not valid")
	ErrProductNotFound    = mysqlInfra.ErrProductNotFound
	ErrConnectionNotFound = mysqlInfra.ErrChannelConnectionNotFound

	ErrProductAlreadyListed        = errors.New("the product already has an active listing on this connection")
	ErrListingLinkedToOtherProduct = errors.New("the listing is already linked to another product")
	ErrListingNotFound             = errors.New("the listing does not exist on MercadoLibre")
	ErrListingNotOwned             = errors.New("the listing does not belong to this connection's MercadoLibre account")
	ErrListingClosed               = errors.New("the listing is closed on MercadoLibre and can't be imported")
	ErrListingFetch                = errors.New("could not fetch the listing from MercadoLibre")
)

// PartNumberMismatchError is returned when the listing's PART_NUMBER attribute
// doesn't contain the product's part number.
type PartNumberMismatchError struct {
	ProductPartNumber  string
	ListingPartNumbers []string
}

func (e *PartNumberMismatchError) Error() string {
	if len(e.ListingPartNumbers) == 0 {
		return fmt.Sprintf("the listing has no PART_NUMBER attribute to compare with %q", e.ProductPartNumber)
	}
	return fmt.Sprintf("the listing's part number (%s) doesn't match the product's (%q)", strings.Join(e.ListingPartNumbers, ", "), e.ProductPartNumber)
}

type Service struct {
	productRepository             *mysqlInfra.ProductRepository
	productDimensionsRepository   *mysqlInfra.ProductDimensionsRepository
	channelRepository             *mysqlInfra.ChannelRepository
	channelConnectionRepository   *mysqlInfra.ChannelConnectionRepository
	channelProductMapRepository   *mysqlInfra.ChannelProductMapRepository
	tokenService                  *syncApp.MercadoLibreTokenService
	productSyncService            *syncApp.MercadoLibreProductSyncService
	compatibilityService          *syncApp.MercadoLibreCompatibilityService
	categorySelectionService      *productCategorySelectionApp.Service
	channelAttributeValuesService *channelAttributeValuesApp.Service
	// Pictures: downloaded, converted to WebP and uploaded to the Azure Blob
	// container configured on the AZURE_BLOB storage disk (see importImages).
	filesRepository                  *mysqlInfra.FilesRepository
	storageDiskRepository            *mysqlInfra.StorageDiskRepository
	storageDiskCredentialsRepository *mysqlInfra.StorageDiskCredentialsRepository
	productImageImportService        *productImageImportApp.Service
	pictureClient                    *http.Client
	jobs                             JobStore
	itemsHandler                 *mercadoLibreInfra.ItemsHandler
	usersHandler                  *mercadoLibreInfra.UsersHandler
}

func NewService(
	productRepository *mysqlInfra.ProductRepository,
	productDimensionsRepository *mysqlInfra.ProductDimensionsRepository,
	channelRepository *mysqlInfra.ChannelRepository,
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository,
	channelProductMapRepository *mysqlInfra.ChannelProductMapRepository,
	tokenService *syncApp.MercadoLibreTokenService,
	productSyncService *syncApp.MercadoLibreProductSyncService,
	compatibilityService *syncApp.MercadoLibreCompatibilityService,
	categorySelectionService *productCategorySelectionApp.Service,
	channelAttributeValuesService *channelAttributeValuesApp.Service,
	filesRepository *mysqlInfra.FilesRepository,
	storageDiskRepository *mysqlInfra.StorageDiskRepository,
	storageDiskCredentialsRepository *mysqlInfra.StorageDiskCredentialsRepository,
	productImageImportService *productImageImportApp.Service,
	jobs JobStore,
	rateLimiter *mercadoLibreInfra.RateLimiter,
) *Service {
	client := mercadoLibreInfra.NewClient(nil, "", rateLimiter)

	return &Service{
		productRepository:             productRepository,
		productDimensionsRepository:   productDimensionsRepository,
		channelRepository:             channelRepository,
		channelConnectionRepository:   channelConnectionRepository,
		channelProductMapRepository:   channelProductMapRepository,
		tokenService:                  tokenService,
		productSyncService:            productSyncService,
		compatibilityService:          compatibilityService,
		categorySelectionService:      categorySelectionService,
		channelAttributeValuesService: channelAttributeValuesService,

		filesRepository:                  filesRepository,
		storageDiskRepository:            storageDiskRepository,
		storageDiskCredentialsRepository: storageDiskCredentialsRepository,
		productImageImportService:        productImageImportService,
		pictureClient:                    &http.Client{Timeout: 30 * time.Second},
		jobs:                             jobs,
		itemsHandler:                 mercadoLibreInfra.NewItemsHandler(client),
		usersHandler:                  mercadoLibreInfra.NewUsersHandler(client),
	}
}

type ImportInput struct {
	ProductID    int64
	ConnectionID int64
	// ExternalID is whatever the user pasted: "MLM123456789", "123456789", or
	// the listing's URL — see NormalizeItemID.
	ExternalID string
	ActorID    int64
}

// StepResult reports what happened in one phase of the import.
type StepResult struct {
	Step   string `json:"step"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// Result is returned once the listing passed validation and the import ran;
// individual phases that failed are reported in Steps (Linked says whether the
// listing ended up recorded in ecom_channel_product_map).
type Result struct {
	ProductID    int64        `json:"productId"`
	ConnectionID int64        `json:"connectionId"`
	ExternalID   string       `json:"externalId"`
	Title        string       `json:"title"`
	CategoryID   string       `json:"categoryId"`
	Status       string       `json:"status"`
	Linked       bool         `json:"linked"`
	Steps        []StepResult `json:"steps"`
}

func recordNormalize(normalize *syncApp.ImportedListingNormalizeResult, record func(step, status, detail string)) {
	switch {
	case normalize.ListingTypeError != "":
		record(StepListingType, StatusWarning, normalize.ListingTypeError)
	case normalize.ListingTypeChanged:
		record(StepListingType, StatusOK, fmt.Sprintf("Tipo de publicación cambiado de %q a Premium.", normalize.ListingTypeFrom))
	default:
		record(StepListingType, StatusSkipped, normalize.ListingTypeSkippedReason)
	}

	switch {
	case normalize.OfficialStoreError != "":
		record(StepOfficialStore, StatusWarning, normalize.OfficialStoreError)
	case normalize.OfficialStoreRemoved:
		record(StepOfficialStore, StatusOK, fmt.Sprintf("Publicación sacada de la tienda oficial %d.", normalize.OfficialStoreID))
	default:
		record(StepOfficialStore, StatusSkipped, normalize.OfficialStoreSkippedReason)
	}
}

func (s *Service) requireMercadoLibreConnection(ctx context.Context, connectionID int64) error {
	connection, err := s.channelConnectionRepository.FindByID(ctx, connectionID)
	if err != nil {
		return err
	}
	channel, err := s.channelRepository.FindByID(ctx, connection.ChannelID)
	if err != nil {
		return fmt.Errorf("error loading channel %d: %w", connection.ChannelID, err)
	}
	if !strings.EqualFold(strings.TrimSpace(channel.Code), mercadoLibreChannelCode) {
		return syncApp.ErrNotMercadoLibreConnection
	}
	return nil
}

func recordPush(push *syncApp.ImportedListingPushResult, record func(step, status, detail string)) {
	switch {
	case push.PriceStockError != "":
		record(StepPriceStock, StatusError, push.PriceStockError)
	case push.PriceStockUpdated:
		record(StepPriceStock, StatusOK, fmt.Sprintf("Precio %.2f MXN y stock %d enviados a MercadoLibre.", push.Price, push.Quantity))
	default:
		record(StepPriceStock, StatusSkipped, push.PriceStockSkippedReason)
	}

	switch {
	case push.TitleError != "":
		// MercadoLibre refusing a title (e.g. the item already has sales) is
		// expected sometimes; the import itself is complete.
		record(StepTitle, StatusWarning, push.TitleError)
	case push.TitleUpdated:
		record(StepTitle, StatusOK, fmt.Sprintf("Título actualizado a %q.", push.Title))
	default:
		record(StepTitle, StatusSkipped, push.TitleSkippedReason)
	}
}

// --- local steps -------------------------------------------------------------

func (s *Service) importName(ctx context.Context, product *mysqlInfra.ProductDTO, item *mercadoLibreInfra.ItemDetail, actorID int64, record func(step, status, detail string)) {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		record(StepName, StatusSkipped, "La publicación no tiene título.")
		return
	}
	if err := s.productRepository.UpdateName(ctx, product.ID, title, actorID); err != nil {
		record(StepName, StatusError, fmt.Sprintf("No se pudo actualizar el nombre: %v", err))
		return
	}
	record(StepName, StatusOK, fmt.Sprintf("Nombre actualizado a %q.", title))
}

// importCategory links the listing's MercadoLibre category to the connection
// (creating the local category and seeding its attribute slots when needed).
// It returns nil when that failed, which makes importAttributes skip.
func (s *Service) importCategory(ctx context.Context, product *mysqlInfra.ProductDTO, item *mercadoLibreInfra.ItemDetail, input ImportInput, record func(step, status, detail string)) *productCategorySelectionApp.SelectionDTO {
	if strings.TrimSpace(item.CategoryID) == "" {
		record(StepCategory, StatusError, "La publicación no tiene categoría.")
		return nil
	}
	selection, err := s.categorySelectionService.Select(ctx, product.ID, input.ConnectionID, item.CategoryID, input.ActorID)
	if err != nil {
		record(StepCategory, StatusError, fmt.Sprintf("No se pudo vincular la categoría %s: %v", item.CategoryID, err))
		return nil
	}
	record(StepCategory, StatusOK, fmt.Sprintf("Categoría %s (%s) vinculada a la conexión.", selection.ExternalCategoryName, selection.ExternalCategoryID))
	return selection
}

func (s *Service) importDescription(ctx context.Context, product *mysqlInfra.ProductDTO, item *mercadoLibreInfra.ItemDetail, accessToken string, actorID int64, record func(step, status, detail string)) {
	text := s.fetchDescription(ctx, accessToken, item)
	if text == "" {
		record(StepDescription, StatusSkipped, "La publicación no tiene descripción; se conservó la local.")
		return
	}
	if err := s.productRepository.UpdateDescription(ctx, product.ID, text, actorID); err != nil {
		record(StepDescription, StatusError, fmt.Sprintf("No se pudo guardar la descripción: %v", err))
		return
	}
	record(StepDescription, StatusOK, "Descripción de la publicación guardada.")
}

// fetchDescription returns the listing's own description or, for catalog /
// user-product listings that have none, the catalog product's short
// description.
func (s *Service) fetchDescription(ctx context.Context, accessToken string, item *mercadoLibreInfra.ItemDetail) string {
	desc, err := s.itemsHandler.GetItemDescription(ctx, accessToken, item.ID)
	if err == nil {
		if text := strings.TrimSpace(desc.PlainText); text != "" {
			return text
		}
		if text := strings.TrimSpace(desc.Text); text != "" {
			return text
		}
	} else if !errors.Is(err, mercadoLibreInfra.ErrItemDescriptionNotFound) {
		log.Printf("meli listing import: item %s description fetch failed: %v", item.ID, err)
	}

	catalogID := strings.TrimSpace(item.CatalogProductID)
	if catalogID == "" {
		return ""
	}
	catalog, err := s.itemsHandler.GetCatalogProduct(ctx, accessToken, catalogID)
	if err != nil {
		log.Printf("meli listing import: item %s catalog product %s fetch failed: %v", item.ID, catalogID, err)
		return ""
	}
	return strings.TrimSpace(catalog.ShortDescription.Content)
}

func (s *Service) importAttributes(ctx context.Context, product *mysqlInfra.ProductDTO, item *mercadoLibreInfra.ItemDetail, selection *productCategorySelectionApp.SelectionDTO, input ImportInput, record func(step, status, detail string)) {
	if selection == nil {
		record(StepAttributes, StatusSkipped, "No se guardaron atributos porque la categoría no se pudo vincular.")
		return
	}

	dimensionsUpdated, err := s.writeDimensions(ctx, item, product.ID, input.ActorID)
	if err != nil {
		record(StepAttributes, StatusError, fmt.Sprintf("No se pudieron guardar las dimensiones: %v", err))
		return
	}

	// Slots are scoped to the category Select just resolved for this
	// connection, not to the product's own catalog category.
	localCategoryID := selection.CategoryID
	provisioned, err := s.channelAttributeValuesService.ProvisionCategoryAttributes(ctx, channelAttributeValuesApp.ProvisionCategoryAttributesInput{
		SKU:                     product.SKU,
		CategoryID:              item.CategoryID,
		ConnectionID:            &input.ConnectionID,
		ActorID:                 input.ActorID,
		LocalCategoryIDOverride: &localCategoryID,
	})
	if err != nil {
		record(StepAttributes, StatusError, fmt.Sprintf("No se pudieron preparar los atributos de la categoría %s: %v", item.CategoryID, err))
		return
	}

	byKey := make(map[string]mercadoLibreInfra.ItemAttribute, len(item.Attributes))
	for _, attr := range item.Attributes {
		byKey[attr.ID] = attr
	}

	written := 0
	var problems []string
	for _, slot := range provisioned.Results {
		// system_field slots (BRAND, PART_NUMBER, SELLER_SKU, ...) are filled
		// from the product / brand / dimensions tables, not stored here.
		if slot.Skipped || slot.SourceType != "custom_attribute" || slot.AttributeID == nil {
			continue
		}
		// Package measures live in ecom_product_dimensions (writeDimensions),
		// never as attribute values.
		if strings.HasPrefix(slot.ExternalKey, "SELLER_PACKAGE_") {
			continue
		}
		attr, ok := byKey[slot.ExternalKey]
		if !ok {
			continue
		}
		value := strings.TrimSpace(attr.ValueName)
		if value == "" {
			value = strings.TrimSpace(attr.ValueID)
		}
		if value == "" {
			continue
		}

		setInput, err := buildSetValueInput(product.SKU, input.ConnectionID, slot.ExternalKey, slot.DataType, value, input.ActorID)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", slot.ExternalKey, err))
			continue
		}
		if _, err := s.channelAttributeValuesService.SetValue(ctx, setInput); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", slot.ExternalKey, err))
			continue
		}
		written++
	}

	detail := fmt.Sprintf("%d atributo(s) guardados", written)
	if dimensionsUpdated {
		detail += " y dimensiones del paquete actualizadas"
	}
	detail += "."
	if len(problems) > 0 {
		record(StepAttributes, StatusWarning, detail+" No se pudieron guardar: "+strings.Join(problems, "; "))
		return
	}
	record(StepAttributes, StatusOK, detail)
}

// buildSetValueInput converts value (as MercadoLibre reports it) into the typed
// field SetValue expects for dataType.
func buildSetValueInput(sku string, connectionID int64, externalKey, dataType, value string, actorID int64) (channelAttributeValuesApp.SetValueInput, error) {
	input := channelAttributeValuesApp.SetValueInput{
		SKU:          sku,
		ConnectionID: &connectionID,
		ExternalKey:  externalKey,
		DataType:     dataType,
		ActorID:      actorID,
	}
	switch dataType {
	case "number":
		parsed, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
		if err != nil {
			return input, fmt.Errorf("valor numérico inválido %q", value)
		}
		input.ValueNumber = &parsed
	case "boolean":
		parsed := parseBoolean(value)
		input.ValueBool = &parsed
	case "date":
		parsed, err := time.Parse(time.DateOnly, value)
		if err != nil {
			return input, fmt.Errorf("fecha inválida %q", value)
		}
		input.ValueDate = &parsed
	case "enum":
		input.EnumValue = &value
	default:
		input.ValueText = &value
	}
	return input, nil
}

// writeDimensions reads the SELLER_PACKAGE_* attributes off the item and
// upserts ecom_product_dimensions (weight converted from grams to kg, sides in
// cm) — the inverse of sync's resolvePackageAttributes. It returns false (no
// error) when the item carries none of them.
func (s *Service) writeDimensions(ctx context.Context, item *mercadoLibreInfra.ItemDetail, productID, actorID int64) (bool, error) {
	var height, length, width, weight string
	for _, attr := range item.Attributes {
		switch attr.ID {
		case "SELLER_PACKAGE_HEIGHT":
			height = formatMeasure(attr.ValueName, lengthToCM, 2)
		case "SELLER_PACKAGE_LENGTH":
			length = formatMeasure(attr.ValueName, lengthToCM, 2)
		case "SELLER_PACKAGE_WIDTH":
			width = formatMeasure(attr.ValueName, lengthToCM, 2)
		case "SELLER_PACKAGE_WEIGHT":
			weight = formatMeasure(attr.ValueName, weightToKG, 3)
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

// importCompatibilities copies the vehicle compatibilities the seller
// registered on the listing. A listing without any is normal and reported as
// skipped; an unexpected failure is only a warning, so it never keeps the rest
// of the listing from being imported.
func (s *Service) importCompatibilities(ctx context.Context, product *mysqlInfra.ProductDTO, externalID, accessToken string, actorID int64, record func(step, status, detail string)) {
	outcomes, newFitments, err := s.compatibilityService.CopyCompatibilitiesForProduct(ctx, accessToken, externalID, product.ID, actorID)
	if err != nil {
		// A listing with no compatibilities (or in a category that doesn't take
		// them) is answered with a 404: nothing to save, not a failure.
		if strings.Contains(err.Error(), "(404)") {
			record(StepCompatibilities, StatusSkipped, "La publicación no tiene compatibilidades registradas.")
			return
		}
		record(StepCompatibilities, StatusWarning, fmt.Sprintf("No se pudieron consultar las compatibilidades: %v", err))
		return
	}
	if len(outcomes) == 0 {
		record(StepCompatibilities, StatusSkipped, "La publicación no tiene compatibilidades registradas.")
		return
	}

	// Each entry ends up linked, already linked from before, left out because
	// MercadoLibre manages it (no individual vehicle), or failed. Counting them
	// apart is what makes "0 vinculadas" readable: a re-run reports everything
	// as "ya estaba", which is not the same as nothing having worked.
	var linked, alreadyLinked, catalogManaged, failed int
	errorSamples := map[string]int{}
	for _, outcome := range outcomes {
		switch {
		case outcome.Error != "":
			failed++
			errorSamples[compatibilityErrorSummary(outcome.Error)]++
		case outcome.CompatibilityID != 0:
			linked++
		case strings.Contains(outcome.SkipReason, "already linked"):
			alreadyLinked++
		default:
			catalogManaged++
		}
	}

	detail := fmt.Sprintf("%d compatibilidad(es) en la publicación: %d nuevas vinculadas, %d ya estaban vinculadas, %d administradas por MercadoLibre sin vehículo individual, %d con error (%d vehículo(s) nuevos creados).",
		len(outcomes), linked, alreadyLinked, catalogManaged, failed, len(newFitments))
	if failed == 0 {
		record(StepCompatibilities, StatusOK, detail)
		return
	}

	causes := make([]string, 0, len(errorSamples))
	for cause, count := range errorSamples {
		causes = append(causes, fmt.Sprintf("%s (%d)", cause, count))
	}
	sort.Strings(causes)
	if len(causes) > 3 {
		causes = causes[:3]
	}
	if ctx.Err() != nil {
		detail += " Se agotó el tiempo disponible para este paso; vuelve a importar para continuar con las que faltan."
	}
	record(StepCompatibilities, StatusWarning, detail+" Causas: "+strings.Join(causes, "; "))
}

// compatibilityErrorSummary shortens a per-entry compatibility error to its
// cause, dropping the ids and raw payloads that make every message unique, so
// failures of the same kind can be counted together.
func compatibilityErrorSummary(message string) string {
	switch {
	case strings.Contains(message, "context deadline exceeded"):
		return "se agotó el tiempo"
	case strings.Contains(message, "context canceled"):
		return "cancelado"
	case strings.Contains(message, "(429)"):
		return "MercadoLibre limitó las consultas (429)"
	case strings.Contains(message, "(404)"):
		return "vehículo no encontrado en el catálogo de MercadoLibre (404)"
	case strings.Contains(message, "missing brand/model/year"):
		return "el producto de catálogo no trae marca, modelo o año"
	}
	if i := strings.Index(message, ":"); i > 0 && i < 80 {
		return message[:i]
	}
	if len(message) > 80 {
		return message[:80]
	}
	return message
}

// --- helpers -----------------------------------------------------------------

var (
	itemIDPattern     = regexp.MustCompile(`(?i)MLM-?(\d{6,})`)
	digitsOnlyPattern = regexp.MustCompile(`^\d{6,}$`)
)

// NormalizeItemID turns what a user pastes — "MLM123456789", "MLM-123456789",
// the bare number, or the listing's URL — into the canonical item id
// ("MLM123456789"). Only the MX site is supported.
func NormalizeItemID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if m := itemIDPattern.FindStringSubmatch(raw); m != nil {
		return "MLM" + m[1], nil
	}
	if digitsOnlyPattern.MatchString(raw) {
		return "MLM" + raw, nil
	}
	return "", ErrInvalidExternalID
}

// validatePartNumber checks that one of the listing's PART_NUMBER values equals
// the product's part number, ignoring case and any punctuation or spacing
// ("26310-2J000" == "263102j000"). The attribute may hold several numbers
// separated by commas, semicolons or pipes.
func validatePartNumber(productPartNumber string, attributes []mercadoLibreInfra.ItemAttribute) error {
	want := normalizePartNumber(productPartNumber)

	var listingNumbers []string
	for _, attr := range attributes {
		if attr.ID != partNumberAttributeID {
			continue
		}
		for _, number := range strings.FieldsFunc(attr.ValueName, func(r rune) bool {
			return r == ',' || r == ';' || r == '|' || r == '\n'
		}) {
			if number = strings.TrimSpace(number); number != "" {
				listingNumbers = append(listingNumbers, number)
			}
		}
	}

	if want != "" {
		for _, number := range listingNumbers {
			if normalizePartNumber(number) == want {
				return nil
			}
		}
	}
	return &PartNumberMismatchError{ProductPartNumber: productPartNumber, ListingPartNumbers: listingNumbers}
}

func normalizePartNumber(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func parseBoolean(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "sí", "si", "yes", "true", "1":
		return true
	default:
		return false
	}
}

var measurePattern = regexp.MustCompile(`^\s*([0-9]+(?:[.,][0-9]+)?)\s*([^\s0-9]*)`)

// lengthToCM and weightToKG convert the unit MercadoLibre reports a package
// measure in (empty means its default: cm for sides, g for weight) into what
// ecom_product_dimensions stores — cm and kg. ok is false for an unknown unit.
func lengthToCM(unit string) (factor float64, ok bool) {
	switch strings.ToLower(unit) {
	case "", "cm":
		return 1, true
	case "mm":
		return 0.1, true
	case "m":
		return 100, true
	case "in", "\"", "pulgadas":
		return 2.54, true
	}
	return 0, false
}

func weightToKG(unit string) (factor float64, ok bool) {
	switch strings.ToLower(unit) {
	case "", "g":
		return 0.001, true
	case "kg":
		return 1, true
	case "mg":
		return 0.000001, true
	case "lb":
		return 0.45359237, true
	case "oz":
		return 0.0283495231, true
	}
	return 0, false
}

// formatMeasure parses a MercadoLibre package value like "10 cm", "1000 g" or
// "0,5 kg" and returns it converted with convert, as a decimal string with the
// given number of decimals. It returns "" when the value has no number or its
// unit is unknown, which leaves the stored dimension untouched.
func formatMeasure(raw string, convert func(unit string) (float64, bool), decimals int) string {
	m := measurePattern.FindStringSubmatch(raw)
	if m == nil {
		return ""
	}
	number, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
	if err != nil {
		return ""
	}
	factor, ok := convert(m[2])
	if !ok {
		return ""
	}
	return strconv.FormatFloat(number*factor, 'f', decimals, 64)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
