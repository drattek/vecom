package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	meliListingImportApp "core-orchestrator/internal/application/meli_listing_import"
	syncApp "core-orchestrator/internal/application/sync"
	"github.com/go-chi/chi/v5"
)

// MeliListingImportHandler serves "Importar una publicación existente" in the
// product detail page's Sincronización tab: POST
// /api/products/{productId}/details/sync/{connectionId}/import-listing links a
// MercadoLibre listing that already exists to the product.
type MeliListingImportHandler struct {
	service *meliListingImportApp.Service
}

func NewMeliListingImportHandler(service *meliListingImportApp.Service) *MeliListingImportHandler {
	return &MeliListingImportHandler{service: service}
}

type importMeliListingRequest struct {
	ExternalID string `json:"externalId"`
}

func (h *MeliListingImportHandler) Import(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	connectionID, err := strconv.ParseInt(chi.URLParam(r, "connectionId"), 10, 64)
	if err != nil || connectionID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid connection id")
		return
	}

	var req importMeliListingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.Import(r.Context(), meliListingImportApp.ImportInput{
		ProductID:    productID,
		ConnectionID: connectionID,
		ExternalID:   req.ExternalID,
		ActorID:      user.ID,
	})
	if err != nil {
		var mismatch *meliListingImportApp.PartNumberMismatchError
		switch {
		case errors.Is(err, meliListingImportApp.ErrInvalidInput):
			writeJSONError(w, http.StatusBadRequest, "externalId is required")
		case errors.Is(err, meliListingImportApp.ErrInvalidExternalID):
			writeJSONError(w, http.StatusBadRequest, "El ID de la publicación no es válido. Usa el formato MLM123456789 o pega el enlace de la publicación.")
		case errors.Is(err, meliListingImportApp.ErrProductNotFound):
			writeJSONError(w, http.StatusNotFound, "product not found")
		case errors.Is(err, meliListingImportApp.ErrConnectionNotFound):
			writeJSONError(w, http.StatusNotFound, "channel connection not found")
		case errors.Is(err, meliListingImportApp.ErrListingNotFound):
			writeJSONError(w, http.StatusNotFound, "La publicación no existe en MercadoLibre.")
		case errors.Is(err, syncApp.ErrNotMercadoLibreConnection):
			writeJSONError(w, http.StatusBadRequest, "La conexión no es de MercadoLibre.")
		case errors.Is(err, meliListingImportApp.ErrProductAlreadyListed):
			writeJSONError(w, http.StatusConflict, "El producto ya tiene una publicación activa en esta conexión.")
		case errors.Is(err, meliListingImportApp.ErrListingLinkedToOtherProduct):
			writeJSONError(w, http.StatusConflict, "La publicación ya está vinculada a otro producto.")
		case errors.Is(err, meliListingImportApp.ErrListingNotOwned):
			writeJSONError(w, http.StatusUnprocessableEntity, "La publicación no pertenece a la cuenta de MercadoLibre de esta conexión.")
		case errors.Is(err, meliListingImportApp.ErrListingClosed):
			writeJSONError(w, http.StatusUnprocessableEntity, "La publicación está cerrada en MercadoLibre y no se puede importar.")
		case errors.As(err, &mismatch):
			writeJSONError(w, http.StatusUnprocessableEntity, partNumberMismatchMessage(mismatch))
		case errors.Is(err, syncApp.ErrMissingMercadoLibreSettings),
			errors.Is(err, syncApp.ErrInvalidMercadoLibreConnection),
			errors.Is(err, syncApp.ErrInvalidExpirationTimeFormat):
			writeJSONError(w, http.StatusBadRequest, "the MercadoLibre connection is not fully configured or authenticated")
		case errors.Is(err, meliListingImportApp.ErrListingFetch):
			log.Printf("meli listing import: %v", err)
			writeJSONError(w, http.StatusBadGateway, "No se pudo consultar la publicación en MercadoLibre. Intenta de nuevo.")
		default:
			log.Printf("meli listing import: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSONResponse(w, result)
}

func partNumberMismatchMessage(err *meliListingImportApp.PartNumberMismatchError) string {
	if len(err.ListingPartNumbers) == 0 {
		return "La publicación no tiene el atributo Número de parte (PART_NUMBER), así que no se puede validar contra el número de parte del producto (" + err.ProductPartNumber + ")."
	}
	return "El número de parte de la publicación (" + strings.Join(err.ListingPartNumbers, ", ") + ") no coincide con el del producto (" + err.ProductPartNumber + ")."
}
