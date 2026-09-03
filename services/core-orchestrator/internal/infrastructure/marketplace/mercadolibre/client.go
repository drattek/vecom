package mercadolibre

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.mercadolibre.com"

// maxTooManyRequestsRetries/tooManyRequestsBaseWait/tooManyRequestsMaxWait
// bound how Do reacts to a 429 from MercadoLibre itself — a sign that the
// account-wide budget this package's RateLimiter enforces (see
// MercadoLibreRateLimit) is still looser than what MercadoLibre actually
// allows for the endpoint being called (write endpoints like compatibility
// creation are throttled tighter than plain reads in practice).
// MercadoLibre's Retry-After header, when present, is honored over the
// fallback wait computed by tooManyRequestsBackoff — MercadoLibre doesn't
// send it in practice for this endpoint, so the fallback is what actually
// runs: it doubles each attempt (10s/20s/40s/60s/60s) rather than a fixed
// wait, since a fixed short wait was still landing on attempt 4-5 of 5 —
// meaning the account's real quota window hadn't reset yet between retries.
const (
	maxTooManyRequestsRetries = 5
	tooManyRequestsBaseWait   = 10 * time.Second
	tooManyRequestsMaxWait    = 60 * time.Second
)

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

// Do waits for the rate limiter's budget (if any) and then issues request,
// retrying up to maxTooManyRequestsRetries times if MercadoLibre itself
// responds 429 (see the const block above) rather than surfacing it as a
// hard failure — the rate limiter already spaced calls out per its own
// budget, so a 429 anyway means MercadoLibre's real limit for this
// particular endpoint is tighter than that budget right now. Every handler
// in this package sends its requests through here rather than calling
// httpClient.Do directly, so both the limiter and this retry apply to every
// endpoint (auth, categories, items, ...) regardless of which handler issues
// it.
func (c *Client) Do(request *http.Request) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
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

		if response.StatusCode != http.StatusTooManyRequests || attempt >= maxTooManyRequestsRetries {
			return response, nil
		}

		wait := retryAfterOrDefault(response, tooManyRequestsBackoff(attempt))
		response.Body.Close()
		log.Printf("mercadolibre client: %s %s -> 429 (quota exceeded), retrying in %s (attempt %d/%d)", request.Method, request.URL.String(), wait, attempt+1, maxTooManyRequestsRetries)

		// The body reader was already drained by the attempt above — rewind
		// it via GetBody (auto-populated by http.NewRequest for the
		// strings.Reader/bytes.Reader bodies every handler in this package
		// uses) so the retry resends the same payload rather than an empty
		// one. GET requests have a nil Body/GetBody to begin with, so this
		// is a no-op for them.
		if request.GetBody != nil {
			body, err := request.GetBody()
			if err != nil {
				return nil, fmt.Errorf("error rewinding mercadolibre request body for retry: %w", err)
			}
			request.Body = body
		}

		timer := time.NewTimer(wait)
		select {
		case <-request.Context().Done():
			timer.Stop()
			return nil, request.Context().Err()
		case <-timer.C:
		}
	}
}

// tooManyRequestsBackoff returns the fallback wait for retry attempt
// (0-indexed, the attempt that just got a 429): tooManyRequestsBaseWait
// doubled once per prior attempt, capped at tooManyRequestsMaxWait.
func tooManyRequestsBackoff(attempt int) time.Duration {
	wait := tooManyRequestsBaseWait
	for i := 0; i < attempt; i++ {
		wait *= 2
		if wait >= tooManyRequestsMaxWait {
			return tooManyRequestsMaxWait
		}
	}
	return wait
}

// retryAfterOrDefault reads the Retry-After header (seconds, MercadoLibre's
// documented form for its own 429s) off response and falls back to def when
// the header is absent or unparseable.
func retryAfterOrDefault(response *http.Response, def time.Duration) time.Duration {
	raw := strings.TrimSpace(response.Header.Get("Retry-After"))
	if raw == "" {
		return def
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 0 {
		return def
	}
	return time.Duration(seconds) * time.Second
}
