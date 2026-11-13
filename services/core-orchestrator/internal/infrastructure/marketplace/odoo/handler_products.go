package odoo

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
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
	// VgSKU is website_vegusa's vg_sku (the product's SKU, distinct from
	// default_code which carries the part number).
	VgSKU                string  `json:"vg_sku,omitempty"`
	// VgShortDescription is website_vegusa's vg_short_description (max 255 chars).
	VgShortDescription   string  `json:"vg_short_description,omitempty"`
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
// to resolve one from — see OdooProductSyncService.update). Image1920 (the
// cover photo) and DescriptionEcommerce are only populated by a full resync
// (OdooProductSyncService.Resync) — Image1920 stays omitempty since a
// failed re-download must never blank out an existing cover, while
// DescriptionEcommerce is sent as-is (including empty) so clearing
// ecom_products.description actually clears it in Odoo too.
type UpdateProductVals struct {
	QtyAvailable         float64 `json:"qty_available"`
	ListPrice            float64 `json:"list_price"`
	Name                 string  `json:"name"`
	Weight               float64 `json:"weight"`
	Volume               float64 `json:"volume"`
	PublicCategIDs       [][]any `json:"public_categ_ids,omitempty"`
	Image1920            string  `json:"image_1920,omitempty"`
	DescriptionEcommerce string  `json:"description_ecommerce"`
	// VgSKU is website_vegusa's vg_sku; omitted when the product has no SKU.
	VgSKU string `json:"vg_sku,omitempty"`
	// VgShortDescription is website_vegusa's vg_short_description (max 255
	// chars); omitted when empty, so it never blanks an existing one.
	VgShortDescription string `json:"vg_short_description,omitempty"`
	// IsPublished (website.published.mixin's is_published) makes the product
	// visible on the Odoo storefront. A pointer + omitempty: nil leaves the
	// current state untouched; a full resync sets it to true only when the
	// product is ready (cover image, stock and price) — it never unpublishes.
	IsPublished *bool `json:"is_published,omitempty"`
}

type UpdateProductRequest struct {
	Credentials Credentials
	ExternalID  int64
	Vals        UpdateProductVals
}

// UpdateProduct calls product.template/write to refresh an already-synced
// product's qty_available/list_price.
func (h *ProductsHandler) UpdateProduct(ctx context.Context, req UpdateProductRequest) error {
	vals, err := h.valsWithoutTrackedStock(ctx, req.Credentials, req.ExternalID, req.Vals)
	if err != nil {
		return err
	}

	params := map[string]any{
		"ids":  []int64{req.ExternalID},
		"vals": vals,
	}

	return h.client.Call(ctx, req.Credentials, "product.template", "write", params, nil)
}

// UpdateProductPriceStockVals is the payload for a price/stock-only refresh
// (see UpdateProductPriceStock) — plus a narrow opt-in sale_ok flip for the
// sync_zero_stock_as_paused flow: a routine refresh still leaves sale_ok
// alone (SaleOK nil → omitempty drops the field), the Odoo connection with
// the zero-stock-as-paused toggle passes false when qty=0 (visible but not
// buyable) and true when stock comes back. Deliberately NO
// name/weight/volume/category here — those stay with OdooProductSyncService
// .update's full flow.
type UpdateProductPriceStockVals struct {
	QtyAvailable float64 `json:"qty_available"`
	ListPrice    float64 `json:"list_price"`
	SaleOK       *bool   `json:"sale_ok,omitempty"`
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
	vals, err := h.valsWithoutTrackedStock(ctx, req.Credentials, req.ExternalID, req.Vals)
	if err != nil {
		return err
	}

	params := map[string]any{
		"ids":  []int64{req.ExternalID},
		"vals": vals,
	}

	return h.client.Call(ctx, req.Credentials, "product.template", "write", params, nil)
}

// trackedTemplatesTTL is how long the set of lot/serial-tracked templates is
// reused before asking Odoo again.
const trackedTemplatesTTL = 10 * time.Minute

type trackedTemplatesEntry struct {
	ids       map[int64]struct{}
	fetchedAt time.Time
}

var (
	trackedTemplatesMu    sync.Mutex
	trackedTemplatesCache = map[string]trackedTemplatesEntry{}
)

