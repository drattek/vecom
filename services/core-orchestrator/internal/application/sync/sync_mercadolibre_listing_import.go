package sync

import (
	"context"
	"errors"
	"fmt"
	"log"
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
// sync" — an import always pushes — and it does not skip a paused listing: a
// listing that already exists is very often paused precisely because it ran out
// of stock, and bringing it back in line with the local price/stock is the
// point of importing it (MercadoLibre reactivates an out-of-stock listing on
// its own once it has stock again). The live status is re-read afterwards and
// recorded as it really is. Closed and under_review listings are left alone,
// as in every other automatic flow. Only price
// and quantity are sent: the package attributes
// updateExistingListings adds would overwrite whatever the seller had on the
// listing with generic defaults when the product has no dimensions of its own.
// A title MercadoLibre refuses (items with sales can't change it) is reported
// in TitleError but never fails the import.
func (s *MercadoLibreProductSyncService) PushImportedListing(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, listing *mysqlInfra.ChannelProductMapDTO, item *mercadoLibreInfra.ItemDetail) *ImportedListingPushResult {
	result := &ImportedListingPushResult{}
	externalID := derefString(listing.ExternalID)

	// closed can't be recovered and under_review is never updated automatically
	// (the user fixes the product's data and presses "Resincronizar"); a paused
	// listing is updated like any other.
	switch listing.Status {
	case mercadoLibreMapStatusClosed:
		reason := "la publicación está cerrada en MercadoLibre; no se modifica"
		result.PriceStockSkippedReason = reason
		result.TitleSkippedReason = reason
		return result
	case mercadoLibreMapStatusUnderReview:
		reason := "la publicación está en revisión en MercadoLibre; corrige los datos del producto y usa Resincronizar"
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
	} else if _, err := s.channelProductMapRepository.Upsert(ctx, s.liveListingMapInput(ctx, product, connectionID, listing, accessToken)); err != nil {
		result.PriceStockError = fmt.Sprintf("precio y stock enviados, pero no se pudo registrar la sincronización: %v", err)
	} else {
		result.PriceStockUpdated = true
		result.Price = finalPrice
		result.Quantity = qtyAvailable
	}

	s.pushBrandedTitle(ctx, product, item, externalID, accessToken, result)

	return result
}

// liveListingStatus returns externalID's current status on MercadoLibre folded
// into the ecom_channel_product_map vocabulary, or fallback if it can't be read.
func (s *MercadoLibreProductSyncService) liveListingStatus(ctx context.Context, accessToken, externalID, fallback string) string {
	live, err := s.itemsHandler.GetItem(ctx, accessToken, externalID)
	if err != nil {
		log.Printf("mercadolibre: listing %s — could not re-read its status after a push, keeping %q: %v", externalID, fallback, err)
		return fallback
	}
	return foldMercadoLibreStatus(live.Status, live.SubStatus)
}

// liveListingMapInput builds the ecom_channel_product_map upsert for listing
// right after a price/stock push, with the status MercadoLibre reports now (the
// push may have reactivated an out-of-stock listing). A listing that is still
// paused is recorded with paused_reason 'user' — this system didn't pause it, so
// no later refresh reactivates it on its own. If the status can't be read, the
// one recorded at import time is kept.
func (s *MercadoLibreProductSyncService) liveListingMapInput(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, listing *mysqlInfra.ChannelProductMapDTO, accessToken string) mysqlInfra.UpsertChannelProductMapInput {
	status := s.liveListingStatus(ctx, accessToken, derefString(listing.ExternalID), listing.Status)

	pausedReason := ""
	if status == mercadoLibreMapStatusPaused {
		pausedReason = "user"
	}

	return mysqlInfra.UpsertChannelProductMapInput{
		ProductID:          product.ID,
		ConnectionID:       connectionID,
		VehicleFitmentID:   listing.VehicleFitmentID,
		ListingTitle:       derefString(listing.ListingTitle),
		ExternalID:         derefString(listing.ExternalID),
		ExternalCategoryID: derefString(listing.ExternalCategoryID),
		Status:             status,
		PausedReason:       pausedReason,
		ActorID:            systemMercadoLibreSyncActorID,
	}
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

	// An item with a family_name (User Product listing) has its title edited
	// through family_name and MercadoLibre rejects a direct title change; the
	// brand is appended to that family_name. Items without one take title.
	useFamilyName := strings.TrimSpace(item.FamilyName) != ""
	currentTitle := item.Title
	if useFamilyName {
		currentTitle = item.FamilyName
	}

	title, skipReason := ComposeBrandedTitle(product.BrandID, brand.Name, currentTitle)
	if skipReason != "" {
		result.TitleSkippedReason = skipReason
		return
	}

	if err := s.itemsHandler.UpdateItemTitle(ctx, mercadoLibreInfra.UpdateItemTitleRequest{
		AccessToken:   accessToken,
		ExternalID:    externalID,
		Title:         title,
		UseFamilyName: useFamilyName,
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

// ImportedListingNormalizeResult reports how NormalizeImportedListing brought an
// imported listing in line with how this system publishes: Premium listing type
// and outside any official store. As in ImportedListingPushResult, *Skipped
// fields explain a deliberate no-op and *Error fields are real failures.
type ImportedListingNormalizeResult struct {
	ListingTypeChanged       bool   `json:"listingTypeChanged"`
	ListingTypeFrom          string `json:"listingTypeFrom,omitempty"`
	ListingTypeSkippedReason string `json:"listingTypeSkippedReason,omitempty"`
	ListingTypeError         string `json:"listingTypeError,omitempty"`

	OfficialStoreRemoved       bool   `json:"officialStoreRemoved"`
	OfficialStoreID            int64  `json:"officialStoreId,omitempty"`
	OfficialStoreSkippedReason string `json:"officialStoreSkippedReason,omitempty"`
	OfficialStoreError         string `json:"officialStoreError,omitempty"`
}

// NormalizeImportedListing makes an already-existing MercadoLibre listing match
// how this system publishes (see createNewItem): listing type Premium
// (mercadoLibreItemListingTypeID) and no official store. It moves the listing to
// Premium when it has another type, and detaches it from its official store when
// it has one. Closed and under_review listings are left alone, like in every
// other automatic flow. MercadoLibre may refuse either change (a listing type can
// be changed only once; removing a store isn't a documented operation), which is
// reported in the matching *Error field and never fails the import.
func (s *MercadoLibreProductSyncService) NormalizeImportedListing(ctx context.Context, connectionID int64, listing *mysqlInfra.ChannelProductMapDTO, item *mercadoLibreInfra.ItemDetail) *ImportedListingNormalizeResult {
	result := &ImportedListingNormalizeResult{}
	externalID := derefString(listing.ExternalID)

	switch listing.Status {
	case mercadoLibreMapStatusClosed:
		reason := "la publicación está cerrada en MercadoLibre; no se modifica"
		result.ListingTypeSkippedReason, result.OfficialStoreSkippedReason = reason, reason
		return result
	case mercadoLibreMapStatusUnderReview:
		reason := "la publicación está en revisión en MercadoLibre; corrige los datos del producto y usa Resincronizar"
		result.ListingTypeSkippedReason, result.OfficialStoreSkippedReason = reason, reason
		return result
	}

	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		message := fmt.Sprintf("no se pudo obtener el token de MercadoLibre: %v", err)
		result.ListingTypeError, result.OfficialStoreError = message, message
		return result
	}

	switch {
	case strings.EqualFold(item.ListingTypeID, mercadoLibreItemListingTypeID):
		result.ListingTypeSkippedReason = "la publicación ya es Premium"
	default:
		result.ListingTypeFrom = item.ListingTypeID
		if err := s.itemsHandler.UpdateItemListingType(ctx, accessToken, externalID, mercadoLibreItemListingTypeID); err != nil {
			result.ListingTypeError = fmt.Sprintf("MercadoLibre no permitió cambiar el tipo de publicación de %q a Premium: %v", item.ListingTypeID, err)
		} else {
			result.ListingTypeChanged = true
		}
	}

	if item.OfficialStoreID == nil || *item.OfficialStoreID == 0 {
		result.OfficialStoreSkippedReason = "la publicación no está en una tienda oficial"
		return result
	}
	result.OfficialStoreID = *item.OfficialStoreID
	if err := s.itemsHandler.RemoveItemOfficialStore(ctx, accessToken, externalID); err != nil {
		result.OfficialStoreError = fmt.Sprintf("MercadoLibre no permitió sacar la publicación de la tienda oficial %d: %v", *item.OfficialStoreID, err)
		return result
	}
	result.OfficialStoreRemoved = true

	return result
}
