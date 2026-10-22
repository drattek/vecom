package sync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// mercadoLibreMaxTitleLength is MercadoLibre's limit on an item title /
// family_name for the MX site.
const mercadoLibreMaxTitleLength = 60

// FoldMercadoLibreItemStatus exposes foldMercadoLibreStatus to
// meli_listing_import, which can't live in this package (it depends on
// product_category_selection, which already imports this one).
func FoldMercadoLibreItemStatus(rawStatus string, subStatus []string) string {
	return foldMercadoLibreStatus(rawStatus, subStatus)
}

// ImportedListingPushResult reports what PushImportedListing sent to
// MercadoLibre after an existing listing was linked to a local product. The
// *SkippedReason fields explain a deliberate no-op (nothing to send, or a rule
// said to leave the listing alone); the *Error fields are real failures.
type ImportedListingPushResult struct {
	PriceStockUpdated       bool    `json:"priceStockUpdated"`
	PriceStockSkippedReason string  `json:"priceStockSkippedReason,omitempty"`
	PriceStockError         string  `json:"priceStockError,omitempty"`
	Price                   float64 `json:"price,omitempty"`
	Quantity                int     `json:"quantity,omitempty"`

	TitleUpdated       bool   `json:"titleUpdated"`
	TitleSkippedReason string `json:"titleSkippedReason,omitempty"`
	TitleError         string `json:"titleError,omitempty"`
	Title              string `json:"title,omitempty"`
}

// PushImportedListing is the last phase of importing an already-existing
// MercadoLibre listing (see meli_listing_import.Service): once its data has
// been saved locally and listing recorded in ecom_channel_product_map, and only
// if the product currently has both stock and a price, it
//
//  1. pushes the product's current price (through the same pricing formula,
//     price list and pinned-SKU rules as every other price push — see
//     resolvePriceStockAndToken) and total stock, and
//  2. appends the brand to the listing's title (see ComposeBrandedTitle).
//
// Unlike Refresh it is never gated on "did price/stock change since the last
// sync" — an import always pushes. It leaves a paused/under_review/closed
// listing alone, same as every other sync path (a human or MercadoLibre put it
// there). Only price and quantity are sent: the package attributes
// updateExistingListings adds would overwrite whatever the seller had on the
// listing with generic defaults when the product has no dimensions of its own.
// A title MercadoLibre refuses (items with sales can't change it) is reported
// in TitleError but never fails the import.
func (s *MercadoLibreProductSyncService) PushImportedListing(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, listing *mysqlInfra.ChannelProductMapDTO, item *mercadoLibreInfra.ItemDetail) *ImportedListingPushResult {
	result := &ImportedListingPushResult{}
	externalID := derefString(listing.ExternalID)

	if isPausedOrUnderReview(listing.Status) {
		reason := fmt.Sprintf("la publicación está en estado %q en MercadoLibre; no se modifica", listing.Status)
		result.PriceStockSkippedReason = reason
		result.TitleSkippedReason = reason
		return result
	}

	finalPrice, qtyAvailable, accessToken, err := s.resolvePriceStockAndToken(ctx, product.ID, product.SKU, product.BrandID, connectionID)
	switch {
	case errors.Is(err, mysqlInfra.ErrProductPriceNotFound):
		result.PriceStockSkippedReason = "el producto no tiene precio"
		result.TitleSkippedReason = result.PriceStockSkippedReason
		return result
	case err != nil:
		result.PriceStockError = err.Error()
		result.TitleSkippedReason = "no se pudo resolver el precio y stock del producto"
		return result
	}
	if finalPrice <= 0 {
		result.PriceStockSkippedReason = "el producto no tiene precio"
		result.TitleSkippedReason = result.PriceStockSkippedReason
		return result
	}
	if qtyAvailable <= 0 {
		result.PriceStockSkippedReason = "el producto no tiene stock"
		result.TitleSkippedReason = result.PriceStockSkippedReason
		return result
	}

	if err := s.itemsHandler.UpdateItem(ctx, mercadoLibreInfra.UpdateItemRequest{
		AccessToken: accessToken,
		ExternalID:  externalID,
		Vals: mercadoLibreInfra.UpdateItemVals{
			Price:             finalPrice,
			AvailableQuantity: qtyAvailable,
		},
	}); err != nil {
		result.PriceStockError = fmt.Sprintf("error actualizando precio y stock en MercadoLibre: %v", err)
	} else if _, err := s.channelProductMapRepository.Upsert(ctx, mysqlInfra.UpsertChannelProductMapInput{
		ProductID:          product.ID,
		ConnectionID:       connectionID,
		VehicleFitmentID:   listing.VehicleFitmentID,
		ListingTitle:       derefString(listing.ListingTitle),
		ExternalID:         externalID,
		ExternalCategoryID: derefString(listing.ExternalCategoryID),
		Status:             mercadoLibreMapStatusSynced,
		ActorID:            systemMercadoLibreSyncActorID,
	}); err != nil {
		result.PriceStockError = fmt.Sprintf("precio y stock enviados, pero no se pudo registrar la sincronización: %v", err)
	} else {
		result.PriceStockUpdated = true
		result.Price = finalPrice
		result.Quantity = qtyAvailable
	}

	s.pushBrandedTitle(ctx, product, item, externalID, accessToken, result)

	return result
}

