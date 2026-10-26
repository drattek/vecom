package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	meliBulkImportApp "core-orchestrator/internal/application/meli_bulk_import"
	syncApp "core-orchestrator/internal/application/sync"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"github.com/go-chi/chi/v5"
)

// maxBulkImportFileBytes bounds the uploaded Excel.
const maxBulkImportFileBytes = 10 << 20

var bulkImportStatuses = map[string]bool{
	mysqlInfra.MeliImportPending:   true,
	mysqlInfra.MeliImportRunning:   true,
	mysqlInfra.MeliImportDone:      true,
	mysqlInfra.MeliImportFailed:    true,
	mysqlInfra.MeliImportMissing:   true,
	mysqlInfra.MeliImportDuplicate: true,
}

// MeliBulkImportHandler serves "Productos → Importar": the Excel upload (sku,
// meli_id) that queues MercadoLibre listings for the background worker, and the
// views over that queue and its history.
//
//	POST /api/meli-bulk-import/batches      multipart: file + connectionId
//	GET  /api/meli-bulk-import/batches      ?connectionId=  → uploaded files, newest first
//	GET  /api/meli-bulk-import/batches/{batchId}/export  → .xlsx report of the batch
//	GET  /api/meli-bulk-import/items       ?status=a,b&batchId=&connectionId=&search=&history=true&offset=&pageSize=
//	GET  /api/meli-bulk-import/summary      ?connectionId=&batchId=  → count per status
//	POST /api/meli-bulk-import/items/{itemId}/retry
//	POST /api/meli-bulk-import/retry        {"status":"failed|missing","connectionId":N}
type MeliBulkImportHandler struct {
	service *meliBulkImportApp.Service
}

func NewMeliBulkImportHandler(service *meliBulkImportApp.Service) *MeliBulkImportHandler {
	return &MeliBulkImportHandler{service: service}
}

