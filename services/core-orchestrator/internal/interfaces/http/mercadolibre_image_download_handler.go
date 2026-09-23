package http

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/HugoSmits86/nativewebp"

	syncApp "core-orchestrator/internal/application/sync"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// MercadoLibreImageDownloadHandler is a TEMPORARY, throwaway endpoint: given
// a connectionId and a batch of MercadoLibre item ids, it downloads every
// picture MercadoLibre reports for each item, re-encodes it as lossless WebP
// (github.com/HugoSmits86/nativewebp — pure Go, no cgo/libwebp, so it still
// builds under this service's CGO_ENABLED=0 Dockerfile) and saves it to disk
// under <baseDir>/<sku>/<sku>_API_<n>.webp (n starting at 1, in the order
// MercadoLibre lists the pictures).
//
// It never touches the database — sku is only used to name folders/files,
// resolved from the local ecom_channel_product_map row for
// (connectionId, itemId) when one exists (the canonical sku already on
// record), falling back to what MercadoLibre itself reports on the item
// (SELLER_SKU attribute, then seller_custom_field) and finally the item id,
// so a listing with no sku recorded anywhere still gets a usable folder name.
//
// Remove this handler and its route once no longer needed.
type MercadoLibreImageDownloadHandler struct {
	tokenService                *syncApp.MercadoLibreTokenService
	itemsHandler                *mercadoLibreInfra.ItemsHandler
	channelProductMapRepository *mysqlInfra.ChannelProductMapRepository
	productRepository           *mysqlInfra.ProductRepository
	baseDir                     string
	httpClient                  *http.Client
}

func NewMercadoLibreImageDownloadHandler(
	tokenService *syncApp.MercadoLibreTokenService,
	rateLimiter *mercadoLibreInfra.RateLimiter,
	channelProductMapRepository *mysqlInfra.ChannelProductMapRepository,
	productRepository *mysqlInfra.ProductRepository,
	baseDir string,
) *MercadoLibreImageDownloadHandler {
	client := mercadoLibreInfra.NewClient(nil, "", rateLimiter)

	return &MercadoLibreImageDownloadHandler{
		tokenService:                tokenService,
		itemsHandler:                mercadoLibreInfra.NewItemsHandler(client),
		channelProductMapRepository: channelProductMapRepository,
		productRepository:           productRepository,
		baseDir:                     baseDir,
		httpClient:                  &http.Client{Timeout: 30 * time.Second},
	}
}

type downloadMercadoLibreImagesRequest struct {
	ConnectionID int64    `json:"connectionId"`
	ItemIDs      []string `json:"itemIds"`
}

type downloadMercadoLibreImagesItemResult struct {
	ItemID           string   `json:"itemId"`
	SKU              string   `json:"sku,omitempty"`
	ImagesDownloaded int      `json:"imagesDownloaded"`
	Errors           []string `json:"errors,omitempty"`
}

type downloadMercadoLibreImagesResult struct {
	Results []downloadMercadoLibreImagesItemResult `json:"results"`
}

// Run downloads and locally stores (as WebP) every picture of each
// req.ItemIDs entry. One item failing (bad id, no pictures, a broken
// picture URL) never aborts the rest of the batch.
func (h *MercadoLibreImageDownloadHandler) Run(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req downloadMercadoLibreImagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ConnectionID <= 0 || len(req.ItemIDs) == 0 {
		writeJSONError(w, http.StatusBadRequest, "connectionId and itemIds are required")
		return
	}

	accessToken, err := h.tokenService.EnsureValidAccessToken(ctx, req.ConnectionID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("error getting mercadolibre access token: %v", err))
		return
	}

	results := make([]downloadMercadoLibreImagesItemResult, 0, len(req.ItemIDs))
	for _, rawItemID := range req.ItemIDs {
		itemID := strings.TrimSpace(rawItemID)
		if itemID == "" {
			continue
		}
		results = append(results, h.downloadItemImages(ctx, req.ConnectionID, accessToken, itemID))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(downloadMercadoLibreImagesResult{Results: results})
}

func (h *MercadoLibreImageDownloadHandler) downloadItemImages(ctx context.Context, connectionID int64, accessToken, itemID string) downloadMercadoLibreImagesItemResult {
	result := downloadMercadoLibreImagesItemResult{ItemID: itemID}

	item, err := h.itemsHandler.GetItem(ctx, accessToken, itemID)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("error fetching item: %v", err))
		return result
	}

	sku := h.resolveSKU(ctx, connectionID, itemID, item)
	result.SKU = sku

	if len(item.Pictures) == 0 {
		result.Errors = append(result.Errors, "item has no pictures")
		return result
	}

	dir := filepath.Join(h.baseDir, sku)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("error creating folder %q: %v", dir, err))
		return result
	}

	for i, picture := range item.Pictures {
		index := i + 1
		if err := h.downloadAndSaveAsWebP(ctx, picture.Source, dir, sku, index); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("image %d (%s): %v", index, picture.Source, err))
			continue
		}
		result.ImagesDownloaded++
	}

	return result
}

// resolveSKU prefers the local ecom_channel_product_map row for
// (connectionID, itemID) — see the handler doc comment for why — and falls
// back to the item's own MercadoLibre data.
func (h *MercadoLibreImageDownloadHandler) resolveSKU(ctx context.Context, connectionID int64, itemID string, item *mercadoLibreInfra.ItemDetail) string {
	if mapping, err := h.channelProductMapRepository.FindByConnectionAndExternalID(ctx, connectionID, itemID); err == nil {
		if product, perr := h.productRepository.FindByID(ctx, mapping.ProductID); perr == nil {
			if sku := strings.TrimSpace(product.SKU); sku != "" {
				return sku
			}
		}
	}

	for _, attr := range item.Attributes {
		if attr.ID == "SELLER_SKU" {
			if sku := strings.TrimSpace(attr.ValueName); sku != "" {
				return sku
			}
		}
	}
	if sku := strings.TrimSpace(item.SellerCustomField); sku != "" {
		return sku
	}
	return itemID
}

// downloadAndSaveAsWebP downloads pictureURL, decodes it (JPEG/PNG — the only
// formats MercadoLibre serves item pictures in) and re-encodes it as
// lossless WebP at <dir>/<sku>_API_<index>.webp.
func (h *MercadoLibreImageDownloadHandler) downloadAndSaveAsWebP(ctx context.Context, pictureURL, dir, sku string, index int) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pictureURL, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	response, err := h.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("error downloading image: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", response.StatusCode)
	}

	img, _, err := image.Decode(response.Body)
	if err != nil {
		return fmt.Errorf("error decoding image: %w", err)
	}

	filename := fmt.Sprintf("%s_API_%d.webp", sku, index)
	file, err := os.Create(filepath.Join(dir, filename))
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	if err := nativewebp.Encode(file, img, nil); err != nil {
		return fmt.Errorf("error encoding webp: %w", err)
	}

	return nil
}
