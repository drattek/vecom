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

	productBulkUpdateApp "core-orchestrator/internal/application/product_bulk_update"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"

	"github.com/go-chi/chi/v5"
)

// maxProductBulkUpdateFileBytes acota el Excel subido.
const maxProductBulkUpdateFileBytes = 10 << 20

var productBulkUpdateStatuses = map[string]bool{
	mysqlInfra.ProductBulkUpdatePending: true,
	mysqlInfra.ProductBulkUpdateRunning: true,
	mysqlInfra.ProductBulkUpdateDone:    true,
	mysqlInfra.ProductBulkUpdateFailed:  true,
	mysqlInfra.ProductBulkUpdateMissing: true,
}

// ProductBulkUpdateHandler sirve "Productos → Actualización masiva": la subida
// del Excel que encola la actualización de productos para el worker de fondo y
// las vistas sobre esa cola y su historial.
//
//	POST /api/product-bulk-update/batches      multipart: file + sourceId + storageDiskId
//	GET  /api/product-bulk-update/batches      → archivos subidos, el más reciente primero
//	GET  /api/product-bulk-update/batches/{batchId}/export  → reporte .xlsx del lote
//	GET  /api/product-bulk-update/template     → plantilla .xlsx
//	GET  /api/product-bulk-update/items        ?status=a,b&batchId=&search=&history=true&offset=&pageSize=
//	GET  /api/product-bulk-update/summary      ?batchId=  → conteo por estado
//	POST /api/product-bulk-update/items/{itemId}/retry
//	POST /api/product-bulk-update/retry        {"status":"failed|missing","batchId":N}
type ProductBulkUpdateHandler struct {
	service *productBulkUpdateApp.Service
}

func NewProductBulkUpdateHandler(service *productBulkUpdateApp.Service) *ProductBulkUpdateHandler {
	return &ProductBulkUpdateHandler{service: service}
}

