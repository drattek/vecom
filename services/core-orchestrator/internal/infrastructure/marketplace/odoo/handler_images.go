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
