package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	currenciesApp "core-orchestrator/internal/application/currencies"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type CurrencyHandler struct {
	service *currenciesApp.CurrencyService
}

type upsertCurrencyRequest struct {
	Name          string `json:"name"`
	Code          string `json:"code"`
	Symbol        string `json:"symbol"`
	DecimalPlaces int    `json:"decimalPlaces"`
}

func NewCurrencyHandler(service *currenciesApp.CurrencyService) *CurrencyHandler {
	return &CurrencyHandler{service: service}
}

func (h *CurrencyHandler) GetCurrencies(w http.ResponseWriter, r *http.Request) {
	offsetStr := r.URL.Query().Get("offset")
	pageSizeStr := r.URL.Query().Get("pageSize")

	offset := 0
	pageSize := 10

	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsedOffset
		}
	}

	if pageSizeStr != "" {
		if parsedPageSize, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = parsedPageSize
		}
	}

	result, err := h.service.GetPaginatedCurrencies(r.Context(), offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *CurrencyHandler) GetCurrencyByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseCurrencyID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid currency id")
		return
	}

	currency, err := h.service.GetCurrencyByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrCurrencyNotFound) {
			writeJSONError(w, http.StatusNotFound, "currency not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(currency)
}

func (h *CurrencyHandler) CreateCurrency(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertCurrencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateCurrencyInput{
		Name:          req.Name,
		Code:          req.Code,
		Symbol:        req.Symbol,
		DecimalPlaces: req.DecimalPlaces,
		CreatedBy:     user.ID,
	}

	currency, err := h.service.CreateCurrency(r.Context(), input)
	if err != nil {
		if errors.Is(err, currenciesApp.ErrInvalidCurrencyPayload) {
			writeJSONError(w, http.StatusBadRequest, "name, code, and symbol are required; decimalPlaces must be 0-8")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(currency)
}

func (h *CurrencyHandler) UpdateCurrency(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseCurrencyID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid currency id")
		return
	}

	var req upsertCurrencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateCurrencyInput{
		Name:          req.Name,
		Code:          req.Code,
		Symbol:        req.Symbol,
		DecimalPlaces: req.DecimalPlaces,
		UpdatedBy:     user.ID,
	}

	currency, err := h.service.UpdateCurrency(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, currenciesApp.ErrInvalidCurrencyPayload) {
			writeJSONError(w, http.StatusBadRequest, "name, code, and symbol are required; decimalPlaces must be 0-8")
			return
		}
		if errors.Is(err, mysqlInfra.ErrCurrencyNotFound) {
			writeJSONError(w, http.StatusNotFound, "currency not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(currency)
}

func (h *CurrencyHandler) DeleteCurrency(w http.ResponseWriter, r *http.Request) {
	id, err := parseCurrencyID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid currency id")
		return
	}

	err = h.service.DeleteCurrency(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrCurrencyNotFound) {
			writeJSONError(w, http.StatusNotFound, "currency not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseCurrencyID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}
