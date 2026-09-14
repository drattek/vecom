package odoo

import (
	"context"
	"fmt"
)

// ProductInfoModel is the custom Odoo model (added by this deployment's website
// plugin) that stores a product.template's extra attributes as flat key/value
// rows: info_key / info_value, with a unique (product_tmpl_id, info_key). See
// ADR 0004.
const ProductInfoModel = "website.sale.product.info"

// ProductInfoRecord mirrors one website.sale.product.info row.
type ProductInfoRecord struct {
	ID        int64           `json:"id"`
	InfoKey   FalseableString `json:"info_key"`
	InfoValue FalseableString `json:"info_value"`
}

var productInfoSearchReadFields = []string{"id", "info_key", "info_value"}

// ProductInfoHandler is the CRUD surface for website.sale.product.info over
// Odoo's External JSON-2 API.
type ProductInfoHandler struct {
	client *Client
}

func NewProductInfoHandler(client *Client) *ProductInfoHandler {
	if client == nil {
		client = NewClient(nil, "", nil)
	}

	return &ProductInfoHandler{client: client}
}

// SearchReadByProductTemplate returns every website.sale.product.info row
// attached to productTmplID.
func (h *ProductInfoHandler) SearchReadByProductTemplate(ctx context.Context, credentials Credentials, productTmplID int64) ([]ProductInfoRecord, error) {
	params := map[string]any{
		"domain": []any{[]any{"product_tmpl_id", "=", productTmplID}},
		"fields": productInfoSearchReadFields,
	}

	var records []ProductInfoRecord
	if err := h.client.Call(ctx, credentials, ProductInfoModel, "search_read", params, &records); err != nil {
		return nil, err
	}

	return records, nil
}

// CreateProductInfoVals mirrors the fields this integration sets when adding a
// key/value row. Sequence orders the rows in Odoo (_order = 'sequence, id').
type CreateProductInfoVals struct {
	ProductTmplID int64  `json:"product_tmpl_id"`
	InfoKey       string `json:"info_key"`
	InfoValue     string `json:"info_value"`
	Sequence      int    `json:"sequence"`
}

// CreateProductInfo calls website.sale.product.info/create with vals wrapped in
// vals_list (Odoo's External JSON-2 create always takes a list) and returns the
// new row ids.
func (h *ProductInfoHandler) CreateProductInfo(ctx context.Context, credentials Credentials, vals []CreateProductInfoVals) ([]int64, error) {
	if len(vals) == 0 {
		return nil, nil
	}

	params := map[string]any{"vals_list": vals}

	var ids []int64
	if err := h.client.Call(ctx, credentials, ProductInfoModel, "create", params, &ids); err != nil {
		return nil, err
	}

	return ids, nil
}

// UpdateProductInfoVals is the subset of fields a key/value row update touches:
// only the value (the key is the stable identifier, and is never rewritten).
type UpdateProductInfoVals struct {
	InfoValue string `json:"info_value"`
}

// WriteProductInfo calls website.sale.product.info/write to change one row's value.
func (h *ProductInfoHandler) WriteProductInfo(ctx context.Context, credentials Credentials, id int64, vals UpdateProductInfoVals) error {
	params := map[string]any{
		"ids":  []int64{id},
		"vals": vals,
	}

	return h.client.Call(ctx, credentials, ProductInfoModel, "write", params, nil)
}

// UnlinkProductInfo deletes the given website.sale.product.info rows.
func (h *ProductInfoHandler) UnlinkProductInfo(ctx context.Context, credentials Credentials, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	params := map[string]any{"ids": ids}

	if err := h.client.Call(ctx, credentials, ProductInfoModel, "unlink", params, nil); err != nil {
		return fmt.Errorf("error unlinking %s rows %v: %w", ProductInfoModel, ids, err)
	}

	return nil
}
