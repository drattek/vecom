package mercadolibre

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type CategoriesHandler struct {
	client *Client
}

type CategoryPredictorRequest struct {
	AccessToken string
	SiteID      string
	Query       string
	Limit       int
}

type CategoryAttribute struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ValueID   string `json:"value_id"`
	ValueName string `json:"value_name"`
}

type CategoryPrediction struct {
	DomainID     string              `json:"domain_id"`
	DomainName   string              `json:"domain_name"`
	CategoryID   string              `json:"category_id"`
	CategoryName string              `json:"category_name"`
	Attributes   []CategoryAttribute `json:"attributes"`
}

type apiError struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

func NewCategoriesHandler(client *Client) *CategoriesHandler {
	if client == nil {
		client = NewClient(nil, "", nil)
	}

	return &CategoriesHandler{client: client}
}

func (h *CategoriesHandler) PredictCategory(ctx context.Context, req CategoryPredictorRequest) ([]CategoryPrediction, error) {
	params := url.Values{}
	params.Set("q", req.Query)
	params.Set("limit", strconv.Itoa(req.Limit))

	requestURL := fmt.Sprintf("%s/sites/%s/domain_discovery/search?%s", h.client.baseURL, url.PathEscape(req.SiteID), params.Encode())
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating mercadolibre categories request: %w", err)
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+req.AccessToken)

	response, err := h.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error calling mercadolibre categories endpoint: %w", err)
	}
	defer response.Body.Close()

	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading mercadolibre categories response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		var errPayload apiError
		if json.Unmarshal(responsePayload, &errPayload) == nil {
			if errPayload.Message != "" {
				return nil, fmt.Errorf("mercadolibre categories failed (%d): %s", response.StatusCode, errPayload.Message)
			}
			if errPayload.Error != "" {
				return nil, fmt.Errorf("mercadolibre categories failed (%d): %s", response.StatusCode, errPayload.Error)
			}
		}

		return nil, fmt.Errorf("mercadolibre categories failed with status %d", response.StatusCode)
	}

	var predictions []CategoryPrediction
	if err := json.Unmarshal(responsePayload, &predictions); err != nil {
		return nil, fmt.Errorf("error decoding mercadolibre categories response: %w", err)
	}

	return predictions, nil
}

// CategoryPathNode is one entry of a category's path_from_root: its id and
// name, ordered from the top-level root category down to (and including)
// the category itself.
type CategoryPathNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CategoryDetails is the subset of GET /categories/{id}'s response this
// integration needs: the category's own id/name plus its full ancestor
// chain, used to replicate MercadoLibre's category hierarchy locally.
type CategoryDetails struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	PathFromRoot []CategoryPathNode `json:"path_from_root"`
}

// CategoryAttributeTags is the subset of GET /categories/{id}/attributes'
// per-attribute "tags" object this integration reads: required marks an
// attribute the seller must supply a value for; read_only marks one
// MercadoLibre computes or fixes itself (e.g. a category with only one
// allowed ITEM_CONDITION value), which a seller-facing flow shouldn't try to
// set. hidden means only "not shown in MercadoLibre's simplified publish
// form" — confirmed against a real category where SELLER_PACKAGE_* come back
// hidden=true, read_only=false, and are still genuinely seller-settable — so
// callers must not treat Hidden as equivalent to ReadOnly (see
// channel_attribute_values.Service.ProvisionCategoryAttributes).
type CategoryAttributeTags struct {
	Required bool `json:"required"`
	Hidden   bool `json:"hidden"`
	ReadOnly bool `json:"read_only"`
}

// CategoryAttributeValue is one entry of a CategoryRequiredAttribute's fixed
// "values" catalogue (only present when ValueType is a closed list, e.g.
// "list"/"string_list") — MercadoLibre's own id/name pair for that option.
// This id is what must travel as an item attribute's value_id (see
// mercadolibre.ItemAttribute), not the name.
type CategoryAttributeValue struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CategoryRequiredAttribute is one entry of GET /categories/{id}/attributes:
// value_type distinguishes how the attribute's value is meant to be typed
// ("string", "number", "boolean", "list", "string_list", "number_unit", ...);
// unlike CategoryAttribute (domain_discovery's per-prediction attribute
// preview), this carries the tags needed to tell a mandatory attribute from
// an optional or seller-unsettable one, plus (for list-typed attributes) the
// closed set of values MercadoLibre actually allows.
type CategoryRequiredAttribute struct {
	ID        string                   `json:"id"`
	Name      string                   `json:"name"`
	ValueType string                   `json:"value_type"`
	Tags      CategoryAttributeTags    `json:"tags"`
	Values    []CategoryAttributeValue `json:"values,omitempty"`
}

// IsListType reports whether attr's value must be sent as one of a fixed,
// MercadoLibre-defined set of options (value_id) rather than free text
// (value_name): always for "list"/"string_list" value_type, and for a plain
// "string" value_type that both ships a closed "values" catalogue AND is
// marked required. MercadoLibre does expose enum-like attributes as
// "string"+values (e.g. SIDE — "Lado": Delantero/Trasero — in some autopart
// categories), and a required one must travel as a value_id from that list.
// The required guard is deliberate: the API also exposes many *optional*
// "string"+values attributes (COLOR's ~50 colours, ORIGIN, the SAT tax keys)
// whose whole option list would otherwise be mirrored into ecom_attribute_options
// on every publish just to sit unused — those stay free text. "boolean"/
// "number" value_types keep their own handling even with a values array
// (e.g. IS_KIT).
func (attr CategoryRequiredAttribute) IsListType() bool {
	switch attr.ValueType {
	case "list", "string_list":
		return true
	case "string":
		return len(attr.Values) > 0 && attr.Tags.Required
	default:
		return false
	}
}

