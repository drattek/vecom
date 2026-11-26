package mercadolibre

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// SalesHandler consulta las ventas del vendedor y sus datos relacionados
// (ADR 0010). Solo lectura. Usa tipos propios, separados de OrdersHandler y
// ShipmentsHandler (que sirven al procesador de notificaciones).
type SalesHandler struct {
	client *Client
}

func NewSalesHandler(client *Client) *SalesHandler {
	if client == nil {
		client = NewClient(nil, "", nil)
	}
	return &SalesHandler{client: client}
}

// FlexString acepta un identificador que MercadoLibre envía como número, como
// texto o como null, sin perder precisión.
type FlexString string

func (f *FlexString) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		*f = ""
		return nil
	}
	if trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return err
		}
		*f = FlexString(strings.TrimSpace(text))
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(trimmed, &number); err != nil {
		return fmt.Errorf("identificador inesperado: %w", err)
	}
	*f = FlexString(number.String())
	return nil
}

// SalesAPIError es una respuesta no exitosa de MercadoLibre.
type SalesAPIError struct {
	Resource string
	Status   int
	Err      error
}

func (e *SalesAPIError) Error() string {
	return fmt.Sprintf("mercadolibre %s: %v", e.Resource, e.Err)
}

func (e *SalesAPIError) Unwrap() error { return e.Err }

// IsSalesNotFound indica que el recurso no existe (404).
func IsSalesNotFound(err error) bool {
	var apiErr *SalesAPIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
}

type SalesOrder struct {
	ID              FlexString      `json:"id"`
	Status          string          `json:"status"`
	StatusDetail    json.RawMessage `json:"status_detail"`
	DateCreated     string          `json:"date_created"`
	DateClosed      string          `json:"date_closed"`
	LastUpdated     string          `json:"last_updated"`
	DateLastUpdated string          `json:"date_last_updated"`
	TotalAmount     *float64        `json:"total_amount"`
	PaidAmount      *float64        `json:"paid_amount"`
	CurrencyID      string          `json:"currency_id"`
	PackID          FlexString      `json:"pack_id"`
	Tags            []string        `json:"tags"`
	Context         struct {
		Site string `json:"site"`
	} `json:"context"`
	Coupon *struct {
		Amount *float64 `json:"amount"`
	} `json:"coupon"`
	Taxes *struct {
		Amount *float64 `json:"amount"`
	} `json:"taxes"`
	Buyer    SalesBuyer          `json:"buyer"`
	Items    []SalesOrderItem    `json:"order_items"`
	Payments []SalesOrderPayment `json:"payments"`
	Shipping struct {
		ID FlexString `json:"id"`
	} `json:"shipping"`
}

type SalesBuyer struct {
	ID        FlexString `json:"id"`
	Nickname  string     `json:"nickname"`
	FirstName string     `json:"first_name"`
	LastName  string     `json:"last_name"`
	Email     string     `json:"email"`
	Phone     *struct {
		AreaCode FlexString `json:"area_code"`
		Number   FlexString `json:"number"`
	} `json:"phone"`
	BillingInfo *struct {
		ID FlexString `json:"id"`
	} `json:"billing_info"`
}

type SalesOrderItem struct {
	Item struct {
		ID                FlexString `json:"id"`
		VariationID       FlexString `json:"variation_id"`
		Title             string     `json:"title"`
		SellerSKU         FlexString `json:"seller_sku"`
		SellerCustomField FlexString `json:"seller_custom_field"`
	} `json:"item"`
	Quantity      float64  `json:"quantity"`
	UnitPrice     *float64 `json:"unit_price"`
	FullUnitPrice *float64 `json:"full_unit_price"`
	// SaleFee es la comisión por unidad.
	SaleFee   *float64 `json:"sale_fee"`
	Discounts []struct {
		Amounts struct {
			Full   float64 `json:"full"`
			Seller float64 `json:"seller"`
		} `json:"amounts"`
	} `json:"discounts"`
}

type SalesOrderPayment struct {
	ID                        FlexString `json:"id"`
	Status                    string     `json:"status"`
	PaymentMethodID           string     `json:"payment_method_id"`
	PaymentType               string     `json:"payment_type"`
	Installments              *int       `json:"installments"`
	TransactionAmount         *float64   `json:"transaction_amount"`
	TransactionAmountRefunded *float64   `json:"transaction_amount_refunded"`
	DateApproved              string     `json:"date_approved"`
}

type salesSearchResponse struct {
	Results []json.RawMessage `json:"results"`
	Paging  struct {
		Total  int `json:"total"`
		Offset int `json:"offset"`
		Limit  int `json:"limit"`
	} `json:"paging"`
}

