package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	channelListingsApp "core-orchestrator/internal/application/channel_listings"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type ChannelListingsHandler struct {
	service *channelListingsApp.Service
}

func NewChannelListingsHandler(service *channelListingsApp.Service) *ChannelListingsHandler {
	return &ChannelListingsHandler{service: service}
}

type createChannelListingsRequest struct {
	ConnectionID int64    `json:"connectionId"`
	SKUs         []string `json:"skus"`
	// OfficialStoreID is forwarded as-is to MercadoLibre's official_store_id
	// item field: nil (the default, whether omitted or sent as JSON null)
	// publishes with no official store assigned; a value assigns the listing
	// to that store. Ignored by channels other than MercadoLibre.
	OfficialStoreID *int64 `json:"officialStoreId"`
}

// CreateListings accepts a connectionId and a list of skus, resolves the
// connection's channel (MercadoLibre, Odoo, or any future one registered on
// the underlying channel_listings.Service), and creates one new listing per
// sku — or one per vehicle compatibility on connections that allow multiple
// listings per product. It only ever creates: a sku (or sku+compatibility)
// that already has a listing on the connection is reported as skipped, never
// updated. It is independent from the MercadoLibre upload/update-price-stock
// endpoints.
func (h *ChannelListingsHandler) CreateListings(w http.ResponseWriter, r *http.Request) {
	var req createChannelListingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.CreateListings(r.Context(), channelListingsApp.CreateListingsInput{
		ConnectionID:    req.ConnectionID,
		SKUs:            req.SKUs,
		OfficialStoreID: req.OfficialStoreID,
	})
	if err != nil {
		if errors.Is(err, channelListingsApp.ErrEmptyChannelListingsInput) {
			writeJSONError(w, http.StatusBadRequest, "connectionId and at least one sku are required")
			return
		}
		if errors.Is(err, channelListingsApp.ErrInvalidChannelConnection) {
			writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
			return
		}
		if errors.Is(err, channelListingsApp.ErrNoPublisherForChannel) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

type publishZeroStockChannelListingsRequest struct {
	ConnectionID    int64  `json:"connectionId"`
	OfficialStoreID *int64 `json:"officialStoreId"`
	Limit           int    `json:"limit"`
}

// PublishZeroStockEligibles accepts a connectionId and publishes every
// active product on that connection that has a cover image, a price, a
// brand, a category and zero on-hand stock but no listing yet — the batch
// behind the "Publicar elegibles sin stock ahora" button. Delegates to
// channel_listings.PublishZeroStockEligibles, which internally calls
// CreateListings. Returns the same shape (list of per-SKU outcomes),
// including an empty list when nothing was eligible.
func (h *ChannelListingsHandler) PublishZeroStockEligibles(w http.ResponseWriter, r *http.Request) {
	var req publishZeroStockChannelListingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.PublishZeroStockEligibles(r.Context(), channelListingsApp.PublishZeroStockEligiblesInput{
		ConnectionID:    req.ConnectionID,
		OfficialStoreID: req.OfficialStoreID,
		Limit:           req.Limit,
	})
	if err != nil {
		if errors.Is(err, channelListingsApp.ErrEmptyChannelListingsInput) {
			writeJSONError(w, http.StatusBadRequest, "connectionId is required")
			return
		}
		if errors.Is(err, channelListingsApp.ErrInvalidChannelConnection) {
			writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
			return
		}
		if errors.Is(err, channelListingsApp.ErrNoPublisherForChannel) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

type refreshChannelListingsRequest struct {
	ConnectionID int64 `json:"connectionId"`
}

// RefreshListings accepts a connectionId and pushes a price/stock refresh
// (plus whatever else that channel's Refresher keeps bundled into the same
// call — category on MercadoLibre, name on Odoo) to every already-published
// listing recorded in ecom_channel_product_map for that connection, but only
// the ones whose product's price or stock actually changed since they were
// last synced. It never creates anything new — use CreateListings for that.
func (h *ChannelListingsHandler) RefreshListings(w http.ResponseWriter, r *http.Request) {
	var req refreshChannelListingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.RefreshListings(r.Context(), channelListingsApp.RefreshListingsInput{
		ConnectionID: req.ConnectionID,
	})
	if err != nil {
		if errors.Is(err, channelListingsApp.ErrInvalidChannelConnection) {
			writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
			return
		}
		if errors.Is(err, channelListingsApp.ErrNoRefresherForChannel) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

type resyncChannelListingsRequest struct {
	ConnectionID int64 `json:"connectionId"`
	ProductID    int64 `json:"productId"`
}

// ResyncListings accepts a connectionId and productId and pushes every field
// that connection's channel allows changing on an already-published listing
// (not just price/stock, and not gated on whether anything actually
// changed) to every ecom_channel_product_map row already recorded for that
// (product, connection) pair — see channel_listings.Service.
// ResyncProductListings. Driven by the "Resincronizar" button on the
// product detail page's Sincronización section; scoped to a single product,
// unlike RefreshListings which sweeps every product on a connection.
func (h *ChannelListingsHandler) ResyncListings(w http.ResponseWriter, r *http.Request) {
	var req resyncChannelListingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.ResyncProductListings(r.Context(), channelListingsApp.ResyncListingsInput{
		ProductID:    req.ProductID,
		ConnectionID: req.ConnectionID,
	})
	if err != nil {
		if errors.Is(err, channelListingsApp.ErrInvalidProductID) {
			writeJSONError(w, http.StatusBadRequest, "invalid productId")
			return
		}
		if errors.Is(err, channelListingsApp.ErrInvalidChannelConnection) {
			writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
			return
		}
		if errors.Is(err, channelListingsApp.ErrNoFullRefresherForChannel) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// SyncCompatibilities accepts a connectionId and productId and reports only
// the product's vehicle compatibilities to its already-published listings on
// that connection (no price/stock/attributes/pictures). Driven by the
// "Sincronizar compatibilidades" button on the product detail page's
// Sincronización section.
func (h *ChannelListingsHandler) SyncCompatibilities(w http.ResponseWriter, r *http.Request) {
	var req resyncChannelListingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.SyncProductCompatibilities(r.Context(), channelListingsApp.ResyncListingsInput{
		ProductID:    req.ProductID,
		ConnectionID: req.ConnectionID,
	})
	if err != nil {
		if errors.Is(err, channelListingsApp.ErrInvalidProductID) {
			writeJSONError(w, http.StatusBadRequest, "invalid productId")
			return
		}
		if errors.Is(err, channelListingsApp.ErrInvalidChannelConnection) {
			writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
			return
		}
		if errors.Is(err, channelListingsApp.ErrNoCompatibilitiesSyncerForChannel) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

type closeChannelListingsRequest struct {
	ProductID    int64   `json:"productId"`
	ConnectionID int64   `json:"connectionId"`
	ListingIDs   []int64 `json:"listingIds"`
}

// CloseListings closes on the marketplace the given listings (ids of
// ecom_channel_product_map) of a (product, connection) pair — only paused,
// under_review or closed ones — so a following publish creates new listings
// (e.g. after the external category changed). Driven by the "Resincronizar"
// button on the product's Sincronización view.
func (h *ChannelListingsHandler) CloseListings(w http.ResponseWriter, r *http.Request) {
	var req closeChannelListingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.ListingIDs) == 0 {
		writeJSONError(w, http.StatusBadRequest, "listingIds is required")
		return
	}

	result, err := h.service.CloseProductListings(r.Context(), channelListingsApp.CloseListingsInput{
		ProductID:    req.ProductID,
		ConnectionID: req.ConnectionID,
		ListingIDs:   req.ListingIDs,
	})
	if err != nil {
		switch {
		case errors.Is(err, channelListingsApp.ErrInvalidProductID):
			writeJSONError(w, http.StatusBadRequest, "invalid productId")
		case errors.Is(err, channelListingsApp.ErrInvalidChannelConnection):
			writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
		case errors.Is(err, channelListingsApp.ErrNoCloserForChannel):
			writeJSONError(w, http.StatusBadRequest, err.Error())
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// ListListings backs the "Publicaciones" screen: every listing on a channel
// (live map rows + products whose last publish attempt failed), with a
// human-facing state and per-state counts.
// Query: channelCode (required), connectionId, state, withError, search,
// offset, pageSize.
func (h *ChannelListingsHandler) ListListings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	channelCode := strings.TrimSpace(q.Get("channelCode"))
	if channelCode == "" {
		writeJSONError(w, http.StatusBadRequest, "channelCode is required")
		return
	}
	connectionID, _ := strconv.ParseInt(q.Get("connectionId"), 10, 64)
	offset, _ := strconv.Atoi(q.Get("offset"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	withError := q.Get("withError") == "true" || q.Get("withError") == "1"

	result, err := h.service.ListListingsView(r.Context(), mysqlInfra.ListingViewFilter{
		ChannelCode:  channelCode,
		ConnectionID: connectionID,
		State:        strings.TrimSpace(q.Get("state")),
		WithError:    withError,
		Search:       q.Get("search"),
		Offset:       offset,
		PageSize:     pageSize,
	})
	if err != nil {
		log.Printf("channel listings view: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// ListingHistory returns every recorded event (publish attempts, refreshes,
// pauses, reactivations, marketplace status changes) for one
// (productId, connectionId, vehicleFitmentId) listing, newest first.
func (h *ChannelListingsHandler) ListingHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	productID, _ := strconv.ParseInt(q.Get("productId"), 10, 64)
	connectionID, _ := strconv.ParseInt(q.Get("connectionId"), 10, 64)
	var fitmentID *int64
	if raw := strings.TrimSpace(q.Get("vehicleFitmentId")); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil && v > 0 {
			fitmentID = &v
		}
	}

	events, err := h.service.ListingHistory(r.Context(), productID, connectionID, fitmentID)
	if err != nil {
		if errors.Is(err, channelListingsApp.ErrInvalidProductID) || errors.Is(err, channelListingsApp.ErrInvalidChannelConnection) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("channel listings history: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"events": events})
}
