package mercadolibre

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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

// MaxItemsBatchSize is MercadoLibre's limit on how many ids a single
// GET /items?ids=... multiget call accepts. Exported so a caller that needs
// to control pagination itself (e.g. sync.MercadoLibreListingsAuditService,
// which re-checks token freshness between batches on a long account-wide
// scan) can chunk consistently with what this package actually sends.
const MaxItemsBatchSize = 20

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
// MaxItemsBatchSize since that's the most MercadoLibre accepts per call.
// ids that MercadoLibre couldn't resolve (code != 200, e.g. deleted items)
// are silently omitted from the result rather than failing the whole batch.
func (h *ItemsHandler) GetItemsStatus(ctx context.Context, accessToken string, externalIDs []string) ([]ItemStatus, error) {
	statuses := make([]ItemStatus, 0, len(externalIDs))

	for start := 0; start < len(externalIDs); start += MaxItemsBatchSize {
		end := start + MaxItemsBatchSize
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
// Title, Price, AvailableQuantity, Attributes and SellerCustomField are read
// by sync.MercadoLibreListingsAuditService (the account-wide flagged-
// listings scan): Price/AvailableQuantity to decide whether a listing
// matches the audit's filter, Attributes/CategoryID to mirror its data into
// ecom_products' local attribute/category tables, Title to derive the
// product's name (see SELLER_SKU within Attributes and SellerCustomField for
// how that flow resolves a SKU even for a listing with no local
// ecom_channel_product_map row).
type ItemDetail struct {
	ID                string          `json:"id"`
	Title             string          `json:"title"`
	CategoryID        string          `json:"category_id"`
	DomainID          string          `json:"domain_id"`
	Price             float64         `json:"price"`
	AvailableQuantity int             `json:"available_quantity"`
	Tags              []string        `json:"tags"`
	UserProductID     string          `json:"user_product_id"`
	Attributes        []ItemAttribute `json:"attributes"`
	SellerCustomField string          `json:"seller_custom_field"`
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

// SellerItemsScanPage is one page of
// GET /users/{sellerID}/items/search?search_type=scan: the listing ids on
// this page, plus the scroll_id to pass into the next call. An empty
// ScrollID or an empty Results means the scan is finished.
type SellerItemsScanPage struct {
	ScrollID string   `json:"scroll_id"`
	Results  []string `json:"results"`
}

// ScanSellerItems fetches one page of every listing id sellerID owns, any
// status, via GET /users/{sellerID}/items/search?search_type=scan —
// MercadoLibre's cursor-based ("scan") pagination mode. Unlike the classic
// offset+limit mode (which MercadoLibre caps at 1000 total results
// regardless of how many listings actually exist), scan mode has no such
// cap — required here since an account can have (and this integration has
// seen) well over 1000 listings. Pass scrollID="" for the first call, then
// keep passing back the ScrollID the previous page returned until a page
// comes back with no results — see
// sync.MercadoLibreListingsAuditService.listAllSellerItemIDs.
func (h *ItemsHandler) ScanSellerItems(ctx context.Context, accessToken string, sellerID int64, scrollID string) (*SellerItemsScanPage, error) {
	requestURL := fmt.Sprintf("%s/users/%d/items/search?search_type=scan", h.client.baseURL, sellerID)
	if scrollID != "" {
		requestURL += "&scroll_id=" + url.QueryEscape(scrollID)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating mercadolibre seller items scan request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := h.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error calling mercadolibre seller items scan endpoint: %w", err)
	}
	defer response.Body.Close()

	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading mercadolibre seller items scan response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mercadolibre seller items scan failed: %w", parseItemAPIError(response.StatusCode, responsePayload))
	}

	var page SellerItemsScanPage
	if err := json.Unmarshal(responsePayload, &page); err != nil {
		return nil, fmt.Errorf("error decoding mercadolibre seller items scan response: %w", err)
	}

	return &page, nil
}

// ItemSummary is the minimal GET /items?ids=...&attributes=id,price,available_quantity
// projection used for a first, cheap pass over every listing in the account
// — see GetItemsSummary — before fetching full detail only for the ones
// that actually match the audit's price/stock filter.
type ItemSummary struct {
	ID                string  `json:"id"`
	Price             float64 `json:"price"`
	AvailableQuantity int     `json:"available_quantity"`
}

type multigetItemSummaryEntry struct {
	Code int         `json:"code"`
	Body ItemSummary `json:"body"`
}

// GetItemsSummary fetches just id/price/available_quantity for a single
// batch of up to MaxItemsBatchSize ids via one GET /items?ids=... call,
// restricting the response to those three fields (the "attributes" query
// param here selects which fields the multiget returns, unrelated to an
// item's own custom attributes) to keep an account-wide scan cheap. Callers
// with more than MaxItemsBatchSize ids must chunk and call this once per
// batch themselves — see sync.MercadoLibreListingsAuditService, which
// deliberately re-validates token freshness between batches on a
// potentially long account-wide scan rather than trusting one token to
// outlive the whole run. ids MercadoLibre couldn't resolve (code != 200) are
// silently omitted, same as GetItemsStatus.
func (h *ItemsHandler) GetItemsSummary(ctx context.Context, accessToken string, externalIDs []string) ([]ItemSummary, error) {
	requestURL := fmt.Sprintf("%s/items?ids=%s&attributes=id,price,available_quantity", h.client.baseURL, strings.Join(externalIDs, ","))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating mercadolibre items summary request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := h.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error calling mercadolibre items summary endpoint: %w", err)
	}
	defer response.Body.Close()

	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading mercadolibre items summary response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mercadolibre items summary failed (%d): %s", response.StatusCode, responsePayload)
	}

	var entries []multigetItemSummaryEntry
	if err := json.Unmarshal(responsePayload, &entries); err != nil {
		return nil, fmt.Errorf("error decoding mercadolibre items summary response: %w", err)
	}

	summaries := make([]ItemSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.Code != http.StatusOK {
			continue
		}
		summaries = append(summaries, entry.Body)
	}

	return summaries, nil
}

