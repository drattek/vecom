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
	Image1920         string `json:"image_1920,omitempty"`
	// VideoURL: enlace de YouTube; con él product.image no necesita imagen.
	VideoURL string `json:"video_url,omitempty"`
	Name     string `json:"name"`
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

const (
	// deleteImagesBatchSize: unlink de product.image es lento (borra también
	// el adjunto); en lotes chicos cada llamada queda por debajo del timeout
	// de 100 s de Cloudflare (error 524).
	deleteImagesBatchSize = 3
	// deleteImagesMaxAttempts: intentos por lote ante un error de gateway.
	deleteImagesMaxAttempts = 3
)

// DeleteProductImages calls product.image/unlink to remove every id in ids, in
// small batches — a no-op (no call made) when ids is empty, since Odoo's
// unlink rejects an empty id list.
//
// A gateway timeout (524, 504...) does not mean the unlink failed: Odoo usually
// keeps working after the proxy gave up. So on that error the batch is
// re-checked against Odoo (only ids that still exist are retried; unlinking a
// missing id raises an error) and retried.
func (h *ImagesHandler) DeleteProductImages(ctx context.Context, credentials Credentials, productTemplateID int64, ids []int64) error {
	for start := 0; start < len(ids); start += deleteImagesBatchSize {
		end := start + deleteImagesBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]

		for attempt := 1; len(batch) > 0; attempt++ {
			err := h.client.Call(ctx, credentials, "product.image", "unlink", map[string]any{"ids": batch}, nil)
			if err == nil {
				break
			}
			if !IsGatewayTimeout(err) || attempt >= deleteImagesMaxAttempts {
				return err
			}

			remaining, searchErr := h.SearchProductImageIDs(ctx, credentials, productTemplateID)
			if searchErr != nil {
				return err
			}
			still := make(map[int64]bool, len(remaining))
			for _, id := range remaining {
				still[id] = true
			}
			pending := make([]int64, 0, len(batch))
			for _, id := range batch {
				if still[id] {
					pending = append(pending, id)
				}
			}
			batch = pending
		}
	}

	return nil
}