func (h *MeliBulkImportHandler) Upload(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBulkImportFileBytes)
	if err := r.ParseMultipartForm(maxBulkImportFileBytes); err != nil {
		writeJSONError(w, http.StatusBadRequest, "No se pudo leer el archivo (máximo 10 MB).")
		return
	}
	connectionID, err := strconv.ParseInt(r.FormValue("connectionId"), 10, 64)
	if err != nil || connectionID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "Elige la conexión de MercadoLibre.")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Selecciona el archivo Excel.")
		return
	}
	defer file.Close()

	result, err := h.service.Upload(r.Context(), meliBulkImportApp.UploadInput{
		ConnectionID: connectionID,
		FileName:     header.Filename,
		File:         file,
		ActorID:      user.ID,
	})
	if err != nil {
		switch {
		case errors.Is(err, meliBulkImportApp.ErrInvalidInput):
			writeJSONError(w, http.StatusBadRequest, "Faltan la conexión o el archivo.")
		case errors.Is(err, meliBulkImportApp.ErrInvalidFile):
			writeJSONError(w, http.StatusBadRequest, "El archivo no es un Excel (.xlsx) válido.")
		case errors.Is(err, meliBulkImportApp.ErrMissingColumns):
			writeJSONError(w, http.StatusUnprocessableEntity, "El Excel debe tener las columnas sku y meli_id en la primera fila.")
		case errors.Is(err, meliBulkImportApp.ErrEmptyFile):
			writeJSONError(w, http.StatusUnprocessableEntity, "El Excel no tiene filas para importar.")
		case errors.Is(err, meliBulkImportApp.ErrTooManyRows):
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error()+".")
		case errors.Is(err, meliBulkImportApp.ErrConnectionNotFound):
			writeJSONError(w, http.StatusNotFound, "channel connection not found")
		case errors.Is(err, syncApp.ErrNotMercadoLibreConnection):
			writeJSONError(w, http.StatusBadRequest, "La conexión no es de MercadoLibre.")
		default:
			log.Printf("meli bulk import: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

func (h *MeliBulkImportHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := mysqlInfra.MeliImportItemFilter{
		Search:        q.Get("search"),
		FinishedOrder: q.Get("history") == "true",
		Offset:        queryInt(q.Get("offset"), 0),
		PageSize:      queryInt(q.Get("pageSize"), 25),
	}
	filter.BatchID = int64(queryInt(q.Get("batchId"), 0))
	filter.ConnectionID = int64(queryInt(q.Get("connectionId"), 0))
	if filter.PageSize > 200 {
		filter.PageSize = 200
	}
	for _, status := range strings.Split(q.Get("status"), ",") {
		if status = strings.TrimSpace(status); status != "" {
			if !bulkImportStatuses[status] {
				writeJSONError(w, http.StatusBadRequest, "invalid status")
				return
			}
			filter.Statuses = append(filter.Statuses, status)
		}
	}

	page, err := h.service.List(r.Context(), filter)
	if err != nil {
		log.Printf("meli bulk import: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSONResponse(w, map[string]any{
		"items":    page.Items,
		"total":    page.Total,
		"offset":   filter.Offset,
		"pageSize": filter.PageSize,
	})
}

func (h *MeliBulkImportHandler) ListBatches(w http.ResponseWriter, r *http.Request) {
	batches, err := h.service.ListBatches(r.Context(), int64(queryInt(r.URL.Query().Get("connectionId"), 0)))
	if err != nil {
		log.Printf("meli bulk import: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSONResponse(w, map[string]any{"batches": batches})
}

func (h *MeliBulkImportHandler) ExportBatch(w http.ResponseWriter, r *http.Request) {
	batchID, err := strconv.ParseInt(chi.URLParam(r, "batchId"), 10, 64)
	if err != nil || batchID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid batch id")
		return
	}

	fileName, content, err := h.service.ExportBatch(r.Context(), batchID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrMeliImportBatchNotFound) {
			writeJSONError(w, http.StatusNotFound, "No se encontró el lote.")
			return
		}
		log.Printf("meli bulk import: export batch %d: %v", batchID, err)
		writeJSONError(w, http.StatusBadGateway, "No se pudo generar el reporte; revisa la conexión con MercadoLibre e inténtalo de nuevo.")
		return
	}

	// The name is the uploaded file's, so strip anything that could break the header.
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	base = strings.Map(func(r rune) rune {
		if r < 32 || r == '"' || r == '\\' || r == '/' {
			return '_'
		}
		return r
	}, base)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="reporte-%s.xlsx"`, base))
	w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
	w.Write(content)
}

func (h *MeliBulkImportHandler) Summary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	counts, err := h.service.Summary(r.Context(), int64(queryInt(q.Get("connectionId"), 0)), int64(queryInt(q.Get("batchId"), 0)))
	if err != nil {
		log.Printf("meli bulk import: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSONResponse(w, counts)
}

func (h *MeliBulkImportHandler) Retry(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	itemID, err := strconv.ParseInt(chi.URLParam(r, "itemId"), 10, 64)
	if err != nil || itemID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid item id")
		return
	}

	if err := h.service.Retry(r.Context(), itemID, user.ID); err != nil {
		writeRetryError(w, err)
		return
	}
	writeJSONResponse(w, map[string]any{"requeued": 1})
}

type retryAllRequest struct {
	Status       string `json:"status"`
	ConnectionID int64  `json:"connectionId"`
}

func (h *MeliBulkImportHandler) RetryAll(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req retryAllRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	requeued, err := h.service.RetryAll(r.Context(), req.Status, req.ConnectionID, user.ID)
	if err != nil {
		writeRetryError(w, err)
		return
	}
	writeJSONResponse(w, map[string]any{"requeued": requeued})
}

func writeRetryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, mysqlInfra.ErrMeliImportItemNotFound):
		writeJSONError(w, http.StatusNotFound, "No se encontró la fila o ya no es reintentable.")
	case errors.Is(err, meliBulkImportApp.ErrItemNotRetryable):
		writeJSONError(w, http.StatusConflict, "Solo se pueden reintentar las filas con error o faltantes.")
	case errors.Is(err, meliBulkImportApp.ErrStillMissing):
		writeJSONError(w, http.StatusConflict, "El SKU sigue sin existir en la tabla de productos.")
	default:
		log.Printf("meli bulk import: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
	}
}

func queryInt(raw string, fallback int) int {
	if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
		return n
	}
	return fallback
}