// trackedTemplateIDs returns the product.template ids tracked by lot or serial
// number in Odoo (tracking != 'none'), cached per client+database.
func (h *ProductsHandler) trackedTemplateIDs(ctx context.Context, credentials Credentials) (map[int64]struct{}, error) {
	key := fmt.Sprintf("%p|%s", h.client, credentials.Database)

	trackedTemplatesMu.Lock()
	entry, ok := trackedTemplatesCache[key]
	trackedTemplatesMu.Unlock()
	if ok && time.Since(entry.fetchedAt) < trackedTemplatesTTL {
		return entry.ids, nil
	}

	params := map[string]any{
		"domain":  []any{[]any{"tracking", "!=", "none"}},
		"fields":  []string{"id"},
		"context": map[string]any{"active_test": false},
	}

	var rows []struct {
		ID int64 `json:"id"`
	}
	if err := h.client.Call(ctx, credentials, "product.template", "search_read", params, &rows); err != nil {
		return nil, fmt.Errorf("error reading lot/serial tracked products: %w", err)
	}

	ids := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		ids[row.ID] = struct{}{}
	}

	trackedTemplatesMu.Lock()
	trackedTemplatesCache[key] = trackedTemplatesEntry{ids: ids, fetchedAt: time.Now()}
	trackedTemplatesMu.Unlock()

	return ids, nil
}

// valsWithoutTrackedStock drops qty_available from a product.template write
// when the template is tracked by lot or serial number. Writing qty_available
// makes Odoo book the whole quantity as a quant WITHOUT lot in the first
// warehouse, which double counts the units that datalake-bridge already keeps
// one by one per serial number (machines). Everything else is sent untouched.
func (h *ProductsHandler) valsWithoutTrackedStock(ctx context.Context, credentials Credentials, externalID int64, vals any) (any, error) {
	tracked, err := h.trackedTemplateIDs(ctx, credentials)
	if err != nil {
		return nil, err
	}
	if _, ok := tracked[externalID]; !ok {
		return vals, nil
	}

	raw, err := json.Marshal(vals)
	if err != nil {
		return nil, fmt.Errorf("error encoding vals for odoo product %d: %w", externalID, err)
	}
	fields := map[string]any{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("error decoding vals for odoo product %d: %w", externalID, err)
	}
	delete(fields, "qty_available")

	return fields, nil
}

// ActiveLanguageCodes returns the codes (es_MX, en_US, ...) of the languages
// installed and active in Odoo.
func (h *ProductsHandler) ActiveLanguageCodes(ctx context.Context, credentials Credentials) ([]string, error) {
	params := map[string]any{
		"domain": []any{[]any{"active", "=", true}},
		"fields": []string{"code"},
	}

	var rows []struct {
		Code string `json:"code"`
	}
	if err := h.client.Call(ctx, credentials, "res.lang", "search_read", params, &rows); err != nil {
		return nil, err
	}

	codes := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Code != "" {
			codes = append(codes, row.Code)
		}
	}
	return codes, nil
}

// UpdateProductNameInLanguages writes product.template.name once per language.
// The field is translatable and a plain write only updates the API user's own
// language (create stores en_US plus that language), so a rename sent without
// this leaves the old name in every other language — including en_US, the
// source one the storefront may fall back to.
func (h *ProductsHandler) UpdateProductNameInLanguages(ctx context.Context, credentials Credentials, externalID int64, name string, languages []string) error {
	for _, lang := range languages {
		params := map[string]any{
			"ids":     []int64{externalID},
			"vals":    map[string]any{"name": name},
			"context": map[string]any{"lang": lang},
		}
		if err := h.client.Call(ctx, credentials, "product.template", "write", params, nil); err != nil {
			return fmt.Errorf("error writing name in language %s: %w", lang, err)
		}
	}
	return nil
}

// UpdateProductVegusaFields writes only website_vegusa's vg_sku and
// vg_short_description on an existing product.template (empty values are
// skipped) — used by flows that must not touch name/price/stock.
func (h *ProductsHandler) UpdateProductVegusaFields(ctx context.Context, credentials Credentials, externalID int64, sku, shortDescription string) error {
	vals := map[string]any{}
	if sku != "" {
		vals["vg_sku"] = sku
	}
	if shortDescription != "" {
		vals["vg_short_description"] = shortDescription
	}
	if len(vals) == 0 {
		return nil
	}
	params := map[string]any{
		"ids":  []int64{externalID},
		"vals": vals,
	}
	return h.client.Call(ctx, credentials, "product.template", "write", params, nil)
}
