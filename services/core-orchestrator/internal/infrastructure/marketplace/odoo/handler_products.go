package odoo

import (
	"context"
	"encoding/json"
	"fmt"
)

type ProductsHandler struct {
	client *Client
}

func NewProductsHandler(client *Client) *ProductsHandler {
	if client == nil {
		client = NewClient(nil, "", nil)
	}

	return &ProductsHandler{client: client}
}

// FalseableString decodes an Odoo char/computed field that comes back as
// JSON `false` (Odoo's convention for "not set") instead of an empty string.
type FalseableString string

func (s *FalseableString) UnmarshalJSON(data []byte) error {
	if string(data) == "false" || string(data) == "null" {
		*s = ""
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	*s = FalseableString(value)
	return nil
}

// Product mirrors the product.template fields needed to list an item on a
// marketplace: identity, pricing, stock, attributes and taxes.
type Product struct {
	ID               int64           `json:"id"`
	Name             string          `json:"name"`
	DefaultCode      FalseableString `json:"default_code"`
	ListPrice        float64         `json:"list_price"`
	QtyAvailable     float64         `json:"qty_available"`
	AttributeLineIDs []int64         `json:"attribute_line_ids"`
	ShowAvailability bool            `json:"show_availability"`
	TaxesID          []int64         `json:"taxes_id"`
	TaxString        FalseableString `json:"tax_string"`
	// PublicCategIDs are the product.public.category (ecommerce category) ids
	// assigned to this product.template. Read by the vecom_sync_product
	// migration to resolve a legacy Odoo listing's category into the local
	// ecom_categories hierarchy.
	PublicCategIDs []int64 `json:"public_categ_ids"`
}

var productSearchReadFields = []string{
	"id",
	"name",
	"default_code",
	"list_price",
	"qty_available",
	"attribute_line_ids",
	"show_availability",
	"taxes_id",
	"tax_string",
	"public_categ_ids",
}

const defaultProductSearchReadLimit = 20

type SearchReadProductsRequest struct {
	Credentials Credentials
	// IDs restricts the search to specific product.template ids. When empty,
	// every sellable, published product is eligible (subject to Limit).
	IDs   []int64
	Limit int
}

// SearchReadProducts reads product.template records that are sellable
// (sale_ok) and published to the website (website_published). A successful
// call also proves odoo_url, ApiKey and (when required) X-Odoo-Database are
// all valid for the connection.
func (h *ProductsHandler) SearchReadProducts(ctx context.Context, req SearchReadProductsRequest) ([]Product, error) {
	domain := []any{
		[]any{"sale_ok", "=", true},
		[]any{"website_published", "=", true},
	}

	if len(req.IDs) > 0 {
		ids := make([]any, len(req.IDs))
		for i, id := range req.IDs {
			ids[i] = id
		}
		domain = append(domain, []any{"id", "in", ids})
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultProductSearchReadLimit
	}

	params := map[string]any{
		"domain": domain,
		"fields": productSearchReadFields,
		"limit":  limit,
	}

	var products []Product
	if err := h.client.Call(ctx, req.Credentials, "product.template", "search_read", params, &products); err != nil {
		return nil, err
	}

	return products, nil
}

// GetProductByID reads a single product.template by id with no sale_ok /
// website_published filter — unlike SearchReadProducts, which only returns
// sellable, published products. Used by the vecom_sync_product migration,
// where a legacy listing may point at a product.template that is no longer
// published. Returns nil when Odoo has no product.template with that id.
func (h *ProductsHandler) GetProductByID(ctx context.Context, credentials Credentials, id int64) (*Product, error) {
	params := map[string]any{
		"domain": []any{[]any{"id", "=", id}},
		"fields": productSearchReadFields,
		"limit":  1,
	}

	var products []Product
	if err := h.client.Call(ctx, credentials, "product.template", "search_read", params, &products); err != nil {
		return nil, err
	}
	if len(products) == 0 {
		return nil, nil
	}

	return &products[0], nil
}

// X2ManyReplace builds Odoo's "replace all" command for an x2many field
// (e.g. public_categ_ids, taxes_id) restricted to a single id, in the shape
// External JSON-2's create/write expect: [[6, 0, [id]]].
func X2ManyReplace(id int64) [][]any {
	return [][]any{{6, 0, []int64{id}}}
}

// CreateProductVals mirrors the product.template fields this integration
// sets when creating a new product in Odoo. public_categ_ids and taxes_id
// are restricted to a single value each (this integration assigns exactly
// one ecommerce category and one tax per product).
type CreateProductVals struct {
	Name                 string  `json:"name"`
	Type                 string  `json:"type"`
	ListPrice            float64 `json:"list_price"`
	SaleOK               bool    `json:"sale_ok"`
	IsPublished          bool    `json:"is_published"`
	PurchaseOK           bool    `json:"purchase_ok"`
	Image1920            string  `json:"image_1920,omitempty"`
	DefaultCode          string  `json:"default_code,omitempty"`
	ShowAvailability     bool    `json:"show_availability"`
	IsStorable           bool    `json:"is_storable"`
	AllowOutOfStockOrder bool    `json:"allow_out_of_stock_order"`
	PublicCategIDs       [][]any `json:"public_categ_ids"`
	DescriptionEcommerce string  `json:"description_ecommerce"`
	TaxesID              [][]any `json:"taxes_id"`
	QtyAvailable         float64 `json:"qty_available"`
	Weight               float64 `json:"weight"`
	Volume               float64 `json:"volume"`
}

type CreateProductRequest struct {
	Credentials Credentials
	Vals        CreateProductVals
}

// CreateProduct calls product.template/create with the given vals wrapped
// in vals_list (Odoo's External JSON-2 create always takes a list, even
// for a single record) and returns the new product.template id.
func (h *ProductsHandler) CreateProduct(ctx context.Context, req CreateProductRequest) (int64, error) {
	params := map[string]any{
		"vals_list": []CreateProductVals{req.Vals},
	}

	var ids []int64
	if err := h.client.Call(ctx, req.Credentials, "product.template", "create", params, &ids); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, fmt.Errorf("odoo product.template.create returned no id")
	}

	return ids[0], nil
}

