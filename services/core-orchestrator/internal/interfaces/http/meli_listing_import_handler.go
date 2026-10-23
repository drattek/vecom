package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	meliListingImportApp "core-orchestrator/internal/application/meli_listing_import"
	syncApp "core-orchestrator/internal/application/sync"
	"github.com/go-chi/chi/v5"
)

// MeliListingImportHandler serves "Importar una publicación existente" in the
// product detail page's Sincronización tab.
//
// POST /api/products/{productId}/details/sync/{connectionId}/import-listing
// validates the request and starts the import in the background, answering 202
// with a job; GET /api/products/{productId}/import-listing-jobs/{jobId} returns
// that job's progress. The import is not done inside the request because it can
// outlast the response time any proxy in front of the API allows (Cloudflare
// answers 524 after 100 s).
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

	job, err := h.service.Start(r.Context(), meliListingImportApp.ImportInput{
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
		case errors.Is(err, meliListingImportApp.ErrJobStore):
			log.Printf("meli listing import: %v", err)
			writeJSONError(w, http.StatusServiceUnavailable, "No se pudo iniciar la importación: el almacén de progreso (Redis) no está disponible.")
		default:
			log.Printf("meli listing import: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(job)
}

// GetImportJob returns the progress of an import started with Import.
func (h *MeliListingImportHandler) GetImportJob(w http.ResponseWriter, r *http.Request) {
	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	job, err := h.service.GetJob(r.Context(), productID, chi.URLParam(r, "jobId"))
	if err != nil {
		if errors.Is(err, meliListingImportApp.ErrJobNotFound) {
			writeJSONError(w, http.StatusNotFound, "No se encontró el avance de la importación (pudo haber caducado). Recarga la sección para ver el resultado.")
			return
		}
		log.Printf("meli listing import: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSONResponse(w, job)
}

func partNumberMismatchMessage(err *meliListingImportApp.PartNumberMismatchError) string {
	return err.Message()
}