func (h *ProductBulkUpdateHandler) Upload(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxProductBulkUpdateFileBytes)
	if err := r.ParseMultipartForm(maxProductBulkUpdateFileBytes); err != nil {
		writeJSONError(w, http.StatusBadRequest, "No se pudo leer el archivo (máximo 10 MB).")
		return
	}
	sourceID, err := strconv.ParseInt(r.FormValue("sourceId"), 10, 64)
	if err != nil || sourceID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "Elige la source de los productos.")
		return
	}
	storageDiskID, err := strconv.ParseInt(r.FormValue("storageDiskId"), 10, 64)
	if err != nil || storageDiskID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "Elige el disco de almacenamiento.")
		return
	}
	// Conexiones donde resincronizar (opcional): syncConnectionIds repetido.
	var syncConnectionIDs []int64
	for _, raw := range r.Form["syncConnectionIds"] {
		id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || id <= 0 {
			writeJSONError(w, http.StatusBadRequest, "Una de las conexiones elegidas no es válida.")
			return
		}
		syncConnectionIDs = append(syncConnectionIDs, id)
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Selecciona el archivo Excel.")
		return
	}
	defer file.Close()

	result, err := h.service.Upload(r.Context(), productBulkUpdateApp.UploadInput{
		SourceID:          sourceID,
		StorageDiskID:     storageDiskID,
		SyncConnectionIDs: syncConnectionIDs,
		FileName:          header.Filename,
		File:              file,
		ActorID:           user.ID,
	})
	if err != nil {
		var invalidConn *productBulkUpdateApp.InvalidSyncConnectionError
		switch {
		case errors.As(err, &invalidConn):
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("La conexión %d no existe o su canal no permite resincronizar publicaciones.", invalidConn.ConnectionID))
		case errors.Is(err, productBulkUpdateApp.ErrInvalidInput):
			writeJSONError(w, http.StatusBadRequest, "Faltan la source, el disco o el archivo.")
		case errors.Is(err, productBulkUpdateApp.ErrInvalidFile):
			writeJSONError(w, http.StatusBadRequest, "El archivo no es un Excel (.xlsx) válido.")
		case errors.Is(err, productBulkUpdateApp.ErrMissingColumns):
			writeJSONError(w, http.StatusUnprocessableEntity, "El Excel debe tener la columna sku en la primera fila. Descarga la plantilla para ver las columnas.")
		case errors.Is(err, productBulkUpdateApp.ErrNoDataColumns):
			writeJSONError(w, http.StatusUnprocessableEntity, "El Excel solo tiene la columna sku; agrega al menos una columna con datos a actualizar.")
		case errors.Is(err, productBulkUpdateApp.ErrEmptyFile):
			writeJSONError(w, http.StatusUnprocessableEntity, "El Excel no tiene filas para actualizar.")
		case errors.Is(err, productBulkUpdateApp.ErrTooManyRows):
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error()+".")
		case errors.Is(err, productBulkUpdateApp.ErrSourceNotFound):
			writeJSONError(w, http.StatusNotFound, "No se encontró la source.")
		case errors.Is(err, productBulkUpdateApp.ErrStorageDiskNotFound):
			writeJSONError(w, http.StatusNotFound, "No se encontró el disco de almacenamiento.")
		default:
			log.Printf("product bulk update: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

func (h *ProductBulkUpdateHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := mysqlInfra.ProductBulkUpdateItemFilter{
		Search:        q.Get("search"),
		FinishedOrder: q.Get("history") == "true",
		Offset:        queryInt(q.Get("offset"), 0),
		PageSize:      queryInt(q.Get("pageSize"), 25),
		BatchID:       int64(queryInt(q.Get("batchId"), 0)),
	}
	if filter.PageSize > 200 {
		filter.PageSize = 200
	}
	for _, status := range strings.Split(q.Get("status"), ",") {
		if status = strings.TrimSpace(status); status != "" {
			if !productBulkUpdateStatuses[status] {
				writeJSONError(w, http.StatusBadRequest, "invalid status")
				return
			}
			filter.Statuses = append(filter.Statuses, status)
		}
	}

	page, err := h.service.List(r.Context(), filter)
	if err != nil {
		log.Printf("product bulk update: %v", err)
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

func (h *ProductBulkUpdateHandler) ListBatches(w http.ResponseWriter, r *http.Request) {
	batches, err := h.service.ListBatches(r.Context())
	if err != nil {
		log.Printf("product bulk update: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSONResponse(w, map[string]any{"batches": batches})
}

func (h *ProductBulkUpdateHandler) ExportBatch(w http.ResponseWriter, r *http.Request) {
	batchID, err := strconv.ParseInt(chi.URLParam(r, "batchId"), 10, 64)
	if err != nil || batchID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid batch id")
		return
	}

	fileName, content, err := h.service.ExportBatch(r.Context(), batchID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductBulkUpdateBatchNotFound) {
			writeJSONError(w, http.StatusNotFound, "No se encontró el lote.")
			return
		}
		log.Printf("product bulk update: export batch %d: %v", batchID, err)
		writeJSONError(w, http.StatusInternalServerError, "No se pudo generar el reporte.")
		return
	}

	// El nombre es el del archivo subido: se limpia lo que pueda romper el header.
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

func (h *ProductBulkUpdateHandler) Template(w http.ResponseWriter, r *http.Request) {
	content, err := h.service.Template()
	if err != nil {
		log.Printf("product bulk update: template: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "No se pudo generar la plantilla.")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="plantilla-actualizacion-masiva.xlsx"`)
	w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
	w.Write(content)
}

func (h *ProductBulkUpdateHandler) Summary(w http.ResponseWriter, r *http.Request) {
	counts, err := h.service.Summary(r.Context(), int64(queryInt(r.URL.Query().Get("batchId"), 0)))
	if err != nil {
		log.Printf("product bulk update: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSONResponse(w, counts)
}

func (h *ProductBulkUpdateHandler) Retry(w http.ResponseWriter, r *http.Request) {
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
		writeProductBulkUpdateRetryError(w, err)
		return
	}
	writeJSONResponse(w, map[string]any{"requeued": 1})
}

type productBulkUpdateRetryAllRequest struct {
	Status  string `json:"status"`
	BatchID int64  `json:"batchId"`
}

func (h *ProductBulkUpdateHandler) RetryAll(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req productBulkUpdateRetryAllRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	requeued, err := h.service.RetryAll(r.Context(), req.Status, req.BatchID, user.ID)
	if err != nil {
		writeProductBulkUpdateRetryError(w, err)
		return
	}
	writeJSONResponse(w, map[string]any{"requeued": requeued})
}

func writeProductBulkUpdateRetryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, mysqlInfra.ErrProductBulkUpdateItemNotFound):
		writeJSONError(w, http.StatusNotFound, "No se encontró la fila o ya no es reintentable. Si el error es del Excel, corrígelo y súbelo de nuevo.")
	case errors.Is(err, productBulkUpdateApp.ErrItemNotRetryable):
		writeJSONError(w, http.StatusConflict, "Solo se pueden reintentar las filas con error o con SKU inexistente.")
	case errors.Is(err, productBulkUpdateApp.ErrStillMissing):
		writeJSONError(w, http.StatusConflict, "El SKU sigue sin existir en la tabla de productos.")
	default:
		log.Printf("product bulk update: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
	}
}
