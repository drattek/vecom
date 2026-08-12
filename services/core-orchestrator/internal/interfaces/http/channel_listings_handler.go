package http

import (
	"encoding/json"
	"errors"
	"net/http"

	channelListingsApp "core-orchestrator/internal/application/channel_listings"
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