// mercadoLibreItemDetailFields restricts GetItemsDetail's multiget response
// to exactly what sync.MercadoLibreListingsAuditService needs — avoids
// pulling every other (often heavy) item field, e.g. description or
// variations, for a batch call.
const mercadoLibreItemDetailFields = "id,title,price,available_quantity,category_id,domain_id,tags,user_product_id,attributes,seller_custom_field"

type multigetItemDetailEntry struct {
	Code int        `json:"code"`
	Body ItemDetail `json:"body"`
}

// GetItemsDetail fetches full ItemDetail for a single batch of up to
// MaxItemsBatchSize ids via one GET /items?ids=... call — meant to be called
// only for the (usually much smaller) subset of ids GetItemsSummary already
// flagged as matching, not the whole account. Callers with more than
// MaxItemsBatchSize matched ids must chunk and call this once per batch
// themselves, same contract as GetItemsSummary. ids MercadoLibre couldn't
// resolve (code != 200) are silently omitted, same as GetItemsStatus.
func (h *ItemsHandler) GetItemsDetail(ctx context.Context, accessToken string, externalIDs []string) ([]ItemDetail, error) {
	requestURL := fmt.Sprintf("%s/items?ids=%s&attributes=%s", h.client.baseURL, strings.Join(externalIDs, ","), mercadoLibreItemDetailFields)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating mercadolibre items detail request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := h.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error calling mercadolibre items detail endpoint: %w", err)
	}
	defer response.Body.Close()

	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading mercadolibre items detail response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mercadolibre items detail failed (%d): %s", response.StatusCode, responsePayload)
	}

	var entries []multigetItemDetailEntry
	if err := json.Unmarshal(responsePayload, &entries); err != nil {
		return nil, fmt.Errorf("error decoding mercadolibre items detail response: %w", err)
	}

	details := make([]ItemDetail, 0, len(entries))
	for _, entry := range entries {
		if entry.Code != http.StatusOK {
			log.Printf("mercadolibre items detail: id=%s returned code=%d, skipping", entry.Body.ID, entry.Code)
			continue
		}
		details = append(details, entry.Body)
	}

	return details, nil
}
