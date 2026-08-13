package mercadolibre

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// UsersHandler wraps MercadoLibre's /users endpoints — split out from
// ItemsHandler since it's a different API topic, per this package's
// one-handler-per-topic convention.
type UsersHandler struct {
	client *Client
}

func NewUsersHandler(client *Client) *UsersHandler {
	if client == nil {
		client = NewClient(nil, "", nil)
	}

	return &UsersHandler{client: client}
}

// Me is the subset of GET /users/me this integration needs: the
// authenticated seller's own numeric id, required to call
// /users/{id}/items/search — MercadoLibre has no "my own items" shortcut
// that skips knowing it.
type Me struct {
	ID int64 `json:"id"`
}

// GetMe calls GET /users/me, resolving the seller account behind
// accessToken.
func (h *UsersHandler) GetMe(ctx context.Context, accessToken string) (*Me, error) {
	requestURL := h.client.baseURL + "/users/me"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating mercadolibre users/me request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := h.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error calling mercadolibre users/me endpoint: %w", err)
	}
	defer response.Body.Close()

	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading mercadolibre users/me response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mercadolibre users/me fetch failed: %w", parseItemAPIError(response.StatusCode, responsePayload))
	}

	var me Me
	if err := json.Unmarshal(responsePayload, &me); err != nil {
		return nil, fmt.Errorf("error decoding mercadolibre users/me response: %w", err)
	}

	return &me, nil
}
