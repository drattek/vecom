package sync

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
)

type memoryStates map[string][]byte

func (m memoryStates) Save(_ context.Context, state string, payload []byte, _ time.Duration) error {
	m[state] = payload
	return nil
}

func (m memoryStates) Take(_ context.Context, state string) ([]byte, bool, error) {
	payload, found := m[state]
	delete(m, state)
	return payload, found, nil
}

func TestCheckLinkedAccount(t *testing.T) {
	cases := []struct {
		name                string
		current, authorized int64
		others              []int64
		want                error
	}{
		{"misma cuenta", 10, 10, nil, nil},
		{"misma cuenta compartida con otra conexión", 10, 10, []int64{10}, nil},
		{"cuenta distinta a la vinculada", 10, 20, nil, ErrOAuthAccountMismatch},
		{"primera vinculación", 0, 20, []int64{10}, nil},
		{"sin cuenta previa y es la de otra conexión", 0, 10, []int64{10}, ErrOAuthAccountInUse},
	}
	for _, tc := range cases {
		if got := checkLinkedAccount(tc.current, tc.authorized, tc.others); !errors.Is(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Sin un state emitido por el servidor no se llega a la base ni a
// MercadoLibre (el servicio no tiene ninguna de las dos).
func TestOAuthRejectsUnknownState(t *testing.T) {
	service := NewMercadoLibreOAuthService(nil, nil, nil, nil, memoryStates{})
	for _, state := range []string{"", "inventado"} {
		result, err := service.Complete(context.Background(), state, "codigo")
		if !errors.Is(err, ErrOAuthStateInvalid) || result.ConnectionID != 0 {
			t.Errorf("state %q: result=%+v err=%v", state, result, err)
		}
	}
}

func TestOAuthStateIsSingleUse(t *testing.T) {
	states := memoryStates{"abc": []byte(`{"connectionId":7,"actorId":1,"codeVerifier":"v"}`)}
	service := NewMercadoLibreOAuthService(nil, nil, nil, nil, states)

	result, err := service.Cancel(context.Background(), "abc")
	if err != nil || result.ConnectionID != 7 {
		t.Fatalf("primer uso: %+v %v", result, err)
	}
	if _, err := service.Complete(context.Background(), "abc", "codigo"); !errors.Is(err, ErrOAuthStateInvalid) {
		t.Errorf("segundo uso: %v", err)
	}
}

func TestOAuthSecretIsRandom(t *testing.T) {
	first, errFirst := newOAuthSecret()
	second, errSecond := newOAuthSecret()
	// 43 caracteres es el mínimo que PKCE admite para el verificador.
	if errFirst != nil || errSecond != nil || first == second || len(first) != 43 {
		t.Errorf("secretos = %q, %q (%v, %v)", first, second, errFirst, errSecond)
	}
}

// Vector de prueba del apéndice B de RFC 7636.
func TestPKCEChallenge(t *testing.T) {
	got := pkceChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
	if want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"; got != want {
		t.Errorf("challenge = %q, want %q", got, want)
	}
}

// Un state sin verificador no se acepta: no se podría canjear con PKCE.
func TestOAuthRejectsStateWithoutVerifier(t *testing.T) {
	states := memoryStates{"abc": []byte(`{"connectionId":7,"actorId":1}`)}
	service := NewMercadoLibreOAuthService(nil, nil, nil, nil, states)
	if _, err := service.Complete(context.Background(), "abc", "codigo"); !errors.Is(err, ErrOAuthStateInvalid) {
		t.Errorf("err = %v", err)
	}
}

func TestAuthorizationURLCarriesStateAndChallenge(t *testing.T) {
	raw := mercadoLibreInfra.NewAuthHandler(nil).AuthorizationURL("123", "https://ejemplo.test/meli_callback", "st-1", "huella-1")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("state") != "st-1" || query.Get("redirect_uri") != "https://ejemplo.test/meli_callback" || query.Get("client_id") != "123" {
		t.Errorf("url = %s", raw)
	}
	if query.Get("code_challenge") != "huella-1" || query.Get("code_challenge_method") != "S256" || query.Has("code_verifier") {
		t.Errorf("PKCE en la url = %s", raw)
	}
}

// El verificador solo se envía al canjear el code, no al renovar.
func TestTokenRequestsAndCodeVerifier(t *testing.T) {
	var forms []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		forms = append(forms, r.PostForm)
		_, _ = w.Write([]byte(`{"access_token":"a","refresh_token":"r"}`))
	}))
	defer server.Close()
	auth := mercadoLibreInfra.NewAuthHandler(mercadoLibreInfra.NewClient(server.Client(), server.URL, nil))

	if _, err := auth.ExchangeAuthorizationCode(context.Background(), mercadoLibreInfra.ExchangeCodeRequest{
		ClientID: "1", ClientSecret: "s", Code: "c", RedirectURI: "https://ejemplo.test/meli_callback", CodeVerifier: "verificador",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.RefreshToken(context.Background(), mercadoLibreInfra.RefreshTokenRequest{ClientID: "1", ClientSecret: "s", RefreshToken: "r"}); err != nil {
		t.Fatal(err)
	}
	if len(forms) != 2 || forms[0].Get("code_verifier") != "verificador" || forms[0].Get("client_secret") != "s" || forms[1].Has("code_verifier") {
		t.Errorf("formularios = %v", forms)
	}
}
