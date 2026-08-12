package mercadolibre

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type ItemsHandler struct {
	client *Client
}

func NewItemsHandler(client *Client) *ItemsHandler {
	if client == nil {
		client = NewClient(nil, "", nil)
	}

	return &ItemsHandler{client: client}
}

// ItemPicture is one entry of an item's pictures array: {"source": url}.
type ItemPicture struct {
	Source string `json:"source"`
}

// ItemAttribute is one entry of an item's attributes array. Most attributes
// are set by value_name (e.g. {"id": "BRAND", "value_name": "..."}), but a
// few — like ITEM_CONDITION — are set by value_id instead
// ({"id": "ITEM_CONDITION", "value_id": "2230284"}). Only set whichever one
// applies; the other is omitted from the JSON payload.
type ItemAttribute struct {
	ID        string `json:"id"`
	ValueID   string `json:"value_id,omitempty"`
	ValueName string `json:"value_name,omitempty"`
}

// CreateItemVals mirrors the /items fields this integration sets when
// listing a new product on MercadoLibre.
type CreateItemVals struct {
	Price             float64 `json:"price"`
	AvailableQuantity int     `json:"available_quantity"`
	CurrencyID        string  `json:"currency_id"`
	Condition         string  `json:"condition"`
	BuyingMode        string  `json:"buying_mode"`
	ListingTypeID     string  `json:"listing_type_id"`
	CategoryID        string  `json:"category_id"`
	// OfficialStoreID is deliberately sent with no `omitempty`: a nil value
	// must reach MercadoLibre as an explicit "official_store_id": null
	// rather than being left out of the payload entirely, since the two are
	// not guaranteed to behave the same way on MercadoLibre's side.
	OfficialStoreID *int64          `json:"official_store_id"`
	FamilyName      string          `json:"family_name"`
	Pictures        []ItemPicture   `json:"pictures"`
	Attributes      []ItemAttribute `json:"attributes"`
}

type CreateItemRequest struct {
	AccessToken string
	Vals        CreateItemVals
}

type CreateItemResponse struct {
	ID         string `json:"id"`
	CategoryID string `json:"category_id"`
}

// itemAPICause is one entry of MercadoLibre's "cause" array, which carries
// the actual field-level reason(s) behind a generic top-level message like
// "Validation error" (e.g. {"code": "item.attributes.mandatory", "message":
// "Attribute BRAND is mandatory"}).
type itemAPICause struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Attribute string `json:"attribute,omitempty"`
}

type itemAPIError struct {
	Message string         `json:"message"`
	Error   string         `json:"error"`
	Cause   []itemAPICause `json:"cause"`
}

// CreateItem calls POST /items to list a new product on MercadoLibre and
// returns the created item's id.
func (h *ItemsHandler) CreateItem(ctx context.Context, req CreateItemRequest) (*CreateItemResponse, error) {
	payload, err := json.Marshal(req.Vals)
	if err != nil {
		return nil, fmt.Errorf("error encoding mercadolibre item request: %w", err)
	}
	log.Printf("mercadolibre items: request payload: %s", payload)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, h.client.baseURL+"/items", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("error creating mercadolibre item request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+req.AccessToken)

	response, err := h.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error calling mercadolibre items endpoint: %w", err)
	}
	defer response.Body.Close()

	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading mercadolibre items response: %w", err)
	}

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("mercadolibre items create failed: %w", parseItemAPIError(response.StatusCode, responsePayload))
	}

	var created CreateItemResponse
	if err := json.Unmarshal(responsePayload, &created); err != nil {
		return nil, fmt.Errorf("error decoding mercadolibre items response: %w", err)
	}

	return &created, nil
}

// UpdateItemVals mirrors the subset of /items/{id} fields this integration
// refreshes on an already-listed item: price and stock (always sent
// together — MercadoLibre requires both on this endpoint), category when the
// caller wants that kept in sync too (channel_refresh's Refresh flow;
// omitted — CategoryID left empty — from the plain price/stock-only refresh
// path in updateExistingListings), and the SELLER_PACKAGE_* attributes when
// the caller wants a later ecom_product_dimensions correction to reach
// already-listed items too. Pictures and every other create-only attribute
// (BRAND, PART_NUMBER, ...) are still only sent via CreateItem.
type UpdateItemVals struct {
	Price             float64         `json:"price"`
	AvailableQuantity int             `json:"available_quantity"`
	CategoryID        string          `json:"category_id,omitempty"`
	Attributes        []ItemAttribute `json:"attributes,omitempty"`
}

type UpdateItemRequest struct {
	AccessToken string
	ExternalID  string
	Vals        UpdateItemVals
}