// pushBrandedTitle appends the product's brand to the listing's current title
// and sends it to MercadoLibre, unless ComposeBrandedTitle says there is
// nothing to do. Outcomes are written into result.
func (s *MercadoLibreProductSyncService) pushBrandedTitle(ctx context.Context, product *mysqlInfra.ProductDTO, item *mercadoLibreInfra.ItemDetail, externalID, accessToken string, result *ImportedListingPushResult) {
	if product.BrandID == nil {
		result.TitleSkippedReason = "el producto no tiene marca"
		return
	}
	brand, err := s.brandsRepository.FindByID(ctx, *product.BrandID)
	if err != nil {
		result.TitleError = fmt.Sprintf("error cargando la marca %d: %v", *product.BrandID, err)
		return
	}

	title, skipReason := ComposeBrandedTitle(product.BrandID, brand.Name, item.Title)
	if skipReason != "" {
		result.TitleSkippedReason = skipReason
		return
	}

	if err := s.itemsHandler.UpdateItemTitle(ctx, mercadoLibreInfra.UpdateItemTitleRequest{
		AccessToken: accessToken,
		ExternalID:  externalID,
		Title:       title,
	}); err != nil {
		result.TitleError = fmt.Sprintf("MercadoLibre no aceptó el nuevo título %q: %v", title, err)
		return
	}
	result.TitleUpdated = true
	result.Title = title
}

// ComposeBrandedTitle builds the title a listing gets once the brand is added
// at the end of its current title: "<title> Original <brand>" for Nissan (see
// withNissanOriginalSuffix) and "<title> <brand>" for any other brand. When it
// returns an empty title, skipReason says why nothing should be sent — the
// title already contains the brand as a whole word, there is no brand, or the
// result would exceed MercadoLibre's title limit.
func ComposeBrandedTitle(brandID *int64, brandName, currentTitle string) (title, skipReason string) {
	currentTitle = strings.TrimSpace(currentTitle)
	brandName = strings.TrimSpace(brandName)

	switch {
	case currentTitle == "":
		return "", "la publicación no tiene título"
	case brandName == "":
		return "", "el producto no tiene marca"
	case titleContainsWord(currentTitle, brandName):
		return "", fmt.Sprintf("el título ya contiene la marca %q", brandName)
	}

	base := currentTitle
	// A Nissan title that already says "Original" only needs the brand.
	if !titleContainsWord(currentTitle, nissanOriginalSuffix) {
		base = withNissanOriginalSuffix(brandID, currentTitle)
	}
	title = strings.TrimSpace(base + " " + brandName)

	if utf8.RuneCountInString(title) > mercadoLibreMaxTitleLength {
		return "", fmt.Sprintf("el título resultante excede los %d caracteres permitidos (%d): %q", mercadoLibreMaxTitleLength, utf8.RuneCountInString(title), title)
	}
	return title, ""
}

// titleContainsWord reports whether phrase appears in title as whole words,
// ignoring case, accents and punctuation ("Nissan" is found in "Filtro NISSAN,
// Sentra" but not in "Nissanito").
func titleContainsWord(title, phrase string) bool {
	normalizedPhrase := normalizeTitleWords(phrase)
	if normalizedPhrase == "" {
		return false
	}
	return strings.Contains(" "+normalizeTitleWords(title)+" ", " "+normalizedPhrase+" ")
}

var titleAccentFolder = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
)

// normalizeTitleWords lower-cases s, strips Spanish accents and replaces every
// run of non-alphanumeric characters with a single space.
func normalizeTitleWords(s string) string {
	s = titleAccentFolder.Replace(strings.ToLower(s))
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}), " ")
}
