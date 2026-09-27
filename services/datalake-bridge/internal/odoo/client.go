// Package odoo es un cliente mínimo de la API externa JSON-2 de Odoo 19,
// autónomo para datalake-bridge. Sigue el mismo patrón que el cliente del
// core-orchestrator (POST /json/2/{model}/{method}, bearer + X-Odoo-Database),
// con soporte explícito de "context" en el cuerpo, que hace falta para los
// ajustes de inventario (inventory_mode).
package odoo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client habla con un Odoo concreto (baseURL) por su API JSON-2.
type Client struct {
	http     *http.Client
	baseURL  string
	apiKey   string
	database string
}

// NewClient crea el cliente. apiKey y database son las credenciales de la API
// JSON-2 (bearer + cabecera X-Odoo-Database).
func NewClient(baseURL, apiKey, database string) *Client {
	return &Client{
		http:     &http.Client{Timeout: 60 * time.Second},
		baseURL:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:   apiKey,
		database: strings.TrimSpace(database),
	}
}

type apiError struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// Call invoca POST {baseURL}/json/2/{model}/{method}. body es el objeto de
// argumentos con nombre (kwargs); las claves especiales "ids" y "context" van
// al mismo nivel, tal como espera la API JSON-2. Decodifica la respuesta en out
// (out puede ser nil si no interesa el resultado).
func (c *Client) Call(ctx context.Context, model, method string, body map[string]any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("codificar %s.%s: %w", model, method, err)
	}

	url := fmt.Sprintf("%s/json/2/%s/%s", c.baseURL, model, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("crear request %s.%s: %w", model, method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "bearer "+c.apiKey)
	req.Header.Set("User-Agent", "vegusa-datalake-bridge odoo-client")
	if c.database != "" {
		req.Header.Set("X-Odoo-Database", c.database)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("llamar %s.%s: %w", model, method, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("leer respuesta %s.%s: %w", model, method, err)
	}

	if resp.StatusCode != http.StatusOK {
		var e apiError
		if json.Unmarshal(raw, &e) == nil && e.Message != "" {
			return fmt.Errorf("odoo %s.%s (%d): %s", model, method, resp.StatusCode, e.Message)
		}
		return fmt.Errorf("odoo %s.%s status %d: %s", model, method, resp.StatusCode, truncate(raw, 300))
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decodificar respuesta %s.%s: %w", model, method, err)
	}
	return nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
