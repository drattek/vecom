package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	mediaApp "core-orchestrator/internal/application/product_media"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// ProductImagesHandler
type upsertProductImageRequest struct {
	ProductID int64 `json:"productId"`
	FileID    int64 `json:"fileId"`
	IsFirst   bool  `json:"isFirst"`
}

type ProductImagesHandler struct {
	service *mediaApp.ProductImagesService
}

func NewProductImagesHandler(service *mediaApp.ProductImagesService) *ProductImagesHandler {
	return &ProductImagesHandler{service: service}
}

func (h *ProductImagesHandler) GetProductImages(w http.ResponseWriter, r *http.Request) {
	offsetStr := r.URL.Query().Get("offset")
	pageSizeStr := r.URL.Query().Get("pageSize")

	offset, pageSize := 0, 10
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsed
		}
	}
	if pageSizeStr != "" {
		if parsed, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = parsed
		}
	}

	result, err := h.service.GetPaginatedImages(r.Context(), offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *ProductImagesHandler) GetImageByID(w http.ResponseWriter, r *http.Request) {
	id := parseProductImageID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid image id")
		return
	}

	image, err := h.service.GetImageByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductImageNotFound) {
			writeJSONError(w, http.StatusNotFound, "image not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(image)
}

func (h *ProductImagesHandler) GetImagesByProduct(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	offsetStr := r.URL.Query().Get("offset")
	pageSizeStr := r.URL.Query().Get("pageSize")

	offset, pageSize := 0, 10
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsed
		}
	}
	if pageSizeStr != "" {
		if parsed, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = parsed
		}
	}

	result, err := h.service.GetImagesByProduct(r.Context(), productID, offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *ProductImagesHandler) CreateImage(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertProductImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateProductImageInput{
		ProductID: req.ProductID,
		FileID:    req.FileID,
		IsFirst:   req.IsFirst,
		CreatedBy: user.ID,
	}

	image, err := h.service.CreateImage(r.Context(), input)
	if err != nil {
		if errors.Is(err, mediaApp.ErrInvalidProductMedia) {
			writeJSONError(w, http.StatusBadRequest, "productId and fileId are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(image)
}

func (h *ProductImagesHandler) UpdateImage(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := parseProductImageID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid image id")
		return
	}

	var req struct {
		IsFirst bool `json:"isFirst"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateProductImageInput{
		IsFirst:   req.IsFirst,
		UpdatedBy: user.ID,
	}

	image, err := h.service.UpdateImage(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductImageNotFound) {
			writeJSONError(w, http.StatusNotFound, "image not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(image)
}

// SetCover marks an existing image as the product cover (is_first), clearing
// the previous one in the same transaction.
func (h *ProductImagesHandler) SetCover(w http.ResponseWriter, r *http.Request) {
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

	id := parseProductImageID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid image id")
		return
	}

	image, err := h.service.SetCover(r.Context(), productID, id, user.ID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductImageNotFound) {
			writeJSONError(w, http.StatusNotFound, "image not found")
			return
		}
		if errors.Is(err, mediaApp.ErrInvalidProductMedia) {
			writeJSONError(w, http.StatusBadRequest, "invalid product or image id")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(image)
}

func (h *ProductImagesHandler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := parseProductImageID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid image id")
		return
	}

	err := h.service.DeleteImage(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductImageNotFound) {
			writeJSONError(w, http.StatusNotFound, "image not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseProductImageID(r *http.Request) int64 {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	return id
}

// ProductVideosHandler
type upsertProductVideoRequest struct {
	ProductID int64 `json:"productId"`
	FileID    int64 `json:"fileId"`
}

type ProductVideosHandler struct {
	service *mediaApp.ProductVideosService
}

func NewProductVideosHandler(service *mediaApp.ProductVideosService) *ProductVideosHandler {
	return &ProductVideosHandler{service: service}
}

func (h *ProductVideosHandler) GetProductVideos(w http.ResponseWriter, r *http.Request) {
	offsetStr := r.URL.Query().Get("offset")
	pageSizeStr := r.URL.Query().Get("pageSize")

	offset, pageSize := 0, 10
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsed
		}
	}
	if pageSizeStr != "" {
		if parsed, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = parsed
		}
	}

	result, err := h.service.GetPaginatedVideos(r.Context(), offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *ProductVideosHandler) GetVideoByID(w http.ResponseWriter, r *http.Request) {
	id := parseProductVideoID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid video id")
		return
	}

	video, err := h.service.GetVideoByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductVideoNotFound) {
			writeJSONError(w, http.StatusNotFound, "video not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(video)
}

func (h *ProductVideosHandler) GetVideosByProduct(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	offsetStr := r.URL.Query().Get("offset")
	pageSizeStr := r.URL.Query().Get("pageSize")

	offset, pageSize := 0, 10
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsed
		}
	}
	if pageSizeStr != "" {
		if parsed, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = parsed
		}
	}

	result, err := h.service.GetVideosByProduct(r.Context(), productID, offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *ProductVideosHandler) CreateVideo(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertProductVideoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateProductVideoInput{
		ProductID: req.ProductID,
		FileID:    req.FileID,
		CreatedBy: user.ID,
	}

	video, err := h.service.CreateVideo(r.Context(), input)
	if err != nil {
		if errors.Is(err, mediaApp.ErrInvalidProductMedia) {
			writeJSONError(w, http.StatusBadRequest, "productId and fileId are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(video)
}

func (h *ProductVideosHandler) UpdateVideo(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := parseProductVideoID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid video id")
		return
	}

	input := mysqlInfra.UpdateProductVideoInput{
		UpdatedBy: user.ID,
	}

	video, err := h.service.UpdateVideo(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductVideoNotFound) {
			writeJSONError(w, http.StatusNotFound, "video not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(video)
}

func (h *ProductVideosHandler) DeleteVideo(w http.ResponseWriter, r *http.Request) {
	id := parseProductVideoID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid video id")
		return
	}

	err := h.service.DeleteVideo(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductVideoNotFound) {
			writeJSONError(w, http.StatusNotFound, "video not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseProductVideoID(r *http.Request) int64 {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	return id
}

// ProductPartNumbersHandler
type upsertProductPartNumberRequest struct {
	ProductID  int64  `json:"productId"`
	PartNumber string `json:"partNumber"`
	Type       string `json:"type"`
	BrandID    int64  `json:"brandId"`
}

type ProductPartNumbersHandler struct {
	service *mediaApp.ProductPartNumbersService
}

func NewProductPartNumbersHandler(service *mediaApp.ProductPartNumbersService) *ProductPartNumbersHandler {
	return &ProductPartNumbersHandler{service: service}
}

func (h *ProductPartNumbersHandler) GetPartNumbers(w http.ResponseWriter, r *http.Request) {
	offsetStr := r.URL.Query().Get("offset")
	pageSizeStr := r.URL.Query().Get("pageSize")

	offset, pageSize := 0, 10
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsed
		}
	}
	if pageSizeStr != "" {
		if parsed, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = parsed
		}
	}

	result, err := h.service.GetPaginatedPartNumbers(r.Context(), offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *ProductPartNumbersHandler) GetPartNumbersByProduct(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	offsetStr := r.URL.Query().Get("offset")
	pageSizeStr := r.URL.Query().Get("pageSize")

	offset, pageSize := 0, 10
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsed
		}
	}
	if pageSizeStr != "" {
		if parsed, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = parsed
		}
	}

	result, err := h.service.GetPartNumbersByProduct(r.Context(), productID, offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *ProductPartNumbersHandler) CreatePartNumber(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertProductPartNumberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateProductPartNumberInput{
		ProductID:  req.ProductID,
		PartNumber: req.PartNumber,
		Type:       req.Type,
		BrandID:    req.BrandID,
		CreatedBy:  user.ID,
	}

	partNumber, err := h.service.CreatePartNumber(r.Context(), input)
	if err != nil {
		if errors.Is(err, mediaApp.ErrInvalidProductMedia) {
			writeJSONError(w, http.StatusBadRequest, "productId and partNumber are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(partNumber)
}

func (h *ProductPartNumbersHandler) UpdatePartNumber(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	productIDStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	partNumber := chi.URLParam(r, "partNumber")
	if partNumber == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid part number")
		return
	}

	var req struct {
		Type    string `json:"type"`
		BrandID int64  `json:"brandId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateProductPartNumberInput{
		Type:      req.Type,
		BrandID:   req.BrandID,
		UpdatedBy: user.ID,
	}

	pn, err := h.service.UpdatePartNumber(r.Context(), productID, partNumber, input)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductPartNumberNotFound) {
			writeJSONError(w, http.StatusNotFound, "part number not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pn)
}

func (h *ProductPartNumbersHandler) DeletePartNumber(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	partNumber := chi.URLParam(r, "partNumber")
	if partNumber == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid part number")
		return
	}

	err = h.service.DeletePartNumber(r.Context(), productID, partNumber)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductPartNumberNotFound) {
			writeJSONError(w, http.StatusNotFound, "part number not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ProductDimensionsHandler
// volume no se recibe del cliente: el repositorio lo calcula como
// largo*ancho*alto (cm³) en cada escritura.
type upsertProductDimensionsRequest struct {
	Weight   string `json:"weight"`
	Length   string `json:"length"`
	Width    string `json:"width"`
	Height   string `json:"height"`
	Diameter string `json:"diameter"`
}

type ProductDimensionsHandler struct {
	service *mediaApp.ProductDimensionsService
}

func NewProductDimensionsHandler(service *mediaApp.ProductDimensionsService) *ProductDimensionsHandler {
	return &ProductDimensionsHandler{service: service}
}

func (h *ProductDimensionsHandler) GetDimensionsByProduct(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	dimensions, err := h.service.GetDimensionsByProduct(r.Context(), productID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductDimensionsNotFound) {
			writeJSONError(w, http.StatusNotFound, "dimensions not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dimensions)
}

func (h *ProductDimensionsHandler) CreateDimensions(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	productIDStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	var req upsertProductDimensionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateProductDimensionsInput{
		ProductID: productID,
		Weight:    req.Weight,
		Length:    req.Length,
		Width:     req.Width,
		Height:    req.Height,
		Diameter:  req.Diameter,
		CreatedBy: user.ID,
	}

	dimensions, err := h.service.CreateDimensions(r.Context(), input)
	if err != nil {
		if errors.Is(err, mediaApp.ErrInvalidProductMedia) {
			writeJSONError(w, http.StatusBadRequest, "invalid product id")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(dimensions)
}

func (h *ProductDimensionsHandler) UpdateDimensions(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	productIDStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	var req upsertProductDimensionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateProductDimensionsInput{
		Weight:    req.Weight,
		Length:    req.Length,
		Width:     req.Width,
		Height:    req.Height,
		Diameter:  req.Diameter,
		UpdatedBy: user.ID,
	}

	dimensions, err := h.service.UpdateDimensions(r.Context(), productID, input)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductDimensionsNotFound) {
			writeJSONError(w, http.StatusNotFound, "dimensions not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dimensions)
}

type bulkUpsertDimensionsItem struct {
	SKU      string   `json:"sku"`
	Weight   *float64 `json:"weight"`
	Length   *float64 `json:"length"`
	Width    *float64 `json:"width"`
	Height   *float64 `json:"height"`
	Diameter *float64 `json:"diameter"`
}

func (h *ProductDimensionsHandler) BulkUpsertDimensions(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req []bulkUpsertDimensionsItem
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req) == 0 {
		writeJSONError(w, http.StatusBadRequest, "request body must be a non-empty array")
		return
	}

	items := make([]mediaApp.BulkDimensionItem, len(req))
	for i, item := range req {
		items[i] = mediaApp.BulkDimensionItem{
			SKU:      item.SKU,
			Weight:   item.Weight,
			Length:   item.Length,
			Width:    item.Width,
			Height:   item.Height,
			Diameter: item.Diameter,
		}
	}

	results, err := h.service.BulkUpsertDimensions(r.Context(), items, user.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

func (h *ProductDimensionsHandler) DeleteDimensions(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	err = h.service.DeleteDimensions(r.Context(), productID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductDimensionsNotFound) {
			writeJSONError(w, http.StatusNotFound, "dimensions not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ProductSEOHandler
type upsertProductSEORequest struct {
	MetaTitle       *string `json:"metaTitle"`
	MetaDescription *string `json:"metaDescription"`
	Keywords        *string `json:"keywords"`
}

type ProductSEOHandler struct {
	service *mediaApp.ProductSEOService
}

func NewProductSEOHandler(service *mediaApp.ProductSEOService) *ProductSEOHandler {
	return &ProductSEOHandler{service: service}
}

func (h *ProductSEOHandler) GetSEOByProduct(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	seo, err := h.service.GetSEOByProduct(r.Context(), productID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductSEONotFound) {
			writeJSONError(w, http.StatusNotFound, "seo not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(seo)
}

func (h *ProductSEOHandler) CreateSEO(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	productIDStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	var req upsertProductSEORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateProductSEOInput{
		ProductID:       productID,
		MetaTitle:       req.MetaTitle,
		MetaDescription: req.MetaDescription,
		Keywords:        req.Keywords,
		CreatedBy:       user.ID,
	}

	seo, err := h.service.CreateSEO(r.Context(), input)
	if err != nil {
		if errors.Is(err, mediaApp.ErrInvalidProductMedia) {
			writeJSONError(w, http.StatusBadRequest, "invalid product id")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(seo)
}

func (h *ProductSEOHandler) UpdateSEO(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	productIDStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	var req upsertProductSEORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateProductSEOInput{
		MetaTitle:       req.MetaTitle,
		MetaDescription: req.MetaDescription,
		Keywords:        req.Keywords,
		UpdatedBy:       user.ID,
	}

	seo, err := h.service.UpdateSEO(r.Context(), productID, input)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductSEONotFound) {
			writeJSONError(w, http.StatusNotFound, "seo not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(seo)
}

func (h *ProductSEOHandler) DeleteSEO(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	err = h.service.DeleteSEO(r.Context(), productID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductSEONotFound) {
			writeJSONError(w, http.StatusNotFound, "seo not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
