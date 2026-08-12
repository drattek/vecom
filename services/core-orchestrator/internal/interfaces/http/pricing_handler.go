package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	pricingApp "core-orchestrator/internal/application/pricing"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type upsertPriceListRequest struct {
	Name      string `json:"name"`
	Currency  int64  `json:"currency"`
	Priority  int    `json:"priority"`
	Status    string `json:"status"`
	ValidFrom string `json:"validFrom"`
	ValidTo   string `json:"validTo"`
}

type PriceListHandler struct {
	service *pricingApp.PriceListService
}

func NewPriceListHandler(service *pricingApp.PriceListService) *PriceListHandler {
	return &PriceListHandler{service: service}
}

func (h *PriceListHandler) GetPriceLists(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.GetPaginatedPriceLists(offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *PriceListHandler) GetPriceListByID(w http.ResponseWriter, r *http.Request) {
	id := parsePriceListID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid price list id")
		return
	}

	pl, err := h.service.GetPriceListByID(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrPriceListNotFound) {
			writeJSONError(w, http.StatusNotFound, "price list not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pl)
}

func (h *PriceListHandler) CreatePriceList(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertPriceListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreatePriceListInput{
		Name:      req.Name,
		Currency:  req.Currency,
		Priority:  req.Priority,
		Status:    req.Status,
		ValidFrom: req.ValidFrom,
		ValidTo:   req.ValidTo,
		CreatedBy: user.ID,
	}

	pl, err := h.service.CreatePriceList(input)
	if err != nil {
		if errors.Is(err, pricingApp.ErrInvalidPricingPayload) {
			writeJSONError(w, http.StatusBadRequest, "name is required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(pl)
}

func (h *PriceListHandler) UpdatePriceList(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := parsePriceListID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid price list id")
		return
	}

	var req upsertPriceListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdatePriceListInput{
		Name:      req.Name,
		Currency:  req.Currency,
		Priority:  req.Priority,
		Status:    req.Status,
		ValidFrom: req.ValidFrom,
		ValidTo:   req.ValidTo,
		UpdatedBy: user.ID,
	}

	pl, err := h.service.UpdatePriceList(id, input)
	if err != nil {
		if errors.Is(err, pricingApp.ErrInvalidPricingPayload) {
			writeJSONError(w, http.StatusBadRequest, "name is required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrPriceListNotFound) {
			writeJSONError(w, http.StatusNotFound, "price list not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pl)
}

func (h *PriceListHandler) DeletePriceList(w http.ResponseWriter, r *http.Request) {
	id := parsePriceListID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid price list id")
		return
	}

	err := h.service.DeletePriceList(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrPriceListNotFound) {
			writeJSONError(w, http.StatusNotFound, "price list not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parsePriceListID(r *http.Request) int64 {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	return id
}

// ProductPricesHandler
type upsertProductPriceRequest struct {
	ProductID   int64  `json:"productId"`
	PriceListID int64  `json:"priceListId"`
	Price       string `json:"price"`
	Currency    int64  `json:"currency"`
	Margin      string `json:"margin"`
	TaxIncluded bool   `json:"taxIncluded"`
}

type ProductPricesHandler struct {
	service *pricingApp.ProductPricesService
}

func NewProductPricesHandler(service *pricingApp.ProductPricesService) *ProductPricesHandler {
	return &ProductPricesHandler{service: service}
}

func (h *ProductPricesHandler) GetProductPrices(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.GetPaginatedProductPrices(offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *ProductPricesHandler) GetProductPriceByID(w http.ResponseWriter, r *http.Request) {
	id := parseProductPriceID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product price id")
		return
	}

	price, err := h.service.GetProductPriceByID(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductPriceNotFound) {
			writeJSONError(w, http.StatusNotFound, "product price not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(price)
}

func (h *ProductPricesHandler) CreateProductPrice(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertProductPriceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateProductPriceInput{
		ProductID:   req.ProductID,
		PriceListID: req.PriceListID,
		Price:       req.Price,
		Currency:    req.Currency,
		Margin:      req.Margin,
		TaxIncluded: req.TaxIncluded,
		UpdatedBy:   user.ID,
	}

	price, err := h.service.CreateProductPrice(input)
	if err != nil {
		if errors.Is(err, pricingApp.ErrInvalidPricingPayload) {
			writeJSONError(w, http.StatusBadRequest, "productId and priceListId are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(price)
}

func (h *ProductPricesHandler) UpdateProductPrice(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := parseProductPriceID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product price id")
		return
	}

	var req struct {
		Price       string `json:"price"`
		Margin      string `json:"margin"`
		TaxIncluded bool   `json:"taxIncluded"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateProductPriceInput{
		Price:       req.Price,
		Margin:      req.Margin,
		TaxIncluded: req.TaxIncluded,
		UpdatedBy:   user.ID,
	}

	price, err := h.service.UpdateProductPrice(id, input)
	if err != nil {
		if errors.Is(err, pricingApp.ErrInvalidPricingPayload) {
			writeJSONError(w, http.StatusBadRequest, "price is required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrProductPriceNotFound) {
			writeJSONError(w, http.StatusNotFound, "product price not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(price)
}

func (h *ProductPricesHandler) DeleteProductPrice(w http.ResponseWriter, r *http.Request) {
	id := parseProductPriceID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product price id")
		return
	}

	err := h.service.DeleteProductPrice(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductPriceNotFound) {
			writeJSONError(w, http.StatusNotFound, "product price not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseProductPriceID(r *http.Request) int64 {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	return id
}

// ExchangeRatesHandler
type upsertExchangeRateRequest struct {
	FromCurrencyID int64  `json:"fromCurrencyId"`
	ToCurrencyID   int64  `json:"toCurrencyId"`
	Rate           string `json:"rate"`
}

type ExchangeRatesHandler struct {
	service *pricingApp.ExchangeRatesService
}

func NewExchangeRatesHandler(service *pricingApp.ExchangeRatesService) *ExchangeRatesHandler {
	return &ExchangeRatesHandler{service: service}
}

func (h *ExchangeRatesHandler) GetExchangeRates(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.GetPaginatedExchangeRates(offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *ExchangeRatesHandler) GetExchangeRateByID(w http.ResponseWriter, r *http.Request) {
	id := parseExchangeRateID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid exchange rate id")
		return
	}

	rate, err := h.service.GetExchangeRateByID(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrExchangeRateNotFound) {
			writeJSONError(w, http.StatusNotFound, "exchange rate not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rate)
}

func (h *ExchangeRatesHandler) CreateExchangeRate(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertExchangeRateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateExchangeRateInput{
		FromCurrencyID: req.FromCurrencyID,
		ToCurrencyID:   req.ToCurrencyID,
		Rate:           req.Rate,
		UpdatedBy:      user.ID,
	}

	rate, err := h.service.CreateExchangeRate(input)
	if err != nil {
		if errors.Is(err, pricingApp.ErrInvalidPricingPayload) {
			writeJSONError(w, http.StatusBadRequest, "fromCurrencyId, toCurrencyId and rate are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rate)
}

func (h *ExchangeRatesHandler) UpdateExchangeRate(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := parseExchangeRateID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid exchange rate id")
		return
	}

	var req struct {
		Rate string `json:"rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateExchangeRateInput{
		Rate:      req.Rate,
		UpdatedBy: user.ID,
	}

	rate, err := h.service.UpdateExchangeRate(id, input)
	if err != nil {
		if errors.Is(err, pricingApp.ErrInvalidPricingPayload) {
			writeJSONError(w, http.StatusBadRequest, "rate is required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrExchangeRateNotFound) {
			writeJSONError(w, http.StatusNotFound, "exchange rate not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rate)
}

func (h *ExchangeRatesHandler) DeleteExchangeRate(w http.ResponseWriter, r *http.Request) {
	id := parseExchangeRateID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid exchange rate id")
		return
	}

	err := h.service.DeleteExchangeRate(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrExchangeRateNotFound) {
			writeJSONError(w, http.StatusNotFound, "exchange rate not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseExchangeRateID(r *http.Request) int64 {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	return id
}