type SalesSearchPage struct {
	Orders []SalesSearchResult
	Total  int
}

// SalesSearchResult conserva el JSON de cada orden: si una no se puede
// decodificar, las demás de la página siguen disponibles.
type SalesSearchResult struct {
	Order SalesOrder
	Raw   json.RawMessage
	Err   error
}

// SearchOrdersParams: DateField es "date_created" o "date_last_updated".
type SearchOrdersParams struct {
	SellerID  int64
	DateField string
	From      string
	Offset    int
	Limit     int
}

func (h *SalesHandler) SearchOrders(ctx context.Context, accessToken string, params SearchOrdersParams) (*SalesSearchPage, error) {
	query := url.Values{}
	query.Set("seller", strconv.FormatInt(params.SellerID, 10))
	query.Set("order."+params.DateField+".from", params.From)
	query.Set("sort", "date_asc")
	query.Set("offset", strconv.Itoa(params.Offset))
	query.Set("limit", strconv.Itoa(params.Limit))

	body, err := h.get(ctx, accessToken, "orders/search", "/orders/search?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var response salesSearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error decoding mercadolibre orders search: %w", err)
	}

	page := &SalesSearchPage{Total: response.Paging.Total, Orders: make([]SalesSearchResult, 0, len(response.Results))}
	for _, raw := range response.Results {
		result := SalesSearchResult{Raw: raw}
		if err := json.Unmarshal(raw, &result.Order); err != nil {
			result.Err = fmt.Errorf("error decoding mercadolibre order: %w", err)
		}
		page.Orders = append(page.Orders, result)
	}
	return page, nil
}

func (h *SalesHandler) GetOrder(ctx context.Context, accessToken, orderID string) (*SalesOrder, []byte, error) {
	body, err := h.get(ctx, accessToken, "orders/"+orderID, "/orders/"+url.PathEscape(orderID), nil)
	if err != nil {
		return nil, nil, err
	}
	var order SalesOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, nil, fmt.Errorf("error decoding mercadolibre order %s: %w", orderID, err)
	}
	return &order, body, nil
}

type SalesNamed struct {
	Name string `json:"name"`
}

type SalesAddress struct {
	AddressLine  string     `json:"address_line"`
	StreetName   string     `json:"street_name"`
	StreetNumber FlexString `json:"street_number"`
	Comment      string     `json:"comment"`
	ZipCode      FlexString `json:"zip_code"`
	Neighborhood SalesNamed `json:"neighborhood"`
	City         SalesNamed `json:"city"`
	State        SalesNamed `json:"state"`
	Country      SalesNamed `json:"country"`
	// Formato anterior: destinatario dentro de receiver_address.
	ReceiverName  string     `json:"receiver_name"`
	ReceiverPhone FlexString `json:"receiver_phone"`
}

type salesDated struct {
	Date string `json:"date"`
}

// SalesShipment acepta las dos formas documentadas de /shipments/{id}: la
// plana (mode, logistic_type, receiver_address) y la de x-format-new
// (logistic, destination, lead_time).
type SalesShipment struct {
	ID             FlexString `json:"id"`
	Status         string     `json:"status"`
	Substatus      string     `json:"substatus"`
	Mode           string     `json:"mode"`
	LogisticType   string     `json:"logistic_type"`
	TrackingNumber FlexString `json:"tracking_number"`
	TrackingMethod string     `json:"tracking_method"`
	Logistic       struct {
		Mode string `json:"mode"`
		Type string `json:"type"`
	} `json:"logistic"`
	Destination *struct {
		ReceiverName    string        `json:"receiver_name"`
		ReceiverPhone   FlexString    `json:"receiver_phone"`
		ShippingAddress *SalesAddress `json:"shipping_address"`
	} `json:"destination"`
	ReceiverAddress *SalesAddress `json:"receiver_address"`
	LeadTime        *struct {
		EstimatedDeliveryTime  salesDated `json:"estimated_delivery_time"`
		EstimatedHandlingLimit salesDated `json:"estimated_handling_limit"`
	} `json:"lead_time"`
	ShippingOption *struct {
		EstimatedDeliveryTime salesDated `json:"estimated_delivery_time"`
	} `json:"shipping_option"`
	StatusHistory struct {
		DateShipped   string `json:"date_shipped"`
		DateDelivered string `json:"date_delivered"`
	} `json:"status_history"`
}

