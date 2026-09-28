package odoo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// M2ORef decodifica un many2one conservando id y display_name (para la marca,
// donde el nombre sirve como término de búsqueda de la foto).
type M2ORef struct {
	ID   int64
	Name string
}

func (m *M2ORef) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "false" || s == "null" {
		return nil
	}
	var pair []json.RawMessage
	if err := json.Unmarshal(b, &pair); err == nil && len(pair) >= 1 {
		_ = json.Unmarshal(pair[0], &m.ID)
		if len(pair) >= 2 {
			_ = json.Unmarshal(pair[1], &m.Name)
		}
		return nil
	}
	return json.Unmarshal(b, &m.ID)
}

// MachineTemplate es una plantilla de máquina para el flujo de fotos.
type MachineTemplate struct {
	ID          int64  `json:"id"` // product.template id
	DefaultCode string `json:"default_code"`
	Name        string `json:"name"`
	Brand       M2ORef `json:"product_brand_id"`
	ImageSrc    Str    `json:"x_image_src"`
}

// SearchMachineTemplates lee las plantillas de una categoría (las máquinas), con
// marca y la fuente de imagen actual (si el campo existe).
func (c *Client) SearchMachineTemplates(ctx context.Context, categoryID int64, imageSrcField string) ([]MachineTemplate, error) {
	fields := []string{"id", "default_code", "name", "product_brand_id"}
	if imageSrcField != "" {
		fields = append(fields, imageSrcField)
	}
	var res []MachineTemplate
	err := c.Call(ctx, "product.template", "search_read", map[string]any{
		"domain": [][]any{{"categ_id", "=", categoryID}},
		"fields": fields,
		"limit":  1000,
		"order":  "default_code",
	}, &res)
	return res, err
}

// EnsureManualCharField garantiza un campo char manual (x_...) en un modelo.
// En dry-run no crea nada y devuelve "" si no existe.
func (c *Client) EnsureManualCharField(ctx context.Context, model, name, label string, dryRun bool) (string, error) {
	var found []struct {
		ID int64 `json:"id"`
	}
	if err := c.Call(ctx, "ir.model.fields", "search_read", map[string]any{
		"domain": [][]any{{"model", "=", model}, {"name", "=", name}},
		"fields": []string{"id"},
		"limit":  1,
	}, &found); err != nil {
		return "", err
	}
	if len(found) > 0 {
		return name, nil
	}
	if dryRun {
		return "", nil
	}
	var models []struct {
		ID int64 `json:"id"`
	}
	if err := c.Call(ctx, "ir.model", "search_read", map[string]any{
		"domain": [][]any{{"model", "=", model}},
		"fields": []string{"id"},
		"limit":  1,
	}, &models); err != nil {
		return "", err
	}
	if len(models) == 0 {
		return "", fmt.Errorf("no se encontró ir.model de %s", model)
	}
	var ids []int64
	if err := c.Call(ctx, "ir.model.fields", "create", map[string]any{
		"vals_list": []map[string]any{{
			"name":              name,
			"field_description": label,
			"model_id":          models[0].ID,
			"model":             model,
			"ttype":             "char",
			"state":             "manual",
		}},
	}, &ids); err != nil {
		return "", fmt.Errorf("crear campo %s: %w", name, err)
	}
	return name, nil
}

// SetProductImage fija la imagen de portada (image_1920, en base64) de una
// plantilla y registra la URL fuente en srcField (para el gate idempotente).
func (c *Client) SetProductImage(ctx context.Context, tmplID int64, imageB64, srcURL, srcField string) error {
	vals := map[string]any{"image_1920": imageB64}
	if srcField != "" {
		vals[srcField] = srcURL
	}
	return c.Call(ctx, "product.template", "write", map[string]any{
		"ids":  []int64{tmplID},
		"vals": vals,
	}, nil)
}

// ProductImageIDs devuelve los product.image (galería) de una plantilla.
func (c *Client) ProductImageIDs(ctx context.Context, tmplID int64) ([]int64, error) {
	var res []struct {
		ID int64 `json:"id"`
	}
	if err := c.Call(ctx, "product.image", "search_read", map[string]any{
		"domain": [][]any{{"product_tmpl_id", "=", tmplID}},
		"fields": []string{"id"},
	}, &res); err != nil {
		return nil, err
	}
	ids := make([]int64, len(res))
	for i, r := range res {
		ids[i] = r.ID
	}
	return ids, nil
}

// DeleteProductImages borra product.image por id (para reconciliar la galería).
func (c *Client) DeleteProductImages(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return c.Call(ctx, "product.image", "unlink", map[string]any{"ids": ids}, nil)
}

// CreateProductImage agrega una imagen de galería a una plantilla.
func (c *Client) CreateProductImage(ctx context.Context, tmplID int64, name, imageB64 string) error {
	return c.Call(ctx, "product.image", "create", map[string]any{
		"vals_list": []map[string]any{{
			"name":            name,
			"image_1920":      imageB64,
			"product_tmpl_id": tmplID,
		}},
	}, nil)
}
