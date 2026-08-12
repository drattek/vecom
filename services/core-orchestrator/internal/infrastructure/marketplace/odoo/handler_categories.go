package odoo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Many2One decodes an Odoo many2one relation field, which the External
// JSON-2 API returns either as `false` (unset) or as an `[id, "display
// name"]` tuple.
type Many2One struct {
	ID    int64
	Valid bool
}

func (m *Many2One) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "false" || trimmed == "null" {
		m.Valid = false
		return nil
	}

	var tuple [2]json.RawMessage
	if err := json.Unmarshal(data, &tuple); err != nil {
		return err
	}
	if err := json.Unmarshal(tuple[0], &m.ID); err != nil {
		return err
	}

	m.Valid = true
	return nil
}

// PublicCategory mirrors product.public.category, Odoo's website/ecommerce
// category tree (distinct from the internal product.category tree used for
// costing).
type PublicCategory struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	ParentID Many2One `json:"parent_id"`
}

type CategoriesHandler struct {
	client *Client
}

func NewCategoriesHandler(client *Client) *CategoriesHandler {
	if client == nil {
		client = NewClient(nil, "", nil)
	}

	return &CategoriesHandler{client: client}
}

var publicCategorySearchReadFields = []string{"id", "name", "parent_id"}

type SearchReadPublicCategoriesRequest struct {
	Credentials Credentials
}

// SearchReadPublicCategories reads every product.public.category record, so
// the full ecommerce category hierarchy can be rebuilt locally from parent
// links.
func (h *CategoriesHandler) SearchReadPublicCategories(ctx context.Context, req SearchReadPublicCategoriesRequest) ([]PublicCategory, error) {
	params := map[string]any{
		"domain": []any{},
		"fields": publicCategorySearchReadFields,
	}

	var categories []PublicCategory
	if err := h.client.Call(ctx, req.Credentials, "product.public.category", "search_read", params, &categories); err != nil {
		return nil, err
	}

	return categories, nil
}

// CreateCategoryVals mirrors the product.public.category fields this
// integration sets when creating a new ecommerce category in Odoo: its name
// and, for anything below the top level, its parent (nil/omitted for a root
// category, matching how Odoo represents "no parent").
type CreateCategoryVals struct {
	Name     string `json:"name"`
	ParentID *int64 `json:"parent_id,omitempty"`
}

type CreateCategoryRequest struct {
	Credentials Credentials
	Vals        CreateCategoryVals
}

// CreateCategory calls product.public.category/create with the given vals
// wrapped in vals_list (Odoo's External JSON-2 create always takes a list,
// even for a single record) and returns the new product.public.category id.
func (h *CategoriesHandler) CreateCategory(ctx context.Context, req CreateCategoryRequest) (int64, error) {
	params := map[string]any{
		"vals_list": []CreateCategoryVals{req.Vals},
	}

	var ids []int64
	if err := h.client.Call(ctx, req.Credentials, "product.public.category", "create", params, &ids); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, fmt.Errorf("odoo product.public.category.create returned no id")
	}

	return ids[0], nil
}
