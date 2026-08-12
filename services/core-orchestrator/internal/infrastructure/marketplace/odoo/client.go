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

type Client struct {
	httpClient *http.Client
	baseURL    string
	limiter    *RateLimiter
}

// NewClient builds an Odoo API client. limiter is optional (nil disables
// rate limiting) and, when provided, should normally be a single instance
// shared across every Client so the budget it enforces is global to Odoo
// rather than per connection or per call site.
func NewClient(httpClient *http.Client, baseURL string, limiter *RateLimiter) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		limiter:    limiter,
	}
}

// Credentials holds the per-connection values required to call Odoo's
// External JSON-2 API: a bearer API key and, for multi-database
// deployments, the target database name.
type Credentials struct {
	APIKey   string
	Database string
}

type apiError struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// Call invokes POST {baseURL}/json/2/{model}/{method}, per Odoo's External
// JSON-2 API (https://www.odoo.com/documentation/19.0/developer/reference/external_api.html),
// and decodes the response body into out.
func (c *Client) Call(ctx context.Context, credentials Credentials, model, method string, params map[string]any, out any) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("error waiting for odoo %s.%s rate limit: %w", model, method, err)
	}

	payload, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("error encoding odoo %s.%s request: %w", model, method, err)
	}

	requestURL := fmt.Sprintf("%s/json/2/%s/%s", c.baseURL, model, method)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("error creating odoo %s.%s request: %w", model, method, err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "bearer "+credentials.APIKey)
	request.Header.Set("User-Agent", "vegusa-core-orchestrator odoo-client")
	if database := strings.TrimSpace(credentials.Database); database != "" {
		request.Header.Set("X-Odoo-Database", database)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("error calling odoo %s.%s: %w", model, method, err)
	}
	defer response.Body.Close()

	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("error reading odoo %s.%s response: %w", model, method, err)
	}

	if response.StatusCode != http.StatusOK {
		var errPayload apiError
		if json.Unmarshal(responsePayload, &errPayload) == nil && errPayload.Message != "" {
			return fmt.Errorf("odoo %s.%s failed (%d): %s", model, method, response.StatusCode, errPayload.Message)
		}
		return fmt.Errorf("odoo %s.%s failed with status %d", model, method, response.StatusCode)
	}

	if out == nil {
		return nil
	}

	if err := json.Unmarshal(responsePayload, out); err != nil {
		return fmt.Errorf("error decoding odoo %s.%s response: %w", model, method, err)
	}

	return nil
}
