package mercadolibre

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ShipmentsHandler wraps MercadoLibre's /shipments endpoint. Read-only in
// shadow mode: the processor only observes shipment state, never updates
// it.
type ShipmentsHandler struct {
	client *Client
}

func NewShipmentsHandler(client *Client) *ShipmentsHandler {
	if client == nil {
		client = NewClient(nil, "", nil)
	}

	return &ShipmentsHandler{client: client}
}

// ShipmentDetail is the subset of GET /shipments/{id} the shadow table
// decodes into typed columns. Everything else survives via raw_payload.
type ShipmentDetail struct {
	ID             int64  `json:"id"`
	OrderID        int64  `json:"order_id"`
	Status         string `json:"status"`
	SubStatus      string `json:"substatus"`
	Mode           string `json:"mode"`
	TrackingNumber string `json:"tracking_number"`
	TrackingMethod string `json:"tracking_method"`
	ServiceID      int64  `json:"service_id"`
	DateCreated    string `json:"date_created"`
	LastUpdated    string `json:"last_updated"`
}

// GetShipment calls GET /shipments/{id}. Shipping details are only
// visible to the seller — the OAuth token must be a seller token (which is
// the case for the tokens stored in ecom_connection_credentials).
func (h *ShipmentsHandler) GetShipment(ctx context.Context, accessToken, externalID string) (*ShipmentDetail, []byte, error) {
	requestURL := fmt.Sprintf("%s/shipments/%s", h.client.baseURL, externalID)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating mercadolibre shipments request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	// MercadoLibre's shipments endpoint requires the x-format-new header to
	// return the current (post-2022) payload shape — the legacy shape omits
	// substatus and tracking fields on some service modes.
	request.Header.Set("x-format-new", "true")

	response, err := h.client.Do(request)
	if err != nil {
		return nil, nil, fmt.Errorf("error calling mercadolibre shipments endpoint: %w", err)
	}
	defer response.Body.Close()

	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading mercadolibre shipments response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, responsePayload, fmt.Errorf("mercadolibre shipment fetch failed: %w", parseItemAPIError(response.StatusCode, responsePayload))
	}

	var shipment ShipmentDetail
	if err := json.Unmarshal(responsePayload, &shipment); err != nil {
		return nil, responsePayload, fmt.Errorf("error decoding mercadolibre shipment response: %w", err)
	}

	return &shipment, responsePayload, nil
}
