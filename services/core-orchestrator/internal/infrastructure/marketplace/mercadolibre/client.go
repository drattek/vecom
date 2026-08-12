package mercadolibre

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.mercadolibre.com"

type Client struct {
	httpClient *http.Client
	baseURL    string
	limiter    *RateLimiter
}

// NewClient builds a MercadoLibre API client. limiter is optional (nil
// disables rate limiting) and, when provided, should normally be a single
// instance shared across every Client so the budget it enforces is global
// to MercadoLibre rather than per connection or per call site.
func NewClient(httpClient *http.Client, baseURL string, limiter *RateLimiter) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
		limiter:    limiter,
	}
}

// Do waits for the rate limiter's budget (if any) and then issues request.
// Every handler in this package sends its requests through here rather than
// calling httpClient.Do directly, so the limiter applies to every endpoint
// (auth, categories, items, ...) regardless of which handler issues it.
func (c *Client) Do(request *http.Request) (*http.Response, error) {
	log.Printf("mercadolibre client: waiting for rate limiter before %s %s", request.Method, request.URL.String())
	if err := c.limiter.Wait(request.Context()); err != nil {
		return nil, fmt.Errorf("error waiting for mercadolibre rate limit: %w", err)
	}

	log.Printf("mercadolibre client: sending %s %s", request.Method, request.URL.String())
	response, err := c.httpClient.Do(request)
	if err != nil {
		log.Printf("mercadolibre client: %s %s failed: %v", request.Method, request.URL.String(), err)
		return nil, err
	}
	log.Printf("mercadolibre client: %s %s -> status %d", request.Method, request.URL.String(), response.StatusCode)

	return response, nil
}
