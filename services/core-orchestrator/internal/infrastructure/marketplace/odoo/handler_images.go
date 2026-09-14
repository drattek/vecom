package odoo

import "context"

type ImagesHandler struct {
	client *Client
}

func NewImagesHandler(client *Client) *ImagesHandler {
	if client == nil {
		client = NewClient(nil, "", nil)
	}

	return &ImagesHandler{client: client}
}

// CreateProductImageVals mirrors product.image, Odoo's extra-photo model
// for a product.template (the cover photo goes inline as image_1920 on the
// product itself via product.template/create, not through this model).
type CreateProductImageVals struct {
	ProductTemplateID int64  `json:"product_tmpl_id"`
	Image1920         string `json:"image_1920"`
	Name              string `json:"name"`
}

type CreateProductImageRequest struct {
	Credentials Credentials
	Vals        CreateProductImageVals
}

// CreateProductImage calls product.image/create to attach one extra photo
// to an already-created product.template.
func (h *ImagesHandler) CreateProductImage(ctx context.Context, req CreateProductImageRequest) error {
	params := map[string]any{
		"vals_list": []CreateProductImageVals{req.Vals},
	}

	return h.client.Call(ctx, req.Credentials, "product.image", "create", params, nil)
}

type productImageIDRow struct {
	ID int64 `json:"id"`
}

// SearchProductImageIDs returns the ids of every product.image row already
// attached to productTemplateID — used by
// sync.OdooProductSyncService.Resync to clear out an already-synced
// product's extra photos before recreating them from
// ecom_product_images' current state (see DeleteProductImages), so a resync
// never accumulates duplicate photos across repeated runs.
func (h *ImagesHandler) SearchProductImageIDs(ctx context.Context, credentials Credentials, productTemplateID int64) ([]int64, error) {
	params := map[string]any{
		"domain": []any{[]any{"product_tmpl_id", "=", productTemplateID}},
		"fields": []string{"id"},
	}

	var rows []productImageIDRow
	if err := h.client.Call(ctx, credentials, "product.image", "search_read", params, &rows); err != nil {
		return nil, err
	}

	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}

	return ids, nil
}

// DeleteProductImages calls product.image/unlink to remove every id in ids —
// a no-op (no call made) when ids is empty, since Odoo's unlink rejects an
// empty id list.
func (h *ImagesHandler) DeleteProductImages(ctx context.Context, credentials Credentials, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	params := map[string]any{
		"ids": ids,
	}

	return h.client.Call(ctx, credentials, "product.image", "unlink", params, nil)
}
