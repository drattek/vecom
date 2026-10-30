package mercadolibre

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OrdersHandler wraps MercadoLibre's /orders endpoints. Reads only: the
// shadow processor never writes orders back to MELI.
type OrdersHandler struct {
	client *Client
}

func NewOrdersHandler(client *Client) *OrdersHandler {
	if client == nil {
		client = NewClient(nil, "", nil)
	}

	return &OrdersHandler{client: client}
}

// OrderBuyer is the subset of the order's buyer payload the shadow table
// stores. billing_info is deliberately not consumed here — it goes into
// raw_payload as-is for the fase 2 CFDI consumer.
type OrderBuyer struct {
	ID        int64  `json:"id"`
	Nickname  string `json:"nickname"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

// OrderItemRef is one entry of order.order_items[], as MELI returns it.
// Only the fields the shadow table columns need are decoded; the full
// item blob is persisted via the raw payload.
type OrderItemRef struct {
	Item struct {
		ID          string `json:"id"`
		VariationID string `json:"variation_id"`
		Title       string `json:"title"`
		CategoryID  string `json:"category_id"`
		SellerSKU   string `json:"seller_sku"`
	} `json:"item"`
	Quantity      int     `json:"quantity"`
	UnitPrice     float64 `json:"unit_price"`
	FullUnitPrice float64 `json:"full_unit_price"`
	CurrencyID    string  `json:"currency_id"`
	SaleFee       float64 `json:"sale_fee"`
	ListingTypeID string  `json:"listing_type_id"`
}

// OrderShippingRef is the shipping sub-doc. The order payload only carries
// the shipping id (and sometimes limited info); the full detail is fetched
// via ShipmentsHandler.GetShipment when the shipments topic fires.
type OrderShippingRef struct {
	ID int64 `json:"id"`
}

type OrderPackRef struct {
	ID int64 `json:"id"`
}

type OrderPayment struct {
	ID               int64   `json:"id"`
	Status           string  `json:"status"`
	StatusDetail     string  `json:"status_detail"`
	TransactionAmount float64 `json:"transaction_amount"`
	CurrencyID       string  `json:"currency_id"`
	PaymentType      string  `json:"payment_type"`
	PaymentMethodID  string  `json:"payment_method_id"`
}

// OrderDetail is the subset of GET /orders/{id} this integration decodes
// into typed columns of ecom_meli_orders. Any field not listed here still
// survives via raw_payload (full JSON stored for audit and fase 2 consumer).
type OrderDetail struct {
	ID             int64            `json:"id"`
	Status         string           `json:"status"`
	StatusDetail   string           `json:"status_detail"`
	DateCreated    string           `json:"date_created"`
	DateClosed     string           `json:"date_closed"`
	LastUpdated    string           `json:"last_updated"`
	TotalAmount    float64          `json:"total_amount"`
	PaidAmount     float64          `json:"paid_amount"`
	CurrencyID     string           `json:"currency_id"`
	Buyer          OrderBuyer       `json:"buyer"`
	OrderItems     []OrderItemRef   `json:"order_items"`
	Shipping       OrderShippingRef `json:"shipping"`
	Pack           *OrderPackRef    `json:"pack_id"`
	Tags           []string         `json:"tags"`
	Payments       []OrderPayment   `json:"payments"`
}

// GetOrder calls GET /orders/{id}. externalID is the numeric MELI order id
// as a string (what the webhook delivers under `resource` as "/orders/N").
// Returns both the typed detail and the raw JSON — the raw is what the
// shadow repository persists verbatim in ecom_meli_orders.raw_payload.
func (h *OrdersHandler) GetOrder(ctx context.Context, accessToken, externalID string) (*OrderDetail, []byte, error) {
	requestURL := fmt.Sprintf("%s/orders/%s", h.client.baseURL, externalID)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating mercadolibre orders request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := h.client.Do(request)
	if err != nil {
		return nil, nil, fmt.Errorf("error calling mercadolibre orders endpoint: %w", err)
	}
	defer response.Body.Close()

	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading mercadolibre orders response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, responsePayload, fmt.Errorf("mercadolibre order fetch failed: %w", parseItemAPIError(response.StatusCode, responsePayload))
	}

	var order OrderDetail
	if err := json.Unmarshal(responsePayload, &order); err != nil {
		return nil, responsePayload, fmt.Errorf("error decoding mercadolibre order response: %w", err)
	}

	return &order, responsePayload, nil
}