// GetCategoryAttributes calls MercadoLibre's public (no access token
// required) GET /categories/{categoryID}/attributes endpoint to read every
// attribute the category defines, including which ones are mandatory
// (Tags.Required) for a listing under it.
func (h *CategoriesHandler) GetCategoryAttributes(ctx context.Context, categoryID string) ([]CategoryRequiredAttribute, error) {
	requestURL := fmt.Sprintf("%s/categories/%s/attributes", h.client.baseURL, url.PathEscape(categoryID))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating mercadolibre category attributes request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	response, err := h.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error calling mercadolibre category attributes endpoint: %w", err)
	}
	defer response.Body.Close()

	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading mercadolibre category attributes response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		var errPayload apiError
		if json.Unmarshal(responsePayload, &errPayload) == nil {
			if errPayload.Message != "" {
				return nil, fmt.Errorf("mercadolibre category %s attributes failed (%d): %s", categoryID, response.StatusCode, errPayload.Message)
			}
			if errPayload.Error != "" {
				return nil, fmt.Errorf("mercadolibre category %s attributes failed (%d): %s", categoryID, response.StatusCode, errPayload.Error)
			}
		}

		return nil, fmt.Errorf("mercadolibre category %s attributes failed with status %d", categoryID, response.StatusCode)
	}

	var attributes []CategoryRequiredAttribute
	if err := json.Unmarshal(responsePayload, &attributes); err != nil {
		return nil, fmt.Errorf("error decoding mercadolibre category attributes response: %w", err)
	}

	return attributes, nil
}

// GetCategory calls MercadoLibre's public (no access token required)
// GET /categories/{categoryID} endpoint to read a category's full
// path_from_root — PredictCategory only ever returns the leaf category, not
// its ancestors.
func (h *CategoriesHandler) GetCategory(ctx context.Context, categoryID string) (*CategoryDetails, error) {
	requestURL := fmt.Sprintf("%s/categories/%s", h.client.baseURL, url.PathEscape(categoryID))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating mercadolibre category request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	response, err := h.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error calling mercadolibre category endpoint: %w", err)
	}
	defer response.Body.Close()

	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading mercadolibre category response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		var errPayload apiError
		if json.Unmarshal(responsePayload, &errPayload) == nil {
			if errPayload.Message != "" {
				return nil, fmt.Errorf("mercadolibre category %s failed (%d): %s", categoryID, response.StatusCode, errPayload.Message)
			}
			if errPayload.Error != "" {
				return nil, fmt.Errorf("mercadolibre category %s failed (%d): %s", categoryID, response.StatusCode, errPayload.Error)
			}
		}

		return nil, fmt.Errorf("mercadolibre category %s failed with status %d", categoryID, response.StatusCode)
	}

	var details CategoryDetails
	if err := json.Unmarshal(responsePayload, &details); err != nil {
		return nil, fmt.Errorf("error decoding mercadolibre category response: %w", err)
	}

	return &details, nil
}

// GetSiteCategoriesRaw calls MercadoLibre's GET /sites/{siteID}/categories
// endpoint — the site's top-level (root) category list — and returns the
// response body exactly as MercadoLibre sent it. Despite being documented as
// public, it 403s ({"error":"...","message":"At least one policy returned
// UNAUTHORIZED."}) without an Authorization header — confirmed against the
// real API, same as GetDomainCompatibilities in handler_compatibilities.go —
// so accessToken is required here too.
func (h *CategoriesHandler) GetSiteCategoriesRaw(ctx context.Context, accessToken, siteID string) (json.RawMessage, error) {
	requestURL := fmt.Sprintf("%s/sites/%s/categories", h.client.baseURL, url.PathEscape(siteID))
	return h.getRaw(ctx, accessToken, requestURL, fmt.Sprintf("mercadolibre site %s categories", siteID))
}

// GetCategoryRaw calls MercadoLibre's GET /categories/{categoryID} endpoint
// and returns the response body exactly as MercadoLibre sent it — unlike
// GetCategory's narrower CategoryDetails, this keeps every field including
// children_categories, attribute_types and settings. accessToken is sent the
// same way GetSiteCategoriesRaw does, since MercadoLibre's public/no-token
// documentation for these endpoints has already proven unreliable in
// practice (see GetSiteCategoriesRaw).
func (h *CategoriesHandler) GetCategoryRaw(ctx context.Context, accessToken, categoryID string) (json.RawMessage, error) {
	requestURL := fmt.Sprintf("%s/categories/%s", h.client.baseURL, url.PathEscape(categoryID))
	return h.getRaw(ctx, accessToken, requestURL, fmt.Sprintf("mercadolibre category %s", categoryID))
}

// getRaw issues a GET against requestURL and returns the raw JSON response
// body — shared by GetSiteCategoriesRaw/GetCategoryRaw so both report errors
// the same way GetCategory/GetCategoryAttributes above do.
func (h *CategoriesHandler) getRaw(ctx context.Context, accessToken, requestURL, label string) (json.RawMessage, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating %s request: %w", label, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := h.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error calling %s endpoint: %w", label, err)
	}
	defer response.Body.Close()

	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading %s response: %w", label, err)
	}

	if response.StatusCode != http.StatusOK {
		var errPayload apiError
		if json.Unmarshal(responsePayload, &errPayload) == nil {
			if errPayload.Message != "" {
				return nil, fmt.Errorf("%s failed (%d): %s", label, response.StatusCode, errPayload.Message)
			}
			if errPayload.Error != "" {
				return nil, fmt.Errorf("%s failed (%d): %s", label, response.StatusCode, errPayload.Error)
			}
		}
		return nil, fmt.Errorf("%s failed with status %d", label, response.StatusCode)
	}

	return json.RawMessage(responsePayload), nil
}
