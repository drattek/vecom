package odoo

import (
	"context"
)

// SalesHandler lee ventas y sus registros relacionados (ADR 0010). Solo usa
// métodos de lectura (fields_get, search_read, read).
type SalesHandler struct {
	client *Client
}

func NewSalesHandler(client *Client) *SalesHandler {
	return &SalesHandler{client: client}
}

// Record es un registro de Odoo tal como llega. Los campos vacíos llegan como
// false y las relaciones many2one como [id, "nombre"].
type Record map[string]any

// FieldNames devuelve los campos que el modelo tiene en esta instancia; varían
// según la versión y los módulos instalados.
func (h *SalesHandler) FieldNames(ctx context.Context, credentials Credentials, model string) (map[string]bool, error) {
	var response map[string]any
	if err := h.client.Call(ctx, credentials, model, "fields_get",
		map[string]any{"attributes": []string{"type"}}, &response); err != nil {
		return nil, err
	}
	names := make(map[string]bool, len(response))
	for name := range response {
		names[name] = true
	}
	return names, nil
}

func (h *SalesHandler) SearchRead(ctx context.Context, credentials Credentials, model string, domain []any, fields []string, offset, limit int) ([]Record, error) {
	var records []Record
	err := h.client.Call(ctx, credentials, model, "search_read", map[string]any{
		"domain": domain,
		"fields": fields,
		"offset": offset,
		"limit":  limit,
		"order":  "id asc",
	}, &records)
	if err != nil {
		return nil, err
	}
	return records, nil
}