// UpdateProductVals mirrors the subset of product.template fields this
// integration refreshes on an already-synced product: stock, price, name
// (always sent — not omitempty — so a product's Odoo listing name stays
// in lockstep with ecom_products/its part number rather than only updating
// it when it happens to differ), weight/volume so a later
// ecom_product_dimensions correction also reaches products already synced
// to Odoo, not just new ones, and PublicCategIDs (omitempty: left unset,
// leaving the Odoo category untouched, when the caller has no local category
// to resolve one from — see OdooProductSyncService.update).
type UpdateProductVals struct {
	QtyAvailable   float64 `json:"qty_available"`
	ListPrice      float64 `json:"list_price"`
	Name           string  `json:"name"`
	Weight         float64 `json:"weight"`
	Volume         float64 `json:"volume"`
	PublicCategIDs [][]any `json:"public_categ_ids,omitempty"`
}

type UpdateProductRequest struct {
	Credentials Credentials
	ExternalID  int64
	Vals        UpdateProductVals
}

// UpdateProduct calls product.template/write to refresh an already-synced
// product's qty_available/list_price.
func (h *ProductsHandler) UpdateProduct(ctx context.Context, req UpdateProductRequest) error {
	params := map[string]any{
		"ids":  []int64{req.ExternalID},
		"vals": req.Vals,
	}

	return h.client.Call(ctx, req.Credentials, "product.template", "write", params, nil)
}

// UpdateProductPriceStockVals is the payload for a price/stock-only refresh
// (see UpdateProductPriceStock) — deliberately just these two fields, unlike
// UpdateProductVals, so a routine refresh never touches name/weight/volume/
// category already curated in Odoo.
type UpdateProductPriceStockVals struct {
	QtyAvailable float64 `json:"qty_available"`
	ListPrice    float64 `json:"list_price"`
}

type UpdateProductPriceStockRequest struct {
	Credentials Credentials
	ExternalID  int64
	Vals        UpdateProductPriceStockVals
}

// UpdateProductPriceStock calls product.template/write with only
// qty_available/list_price — used by OdooProductSyncService.Refresh
// (channel_listings.RefreshListings), which must never send anything else.
// UpdateProduct above remains the one used by Sync/update for a full
// refresh (name/weight/volume/category included).
func (h *ProductsHandler) UpdateProductPriceStock(ctx context.Context, req UpdateProductPriceStockRequest) error {
	params := map[string]any{
		"ids":  []int64{req.ExternalID},
		"vals": req.Vals,
	}

	return h.client.Call(ctx, req.Credentials, "product.template", "write", params, nil)
}
