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
// attribute the seller must supply a value for; hidden/read_only mark ones
// MercadoLibre computes or fixes itself (e.g. a category with only one
// allowed ITEM_CONDITION value), which a seller-facing flow shouldn't try to
// set.
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
// (value_name) — true for "list" and "string_list" value_type.
func (attr CategoryRequiredAttribute) IsListType() bool {
	return attr.ValueType == "list" || attr.ValueType == "string_list"
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
