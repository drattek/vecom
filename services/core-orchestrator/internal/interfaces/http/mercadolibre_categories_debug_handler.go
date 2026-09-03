package http

import (
	"net/http"
	"strconv"
	"strings"

	syncApp "core-orchestrator/internal/application/sync"
)

// mercadoLibreCategoriesDebugDefaultSiteID is the only MercadoLibre site this
// integration operates against (mirrors mercadoLibreCompatibilitySiteID in
// sync_mercadolibre_compatibilities.go) — used when a caller doesn't pass
// siteId explicitly.
const mercadoLibreCategoriesDebugDefaultSiteID = "MLM"

// MercadoLibreCategoriesDebugHandler is a TEMPORARY diagnostic endpoint for
// browsing MercadoLibre's category tree: without a categoryId it returns the
// site's root categories (GET /sites/{siteId}/categories); with one it
// returns that category's full MercadoLibre JSON as-is — including
// children_categories — so the tree can be walked one level at a time from
// the response of the previous call. Both calls need a valid access token
// (see CategoriesHandler.GetSiteCategoriesRaw — MercadoLibre 403s these
// without one despite documenting them as public), so connectionId is
// required to know which connection's token to use. Remove once the category
// lookup it's for is done.
type MercadoLibreCategoriesDebugHandler struct {
	categoryPredictorService *syncApp.MercadoLibreCategoryPredictorService
}

func NewMercadoLibreCategoriesDebugHandler(categoryPredictorService *syncApp.MercadoLibreCategoryPredictorService) *MercadoLibreCategoriesDebugHandler {
	return &MercadoLibreCategoriesDebugHandler{categoryPredictorService: categoryPredictorService}
}

// GetCategories handles
// GET .../debug/categories?connectionId=1&categoryId=MLM1743 (a specific
// category's own children) or GET .../debug/categories?connectionId=1 (root
// categories — categoryId omitted; siteId itself optional, defaulting to
// mercadoLibreCategoriesDebugDefaultSiteID). siteId is ignored once
// categoryId is present, since a category id is already site-specific.
func (h *MercadoLibreCategoriesDebugHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	connectionID, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("connectionId")), 10, 64)
	if err != nil || connectionID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "connectionId query param is required")
		return
	}

	categoryID := strings.TrimSpace(r.URL.Query().Get("categoryId"))

	var payload []byte
	if categoryID == "" {
		siteID := strings.TrimSpace(r.URL.Query().Get("siteId"))
		if siteID == "" {
			siteID = mercadoLibreCategoriesDebugDefaultSiteID
		}
		payload, err = h.categoryPredictorService.BrowseRootCategories(r.Context(), connectionID, siteID)
	} else {
		payload, err = h.categoryPredictorService.BrowseCategory(r.Context(), connectionID, categoryID)
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(payload)
}