// UpdateItem calls PUT /items/{id} to refresh an already-listed item's
// price and stock.
func (h *ItemsHandler) UpdateItem(ctx context.Context, req UpdateItemRequest) error {
	payload, err := json.Marshal(req.Vals)
	if err != nil {
		return fmt.Errorf("error encoding mercadolibre item update request: %w", err)
	}
	return h.putItem(ctx, req.AccessToken, req.ExternalID, payload)
}

// ItemShipping mirrors the shipping object this integration sets per
// MercadoLibre's documented free_shipping payload: "mode": "me2" (Mercado
// Envíos, required for free_shipping to be accepted) and "free_shipping":
// true. store_pick_up is deliberately not sent — MercadoLibre rejects
// changing it on an existing listing.
type ItemShipping struct {
	Mode         string `json:"mode"`
	FreeShipping bool   `json:"free_shipping"`
}

// mercadoLibreShippingModeMercadoEnvios is MercadoLibre's "me2" shipping
// mode (Mercado Envíos) — required for free_shipping to be accepted.
const mercadoLibreShippingModeMercadoEnvios = "me2"

type UpdateItemShippingRequest struct {
	AccessToken string
	ExternalID  string
}

// UpdateItemShipping calls PUT /items/{id} with only the shipping field,
// setting it to {"mode": "me2", "free_shipping": true}. It never touches
// listing_type_id or shipping.store_pick_up — MercadoLibre rejects changing
// either once a listing already exists — nor price/stock/category;
// MercadoLibre's PUT /items/{id} is a partial update, so a field omitted
// here is left as-is on the listing.
func (h *ItemsHandler) UpdateItemShipping(ctx context.Context, req UpdateItemShippingRequest) error {
	payload, err := json.Marshal(struct {
		Shipping ItemShipping `json:"shipping"`
	}{
		Shipping: ItemShipping{
			Mode:         mercadoLibreShippingModeMercadoEnvios,
			FreeShipping: true,
		},
	})
	if err != nil {
		return fmt.Errorf("error encoding mercadolibre item shipping update request: %w", err)
	}
	return h.putItem(ctx, req.AccessToken, req.ExternalID, payload)
}

