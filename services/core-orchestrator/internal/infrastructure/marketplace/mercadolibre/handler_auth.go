package mercadolibre

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// mercadoLibreAuthDomain is MercadoLibre's authorization (login/consent)
// domain for the Mexico site (MLM), per
// https://developers.mercadolibre.com.mx/es_ar/autenticacion-y-autorizacion.
const mercadoLibreAuthDomain = "https://auth.mercadolibre.com.mx"

type AuthHandler struct {
	client *Client
}

type RefreshTokenRequest struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
}

// ExchangeCodeRequest carries the values needed to trade a one-time OAuth
// authorization code (obtained via AuthorizationURL) for the connection's
// first access_token/refresh_token pair.
type ExchangeCodeRequest struct {
	ClientID     string
	ClientSecret string
	Code         string
	RedirectURI  string
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Message      string `json:"message"`
	Error        string `json:"error"`
}

func NewAuthHandler(client *Client) *AuthHandler {
	if client == nil {
		client = NewClient(nil, "", nil)
	}

	return &AuthHandler{client: client}
}

// AuthorizationURL builds the URL to send a user to so they can authorize
// this app against their MercadoLibre account. The user approves there,
// then MercadoLibre redirects their browser to redirectURI with a `code`
// query parameter, which ExchangeAuthorizationCode trades for tokens.
func (h *AuthHandler) AuthorizationURL(clientID, redirectURI string) string {
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)

	return fmt.Sprintf("%s/authorization?%s", mercadoLibreAuthDomain, params.Encode())
}

func (h *AuthHandler) RefreshToken(ctx context.Context, req RefreshTokenRequest) (*RefreshTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", req.ClientID)
	form.Set("client_secret", req.ClientSecret)
	form.Set("refresh_token", req.RefreshToken)

	return h.requestToken(ctx, form)
}

// ExchangeAuthorizationCode trades the one-time code MercadoLibre's
// redirect handed back (after AuthorizationURL) for the connection's first
// access_token/refresh_token pair. redirectURI must exactly match the one
// used to obtain code.
func (h *AuthHandler) ExchangeAuthorizationCode(ctx context.Context, req ExchangeCodeRequest) (*RefreshTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", req.ClientID)
	form.Set("client_secret", req.ClientSecret)
	form.Set("code", req.Code)
	form.Set("redirect_uri", req.RedirectURI)

	return h.requestToken(ctx, form)
}

func (h *AuthHandler) requestToken(ctx context.Context, form url.Values) (*RefreshTokenResponse, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, h.client.baseURL+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("error creating mercadolibre oauth request: %w", err)
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := h.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error calling mercadolibre oauth endpoint: %w", err)
	}
	defer response.Body.Close()

	var payload RefreshTokenResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("error decoding mercadolibre oauth response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		if payload.Message != "" {
			return nil, fmt.Errorf("mercadolibre oauth request failed (%d): %s", response.StatusCode, payload.Message)
		}
		if payload.Error != "" {
			return nil, fmt.Errorf("mercadolibre oauth request failed (%d): %s", response.StatusCode, payload.Error)
		}
		return nil, fmt.Errorf("mercadolibre oauth request failed with status %d", response.StatusCode)
	}

	return &payload, nil
}
