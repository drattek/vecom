package mercadolibre

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type CompatibilitiesHandler struct {
	client *Client
}

func NewCompatibilitiesHandler(client *Client) *CompatibilitiesHandler {
	if client == nil {
		client = NewClient(nil, "", nil)
	}

	return &CompatibilitiesHandler{client: client}
}

// DomainCategoryEntry is one autopart category's compatibility settings
// within a DomainCompatibility block: whether reporting compatibilities is
// required for it, whether it supports position/side restrictions
// (RestrictionsStatus "ENABLED"/"DISABLED") and whether restrictions are
// themselves mandatory.
type DomainCategoryEntry struct {
	ID                   string `json:"id"`
	Required             bool   `json:"required"`
	NoteStatus           string `json:"note_status"`
	RestrictionsStatus   string `json:"restrictions_status"`
	UniversalStatus      string `json:"universal_status"`
	RestrictionsRequired bool   `json:"restrictions_required"`
}

// DomainCompatibility describes one autopart domain's compatibility link to
// a vehicle domain (CompatibleDomainID) — only entries with Type "EXTENSION"
// actually support reporting compatibilities, per MercadoLibre's own docs.
type DomainCompatibility struct {
	CompatibleDomainID string                `json:"compatible_domain_id"`
	Type               string                `json:"type"`
	Categories         []DomainCategoryEntry `json:"categories"`
}

// DomainDumpEntry is one entry of the site-wide compatibility dump: an
// autopart domain (e.g. "MLM-VEHICLE_BRAKE_PADS") and which vehicle
// domain(s) it can report compatibilities against.
type DomainDumpEntry struct {
	DomainID        string                `json:"domain_id"`
	Main            bool                  `json:"main"`
	Compatibilities []DomainCompatibility `json:"compatibilities"`
}

// GetDomainCompatibilities calls GET /catalog/dumps/domains/{siteID}/compatibilities,
// the site-wide dump of which autopart domains support/require vehicle
// compatibilities and which vehicle domain each one reports against. It's
// the same dump regardless of which item/category is being fixed, so
// callers processing a batch should fetch it once and reuse the result
// across every item rather than calling this per item. The response shape
// is inconsistent across MercadoLibre's own docs (flat array vs an array
// wrapping one array), so both are tried.
//
// Despite MercadoLibre's own example curl for this endpoint omitting an
// Authorization header, it 403s ({"blocked_by":"PolicyAgent","code":
// "PA_UNAUTHORIZED_RESULT_FROM_POLICIES"}) without one — confirmed against
// the real API — so accessToken is required here same as every other call.
func (h *CompatibilitiesHandler) GetDomainCompatibilities(ctx context.Context, accessToken, siteID string) ([]DomainDumpEntry, error) {
	requestURL := fmt.Sprintf("%s/catalog/dumps/domains/%s/compatibilities", h.client.baseURL, siteID)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating mercadolibre domain compatibilities request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := h.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error calling mercadolibre domain compatibilities endpoint: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading mercadolibre domain compatibilities response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mercadolibre domain compatibilities fetch failed: %w", parseItemAPIError(response.StatusCode, body))
	}

	var flat []DomainDumpEntry
	if err := json.Unmarshal(body, &flat); err == nil && len(flat) > 0 {
		return flat, nil
	}

	var nested [][]DomainDumpEntry
	if err := json.Unmarshal(body, &nested); err == nil {
		flattened := make([]DomainDumpEntry, 0, len(nested))
		for _, group := range nested {
			flattened = append(flattened, group...)
		}
		return flattened, nil
	}

	return nil, fmt.Errorf("could not decode mercadolibre domain compatibilities response (tried flat and nested array shapes)")
}

// CreateCompatibilitiesResponse mirrors the shape MercadoLibre returns on a
// successful POST to either /items/{id}/compatibilities or
// /user-products/{id}/compatibilities.
type CreateCompatibilitiesResponse struct {
	CreatedCompatibilitiesCount int    `json:"created_compatibilities_count"`
	Message                     string `json:"message"`
	Error                       string `json:"error"`
}

// CreateItemCompatibilities calls POST /items/{externalID}/compatibilities.
// MercadoLibre rejects this for items linked to a "User Product" (see
// ItemDetail.UserProductID) — use CreateUserProductCompatibilities for
// those instead. payload is built by the caller (application layer), not
// here, keeping this package a thin transport over the documented wire
// format.
func (h *CompatibilitiesHandler) CreateItemCompatibilities(ctx context.Context, accessToken, externalID string, payload map[string]any) (*CreateCompatibilitiesResponse, error) {
	requestURL := fmt.Sprintf("%s/items/%s/compatibilities", h.client.baseURL, externalID)
	return h.postCompatibilities(ctx, accessToken, requestURL, payload)
}

// CreateUserProductCompatibilities calls
// POST /user-products/{userProductID}/compatibilities — required instead of
// CreateItemCompatibilities whenever the item carries a non-empty
// ItemDetail.UserProductID. payload must carry domain_id/category_id at its
// root (outside products_families), per MercadoLibre's documented shape for
// this endpoint.
func (h *CompatibilitiesHandler) CreateUserProductCompatibilities(ctx context.Context, accessToken, userProductID string, payload map[string]any) (*CreateCompatibilitiesResponse, error) {
	requestURL := fmt.Sprintf("%s/user-products/%s/compatibilities", h.client.baseURL, userProductID)
	return h.postCompatibilities(ctx, accessToken, requestURL, payload)
}

func (h *CompatibilitiesHandler) postCompatibilities(ctx context.Context, accessToken, requestURL string, payload map[string]any) (*CreateCompatibilitiesResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error encoding mercadolibre compatibilities request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("error creating mercadolibre compatibilities request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := h.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error calling mercadolibre compatibilities endpoint: %w", err)
	}
	defer response.Body.Close()

	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading mercadolibre compatibilities response: %w", err)
	}

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("mercadolibre compatibilities create failed: %w", parseItemAPIError(response.StatusCode, responsePayload))
	}

	var result CreateCompatibilitiesResponse
	if err := json.Unmarshal(responsePayload, &result); err != nil {
		return nil, fmt.Errorf("error decoding mercadolibre compatibilities response: %w", err)
	}

	return &result, nil
}
