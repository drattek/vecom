package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	importApp "core-orchestrator/internal/application/product_image_import"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type importProductImagesProductRequest struct {
	SKU  string   `json:"sku"`
	URLs []string `json:"urls"`
}

type importProductImagesRequest struct {
	StorageDiskID int64                               `json:"storageDiskId"`
	Products      []importProductImagesProductRequest `json:"products"`
}

type ProductImageImportHandler struct {
	service *importApp.Service
}

func NewProductImageImportHandler(service *importApp.Service) *ProductImageImportHandler {
	return &ProductImageImportHandler{service: service}
}

func (h *ProductImageImportHandler) Import(w http.ResponseWriter, r *http.Request) {
	_, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req importProductImagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	products := make([]importApp.ProductImageURLsInput, 0, len(req.Products))
	for _, p := range req.Products {
		products = append(products, importApp.ProductImageURLsInput{
			SKU:  p.SKU,
			URLs: p.URLs,
		})
	}

	result, err := h.service.Import(r.Context(), importApp.ImportProductImagesInput{
		StorageDiskID: req.StorageDiskID,
		Products:      products,
	})
	if err != nil {
		if errors.Is(err, importApp.ErrInvalidImportPayload) {
			writeJSONError(w, http.StatusBadRequest, "storageDiskId and products are required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrStorageDiskNotFound) {
			writeJSONError(w, http.StatusNotFound, "storage disk not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

type importForProductImageRequest struct {
	URL     string `json:"url"`
	IsFirst bool   `json:"isFirst"`
}

type importForProductRequest struct {
	StorageDiskID int64                          `json:"storageDiskId"`
	Images        []importForProductImageRequest `json:"images"`
}

// ImportForProduct attaches image URLs to a single product (product detail →
// Multimedia → "Agregar imágenes"). Same URL validation as the SKU bulk import,
// scoped to one product.
func (h *ProductImageImportHandler) ImportForProduct(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	productID, err := strconv.ParseInt(chi.URLParam(r, "productId"), 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	var req importForProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	images := make([]importApp.ProductImageItemInput, 0, len(req.Images))
	for _, img := range req.Images {
		images = append(images, importApp.ProductImageItemInput{URL: img.URL, IsFirst: img.IsFirst})
	}

	result, err := h.service.ImportForProduct(r.Context(), importApp.ImportForProductInput{
		ProductID:     productID,
		StorageDiskID: req.StorageDiskID,
		Images:        images,
		ActorID:       user.ID,
	})
	if err != nil {
		switch {
		case errors.Is(err, importApp.ErrInvalidImportPayload):
			writeJSONError(w, http.StatusBadRequest, "storageDiskId and images are required")
		case errors.Is(err, mysqlInfra.ErrStorageDiskNotFound):
			writeJSONError(w, http.StatusNotFound, "storage disk not found")
		case errors.Is(err, importApp.ErrProductNotFoundForImport):
			writeJSONError(w, http.StatusNotFound, "product not found")
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

type replaceProductImageRequest struct {
	StorageDiskID int64  `json:"storageDiskId"`
	URL           string `json:"url"`
}

// ReplaceProductImage swaps the file behind one product image for a new URL.
func (h *ProductImageImportHandler) ReplaceProductImage(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	productID, err := strconv.ParseInt(chi.URLParam(r, "productId"), 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	imageID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || imageID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid image id")
		return
	}

	var req replaceProductImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.ReplaceProductImage(r.Context(), importApp.ReplaceProductImageInput{
		ProductID:     productID,
		ImageID:       imageID,
		StorageDiskID: req.StorageDiskID,
		URL:           req.URL,
		ActorID:       user.ID,
	})
	if err != nil {
		switch {
		case errors.Is(err, importApp.ErrInvalidImportPayload):
			writeJSONError(w, http.StatusBadRequest, "storageDiskId and url are required")
		case errors.Is(err, mysqlInfra.ErrStorageDiskNotFound):
			writeJSONError(w, http.StatusNotFound, "storage disk not found")
		case errors.Is(err, importApp.ErrProductNotFoundForImport):
			writeJSONError(w, http.StatusNotFound, "product not found")
		case errors.Is(err, importApp.ErrProductImageNotFound):
			writeJSONError(w, http.StatusNotFound, "image not found")
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// --- Archivos adjuntos (ecom_product_media) --------------------------------

type importAttachmentRequest struct {
	URL  string `json:"url"`
	Type string `json:"type"`
}

type importAttachmentsForProductRequest struct {
	StorageDiskID int64                     `json:"storageDiskId"`
	Attachments   []importAttachmentRequest `json:"attachments"`
}

// ImportAttachmentsForProduct attaches file URLs (manuals, datasheets,
// certificates, images) to a product, each with its own type.
func (h *ProductImageImportHandler) ImportAttachmentsForProduct(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	productID, err := strconv.ParseInt(chi.URLParam(r, "productId"), 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	var req importAttachmentsForProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	attachments := make([]importApp.ProductAttachmentItemInput, 0, len(req.Attachments))
	for _, item := range req.Attachments {
		attachments = append(attachments, importApp.ProductAttachmentItemInput{URL: item.URL, Type: item.Type})
	}

	result, err := h.service.ImportAttachmentsForProduct(r.Context(), importApp.ImportAttachmentsForProductInput{
		ProductID:     productID,
		StorageDiskID: req.StorageDiskID,
		Attachments:   attachments,
		ActorID:       user.ID,
	})
	if err != nil {
		switch {
		case errors.Is(err, importApp.ErrInvalidImportPayload):
			writeJSONError(w, http.StatusBadRequest, "storageDiskId and attachments are required")
		case errors.Is(err, mysqlInfra.ErrStorageDiskNotFound):
			writeJSONError(w, http.StatusNotFound, "storage disk not found")
		case errors.Is(err, importApp.ErrProductNotFoundForImport):
			writeJSONError(w, http.StatusNotFound, "product not found")
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

type replaceProductAttachmentRequest struct {
	StorageDiskID int64  `json:"storageDiskId"`
	URL           string `json:"url"`
	Type          string `json:"type"`
}

// ReplaceProductAttachment swaps the file behind one attachment for a new URL.
func (h *ProductImageImportHandler) ReplaceProductAttachment(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	productID, err := strconv.ParseInt(chi.URLParam(r, "productId"), 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	fileID, err := strconv.ParseInt(chi.URLParam(r, "fileId"), 10, 64)
	if err != nil || fileID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	var req replaceProductAttachmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.ReplaceProductAttachment(r.Context(), importApp.ReplaceProductAttachmentInput{
		ProductID:     productID,
		FileID:        fileID,
		StorageDiskID: req.StorageDiskID,
		URL:           req.URL,
		Type:          req.Type,
		ActorID:       user.ID,
	})
	if err != nil {
		switch {
		case errors.Is(err, importApp.ErrInvalidImportPayload):
			writeJSONError(w, http.StatusBadRequest, "storageDiskId and url are required")
		case errors.Is(err, mysqlInfra.ErrStorageDiskNotFound):
			writeJSONError(w, http.StatusNotFound, "storage disk not found")
		case errors.Is(err, importApp.ErrProductNotFoundForImport):
			writeJSONError(w, http.StatusNotFound, "product not found")
		case errors.Is(err, importApp.ErrProductMediaNotFound):
			writeJSONError(w, http.StatusNotFound, "attachment not found")
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// DeleteProductAttachment soft-deletes one ecom_product_media row.
func (h *ProductImageImportHandler) DeleteProductAttachment(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	productID, err := strconv.ParseInt(chi.URLParam(r, "productId"), 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	fileID, err := strconv.ParseInt(chi.URLParam(r, "fileId"), 10, 64)
	if err != nil || fileID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	if err := h.service.DeleteProductAttachment(r.Context(), productID, fileID, user.ID); err != nil {
		if errors.Is(err, importApp.ErrProductMediaNotFound) {
			writeJSONError(w, http.StatusNotFound, "attachment not found")
			return
		}
		if errors.Is(err, importApp.ErrInvalidImportPayload) {
			writeJSONError(w, http.StatusBadRequest, "invalid product or file id")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Videos (ecom_product_videos) -----------------------------------------

type importVideosForProductRequest struct {
	StorageDiskID int64    `json:"storageDiskId"`
	URLs          []string `json:"urls"`
}

// ImportVideosForProduct attaches video URLs to a product. Only the disk and the
// URLs are needed — no order, no cover, no type.
func (h *ProductImageImportHandler) ImportVideosForProduct(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	productID, err := strconv.ParseInt(chi.URLParam(r, "productId"), 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	var req importVideosForProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.ImportVideosForProduct(r.Context(), importApp.ImportVideosForProductInput{
		ProductID:     productID,
		StorageDiskID: req.StorageDiskID,
		URLs:          req.URLs,
		ActorID:       user.ID,
	})
	if err != nil {
		switch {
		case errors.Is(err, importApp.ErrInvalidImportPayload):
			writeJSONError(w, http.StatusBadRequest, "storageDiskId and urls are required")
		case errors.Is(err, mysqlInfra.ErrStorageDiskNotFound):
			writeJSONError(w, http.StatusNotFound, "storage disk not found")
		case errors.Is(err, importApp.ErrProductNotFoundForImport):
			writeJSONError(w, http.StatusNotFound, "product not found")
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

type replaceProductVideoRequest struct {
	StorageDiskID int64  `json:"storageDiskId"`
	URL           string `json:"url"`
}

// ReplaceProductVideo swaps the file behind one video for a new URL.
func (h *ProductImageImportHandler) ReplaceProductVideo(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	productID, err := strconv.ParseInt(chi.URLParam(r, "productId"), 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	videoID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || videoID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid video id")
		return
	}

	var req replaceProductVideoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.ReplaceProductVideo(r.Context(), importApp.ReplaceProductVideoInput{
		ProductID:     productID,
		VideoID:       videoID,
		StorageDiskID: req.StorageDiskID,
		URL:           req.URL,
		ActorID:       user.ID,
	})
	if err != nil {
		switch {
		case errors.Is(err, importApp.ErrInvalidImportPayload):
			writeJSONError(w, http.StatusBadRequest, "storageDiskId and url are required")
		case errors.Is(err, mysqlInfra.ErrStorageDiskNotFound):
			writeJSONError(w, http.StatusNotFound, "storage disk not found")
		case errors.Is(err, importApp.ErrProductNotFoundForImport):
			writeJSONError(w, http.StatusNotFound, "product not found")
		case errors.Is(err, importApp.ErrProductVideoNotFound):
			writeJSONError(w, http.StatusNotFound, "video not found")
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
