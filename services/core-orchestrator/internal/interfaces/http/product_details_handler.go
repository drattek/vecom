package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	productDetailsApp "core-orchestrator/internal/application/product_details"
)

// ProductDetailsHandler serves the read-only sections of the product detail
// page, one endpoint per section so the UI only fetches the tab being viewed.
type ProductDetailsHandler struct {
	service *productDetailsApp.Service
}

func NewProductDetailsHandler(service *productDetailsApp.Service) *ProductDetailsHandler {
	return &ProductDetailsHandler{service: service}
}

func (h *ProductDetailsHandler) GetGeneral(w http.ResponseWriter, r *http.Request) {
	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	general, err := h.service.GetGeneral(r.Context(), productID)
	if err != nil {
		writeProductDetailsError(w, err)
		return
	}

	writeJSONResponse(w, general)
}

// PatchGeneral aplica una edición inline de la sección General. El body solo
// lleva las claves que se están editando (name, description, shortDescription);
// las ausentes no se tocan.
func (h *ProductDetailsHandler) PatchGeneral(w http.ResponseWriter, r *http.Request) {
	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// brandId / categoryId: omitido = no tocar; 0 (o negativo) = quitar la
	// relación; > 0 = asignar ese id.
	var req struct {
		Name             *string `json:"name"`
		Description      *string `json:"description"`
		ShortDescription *string `json:"shortDescription"`
		BrandID          *int64  `json:"brandId"`
		CategoryID       *int64  `json:"categoryId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	general, err := h.service.UpdateGeneral(r.Context(), productID, user.ID, productDetailsApp.UpdateGeneralInput{
		Name:             req.Name,
		Description:      req.Description,
		ShortDescription: req.ShortDescription,
		BrandID:          req.BrandID,
		CategoryID:       req.CategoryID,
	})
	if err != nil {
		writeProductDetailsError(w, err)
		return
	}

	writeJSONResponse(w, general)
}

func (h *ProductDetailsHandler) GetMedia(w http.ResponseWriter, r *http.Request) {
	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	media, err := h.service.GetMedia(r.Context(), productID)
	if err != nil {
		writeProductDetailsError(w, err)
		return
	}

	writeJSONResponse(w, media)
}

func (h *ProductDetailsHandler) GetPricing(w http.ResponseWriter, r *http.Request) {
	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	pricing, err := h.service.GetPricing(r.Context(), productID)
	if err != nil {
		writeProductDetailsError(w, err)
		return
	}

	writeJSONResponse(w, pricing)
}

func (h *ProductDetailsHandler) GetInventory(w http.ResponseWriter, r *http.Request) {
	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	inventory, err := h.service.GetInventory(r.Context(), productID)
	if err != nil {
		writeProductDetailsError(w, err)
		return
	}

	writeJSONResponse(w, inventory)
}

func (h *ProductDetailsHandler) GetStockMovements(w http.ResponseWriter, r *http.Request) {
	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	offset, pageSize := parseDetailPagination(r)

	page, err := h.service.GetStockMovements(r.Context(), productID, offset, pageSize)
	if err != nil {
		writeProductDetailsError(w, err)
		return
	}

	writeJSONResponse(w, page)
}

func (h *ProductDetailsHandler) GetPriceHistory(w http.ResponseWriter, r *http.Request) {
	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	offset, pageSize := parseDetailPagination(r)

	page, err := h.service.GetPriceHistory(r.Context(), productID, offset, pageSize)
	if err != nil {
		writeProductDetailsError(w, err)
		return
	}

	writeJSONResponse(w, page)
}

func (h *ProductDetailsHandler) GetPartNumbers(w http.ResponseWriter, r *http.Request) {
	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	partNumbers, err := h.service.GetPartNumbers(r.Context(), productID)
	if err != nil {
		writeProductDetailsError(w, err)
		return
	}

	writeJSONResponse(w, partNumbers)
}

func (h *ProductDetailsHandler) GetAttributes(w http.ResponseWriter, r *http.Request) {
	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	attributes, err := h.service.GetAttributes(r.Context(), productID)
	if err != nil {
		writeProductDetailsError(w, err)
		return
	}

	writeJSONResponse(w, attributes)
}

func (h *ProductDetailsHandler) GetCompatibilities(w http.ResponseWriter, r *http.Request) {
	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	compatibilities, err := h.service.GetCompatibilities(r.Context(), productID)
	if err != nil {
		writeProductDetailsError(w, err)
		return
	}

	writeJSONResponse(w, compatibilities)
}

// PatchDimensions hace un upsert de la sección Dimensiones (crea la fila si no
// existe) y devuelve la sección Atributos completa refrescada.
func (h *ProductDetailsHandler) PatchDimensions(w http.ResponseWriter, r *http.Request) {
	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// volume no se recibe: lo calcula el core como largo*ancho*alto.
	var req struct {
		Weight   string `json:"weight"`
		Length   string `json:"length"`
		Width    string `json:"width"`
		Height   string `json:"height"`
		Diameter string `json:"diameter"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	attributes, err := h.service.UpdateDimensions(r.Context(), productID, user.ID, productDetailsApp.UpdateDimensionsInput{
		Weight:   req.Weight,
		Length:   req.Length,
		Width:    req.Width,
		Height:   req.Height,
		Diameter: req.Diameter,
	})
	if err != nil {
		writeProductDetailsError(w, err)
		return
	}

	writeJSONResponse(w, attributes)
}

// PatchSEO hace un upsert de la sección SEO (crea la fila si no existe) y
// devuelve la sección Atributos completa refrescada.
func (h *ProductDetailsHandler) PatchSEO(w http.ResponseWriter, r *http.Request) {
	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		MetaTitle       string `json:"metaTitle"`
		MetaDescription string `json:"metaDescription"`
		Keywords        string `json:"keywords"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	attributes, err := h.service.UpdateSEO(r.Context(), productID, user.ID, productDetailsApp.UpdateSEOInput{
		MetaTitle:       req.MetaTitle,
		MetaDescription: req.MetaDescription,
		Keywords:        req.Keywords,
	})
	if err != nil {
		writeProductDetailsError(w, err)
		return
	}

	writeJSONResponse(w, attributes)
}

func parseDetailProductID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	productID, err := strconv.ParseInt(chi.URLParam(r, "productId"), 10, 64)
	if err != nil || productID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return 0, false
	}

	return productID, true
}

// parseDetailPagination lee offset/pageSize del query string; los valores fuera de
// rango o ausentes los normaliza la capa de servicio (normalizeDetailPage).
func parseDetailPagination(r *http.Request) (offset, pageSize int) {
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil {
		offset = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("pageSize")); err == nil {
		pageSize = v
	}
	return offset, pageSize
}

func writeProductDetailsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, productDetailsApp.ErrInvalidProductID):
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
	case errors.Is(err, productDetailsApp.ErrInvalidGeneralPatch):
		writeJSONError(w, http.StatusBadRequest, "invalid general patch")
	case errors.Is(err, productDetailsApp.ErrInvalidDimensionsPatch):
		writeJSONError(w, http.StatusBadRequest, "invalid dimensions patch")
	case errors.Is(err, productDetailsApp.ErrInvalidSEOPatch):
		writeJSONError(w, http.StatusBadRequest, "invalid seo patch")
	case errors.Is(err, productDetailsApp.ErrProductNotFound):
		writeJSONError(w, http.StatusNotFound, "product not found")
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeJSONResponse(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(payload)
}
