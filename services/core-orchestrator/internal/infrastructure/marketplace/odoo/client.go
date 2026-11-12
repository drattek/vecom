package odoo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

// StatusError is the error Call returns for a non-200 response, so callers can
// react to the HTTP status (errors.As) — e.g. retry a gateway timeout.
type StatusError struct {
	Model   string
	Method  string
	Status  int
	Message string
}

func (e *StatusError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("odoo %s.%s failed (%d): %s", e.Model, e.Method, e.Status, e.Message)
	}
	return fmt.Sprintf("odoo %s.%s failed with status %d", e.Model, e.Method, e.Status)
}

// IsGatewayTimeout reports whether err is a proxy/gateway error (502, 503,
// 504, or Cloudflare's 524): the request may well have completed in Odoo.
func IsGatewayTimeout(err error) bool {
	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	switch statusErr.Status {
	case 502, 503, 504, 524:
		return true
	}
	return false
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
		_ = json.Unmarshal(responsePayload, &errPayload)
		return &StatusError{Model: model, Method: method, Status: response.StatusCode, Message: errPayload.Message}
	}

	if out == nil {
		return nil
	}

	if err := json.Unmarshal(responsePayload, out); err != nil {
		return fmt.Errorf("error decoding odoo %s.%s response: %w", model, method, err)
	}

	return nil
}