func (h *SalesHandler) GetShipment(ctx context.Context, accessToken, shipmentID string) (*SalesShipment, []byte, error) {
	body, err := h.get(ctx, accessToken, "shipments/"+shipmentID,
		"/shipments/"+url.PathEscape(shipmentID)+"?views=destination",
		map[string]string{"x-format-new": "true"})
	if err != nil {
		return nil, nil, err
	}
	var shipment SalesShipment
	if err := json.Unmarshal(body, &shipment); err != nil {
		return nil, nil, fmt.Errorf("error decoding mercadolibre shipment %s: %w", shipmentID, err)
	}
	return &shipment, body, nil
}

type SalesShipmentCosts struct {
	Receiver *struct {
		Cost *float64 `json:"cost"`
	} `json:"receiver"`
	Senders []struct {
		Cost float64 `json:"cost"`
	} `json:"senders"`
}

func (h *SalesHandler) GetShipmentCosts(ctx context.Context, accessToken, shipmentID string) (*SalesShipmentCosts, error) {
	body, err := h.get(ctx, accessToken, "shipments/"+shipmentID+"/costs",
		"/shipments/"+url.PathEscape(shipmentID)+"/costs",
		map[string]string{"x-format-new": "true"})
	if err != nil {
		return nil, err
	}
	var costs SalesShipmentCosts
	if err := json.Unmarshal(body, &costs); err != nil {
		return nil, fmt.Errorf("error decoding mercadolibre shipment costs %s: %w", shipmentID, err)
	}
	return &costs, nil
}

// GetShipmentDispatchDeadline devuelve la fecha límite de despacho (SLA).
func (h *SalesHandler) GetShipmentDispatchDeadline(ctx context.Context, accessToken, shipmentID string) (string, error) {
	body, err := h.get(ctx, accessToken, "shipments/"+shipmentID+"/sla",
		"/shipments/"+url.PathEscape(shipmentID)+"/sla", nil)
	if err != nil {
		return "", err
	}
	var sla struct {
		ExpectedDate string `json:"expected_date"`
	}
	if err := json.Unmarshal(body, &sla); err != nil {
		return "", fmt.Errorf("error decoding mercadolibre shipment sla %s: %w", shipmentID, err)
	}
	return sla.ExpectedDate, nil
}

type SalesBillingInfo struct {
	Buyer struct {
		BillingInfo struct {
			Name           string `json:"name"`
			LastName       string `json:"last_name"`
			Identification struct {
				Type   string     `json:"type"`
				Number FlexString `json:"number"`
			} `json:"identification"`
			Taxes struct {
				TaxpayerType struct {
					ID          FlexString `json:"id"`
					Description string     `json:"description"`
				} `json:"taxpayer_type"`
				CFDI struct {
					ID          FlexString `json:"id"`
					Description string     `json:"description"`
				} `json:"cfdi"`
			} `json:"taxes"`
			Address struct {
				StreetName   string     `json:"street_name"`
				StreetNumber FlexString `json:"street_number"`
				CityName     string     `json:"city_name"`
				ZipCode      FlexString `json:"zip_code"`
				CountryID    string     `json:"country_id"`
				State        SalesNamed `json:"state"`
			} `json:"address"`
		} `json:"billing_info"`
	} `json:"buyer"`
}

func (h *SalesHandler) GetBillingInfo(ctx context.Context, accessToken, siteID, billingInfoID string) (*SalesBillingInfo, []byte, error) {
	body, err := h.get(ctx, accessToken, "orders/billing-info",
		"/orders/billing-info/"+url.PathEscape(siteID)+"/"+url.PathEscape(billingInfoID), nil)
	if err != nil {
		return nil, nil, err
	}
	var info SalesBillingInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, nil, fmt.Errorf("error decoding mercadolibre billing info: %w", err)
	}
	return &info, body, nil
}

// get acepta 200 y 206: MercadoLibre responde 206 cuando omite parte de los
// datos (p. ej. el comprador). resource identifica la consulta en el error,
// sin parámetros ni cuerpo de la respuesta.
func (h *SalesHandler) get(ctx context.Context, accessToken, resource, path string, headers map[string]string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, h.client.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating mercadolibre %s request: %w", resource, err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := h.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error calling mercadolibre %s: %w", resource, err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading mercadolibre %s response: %w", resource, err)
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		return nil, &SalesAPIError{Resource: resource, Status: response.StatusCode, Err: salesErrorDetail(response.StatusCode, body)}
	}
	return body, nil
}

// salesErrorDetail toma solo el mensaje y el código del error. A diferencia de
// parseItemAPIError no incluye el cuerpo: podría traer datos del comprador.
func salesErrorDetail(status int, body []byte) error {
	var payload struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	_ = json.Unmarshal(body, &payload)
	return fmt.Errorf("(%d): %s %s", status, payload.Error, payload.Message)
}