// putItem sends payload as the body of a PUT /items/{id} request, shared by
// UpdateItem and UpdateItemShipping.
func (h *ItemsHandler) putItem(ctx context.Context, accessToken, externalID string, payload []byte) error {
	log.Printf("mercadolibre items: update payload for %s: %s", externalID, payload)

	requestURL := fmt.Sprintf("%s/items/%s", h.client.baseURL, externalID)
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, requestURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("error creating mercadolibre item update request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := h.client.Do(request)
	if err != nil {
		return fmt.Errorf("error calling mercadolibre item update endpoint: %w", err)
	}
	defer response.Body.Close()

	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("error reading mercadolibre item update response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("mercadolibre items update failed: %w", parseItemAPIError(response.StatusCode, responsePayload))
	}

	// TEMPORARY diagnostic logging: a 200 here doesn't guarantee MercadoLibre
	// actually applied every field in the payload (e.g. free_shipping can be
	// silently ignored for business reasons like a price below their
	// minimum). Logging the full response body — normally discarded on the
	// success path — surfaces any "warnings" MELI included. Remove once the
	// free_shipping investigation is done.
	log.Printf("mercadolibre items: update response for %s (status %d): %s", externalID, response.StatusCode, responsePayload)

	return nil
}

// parseItemAPIError builds a detailed error from a non-2xx /items response:
// the top-level message plus every entry of MercadoLibre's "cause" array
// (which carries the actual field-level reason), and the raw response body
// as a fallback so nothing is ever silently lost.
func parseItemAPIError(statusCode int, body []byte) error {
	var errPayload itemAPIError
	_ = json.Unmarshal(body, &errPayload)

	detail := errPayload.Message
	if detail == "" {
		detail = errPayload.Error
	}
	if detail == "" {
		detail = "unknown error"
	}

	if len(errPayload.Cause) > 0 {
		causes := make([]string, 0, len(errPayload.Cause))
		for _, cause := range errPayload.Cause {
			switch {
			case cause.Attribute != "" && cause.Message != "":
				causes = append(causes, fmt.Sprintf("%s [%s] (attribute: %s)", cause.Message, cause.Code, cause.Attribute))
			case cause.Message != "":
				causes = append(causes, fmt.Sprintf("%s [%s]", cause.Message, cause.Code))
			default:
				causes = append(causes, cause.Code)
			}
		}
		detail = fmt.Sprintf("%s: %s", detail, strings.Join(causes, "; "))
	}

	return fmt.Errorf("(%d): %s — raw response: %s", statusCode, detail, body)
}

// maxItemsStatusBatch is MercadoLibre's limit on how many ids a single
// GET /items?ids=... multiget call accepts.
const maxItemsStatusBatch = 20

// ItemStatus is one item's current status, category, title and permalink as
// reported by MercadoLibre (e.g. status "active", "paused", "closed",
// "under_review"). SubStatus qualifies status further (e.g. an item can be
// "under_review" with sub_status "forbidden" — MercadoLibre's own docs note
// that one specifically never leaves review through the normal flow, only
// through exclusion — so callers that care about that distinction shouldn't
// rely on Status alone).
type ItemStatus struct {
	ID         string   `json:"id"`
	Status     string   `json:"status"`
	SubStatus  []string `json:"subStatus"`
	CategoryID string   `json:"categoryId"`
	Title      string   `json:"title"`
	Permalink  string   `json:"permalink"`
}

type multigetItemEntry struct {
	Code int `json:"code"`
	Body struct {
		ID         string   `json:"id"`
		Status     string   `json:"status"`
		SubStatus  []string `json:"sub_status"`
		CategoryID string   `json:"category_id"`
		Title      string   `json:"title"`
		Permalink  string   `json:"permalink"`
	} `json:"body"`
}

// GetItemsStatus fetches the current status of every id in externalIDs via
// GET /items?ids=..., batching requests in groups of up to
// maxItemsStatusBatch since that's the most MercadoLibre accepts per call.
// ids that MercadoLibre couldn't resolve (code != 200, e.g. deleted items)
// are silently omitted from the result rather than failing the whole batch.
func (h *ItemsHandler) GetItemsStatus(ctx context.Context, accessToken string, externalIDs []string) ([]ItemStatus, error) {
	statuses := make([]ItemStatus, 0, len(externalIDs))

	for start := 0; start < len(externalIDs); start += maxItemsStatusBatch {
		end := start + maxItemsStatusBatch
		if end > len(externalIDs) {
			end = len(externalIDs)
		}
		batch := externalIDs[start:end]

		requestURL := fmt.Sprintf("%s/items?ids=%s", h.client.baseURL, strings.Join(batch, ","))
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating mercadolibre items status request: %w", err)
		}
		request.Header.Set("Authorization", "Bearer "+accessToken)

		response, err := h.client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("error calling mercadolibre items status endpoint: %w", err)
		}

		responsePayload, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("error reading mercadolibre items status response: %w", err)
		}

		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("mercadolibre items status failed (%d): %s", response.StatusCode, responsePayload)
		}

		var entries []multigetItemEntry
		if err := json.Unmarshal(responsePayload, &entries); err != nil {
			return nil, fmt.Errorf("error decoding mercadolibre items status response: %w", err)
		}

		for _, entry := range entries {
			if entry.Code != http.StatusOK {
				log.Printf("mercadolibre items status: id=%s returned code=%d, skipping", entry.Body.ID, entry.Code)
				continue
			}
			statuses = append(statuses, ItemStatus{ID: entry.Body.ID, Status: entry.Body.Status, SubStatus: entry.Body.SubStatus, CategoryID: entry.Body.CategoryID, Title: entry.Body.Title, Permalink: entry.Body.Permalink})
		}
	}

	return statuses, nil
}

// ItemDetail is the subset of GET /items/{id} this integration reads to
// drive compatibility fixes: which category/domain the item belongs to
// (needed to resolve the vehicle domain via the compatibilities dump),
// whether it still carries the incomplete_compatibilities tag, and —
// crucially — UserProductID, which is non-empty when the item is linked to
// a MercadoLibre "User Product" (tag user_product_listing). Compatibilities
// for those items must be created via /user-products/{up_id}/compatibilities
// instead of /items/{id}/compatibilities; MercadoLibre rejects the latter
// for them with "This Item has User Product compatibilities. Use the
// corresponding User Product resources." (confirmed against a real item).
type ItemDetail struct {
	ID            string   `json:"id"`
	CategoryID    string   `json:"category_id"`
	DomainID      string   `json:"domain_id"`
	Tags          []string `json:"tags"`
	UserProductID string   `json:"user_product_id"`
}

// GetItem calls GET /items/{id} and returns the fields ItemDetail needs.
func (h *ItemsHandler) GetItem(ctx context.Context, accessToken, externalID string) (*ItemDetail, error) {
	requestURL := fmt.Sprintf("%s/items/%s", h.client.baseURL, externalID)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating mercadolibre item request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := h.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error calling mercadolibre item endpoint: %w", err)
	}
	defer response.Body.Close()

	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading mercadolibre item response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mercadolibre item fetch failed: %w", parseItemAPIError(response.StatusCode, responsePayload))
	}

	var item ItemDetail
	if err := json.Unmarshal(responsePayload, &item); err != nil {
		return nil, fmt.Errorf("error decoding mercadolibre item response: %w", err)
	}

	return &item, nil
}
