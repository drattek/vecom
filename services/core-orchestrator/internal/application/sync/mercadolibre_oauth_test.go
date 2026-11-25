package sync

import (
	"context"
	"errors"
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
	states := memoryStates{"abc": []byte(`{"connectionId":7,"actorId":1}`)}
	service := NewMercadoLibreOAuthService(nil, nil, nil, nil, states)

	result, err := service.Cancel(context.Background(), "abc")
	if err != nil || result.ConnectionID != 7 {
		t.Fatalf("primer uso: %+v %v", result, err)
	}
	if _, err := service.Complete(context.Background(), "abc", "codigo"); !errors.Is(err, ErrOAuthStateInvalid) {
		t.Errorf("segundo uso: %v", err)
	}
}

func TestOAuthStateIsRandom(t *testing.T) {
	first, errFirst := newOAuthState()
	second, errSecond := newOAuthState()
	if errFirst != nil || errSecond != nil || first == second || len(first) < 40 {
		t.Errorf("states = %q, %q (%v, %v)", first, second, errFirst, errSecond)
	}
}

func TestAuthorizationURLCarriesState(t *testing.T) {
	raw := mercadoLibreInfra.NewAuthHandler(nil).AuthorizationURL("123", "https://ejemplo.test/meli_callback", "st-1")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("state") != "st-1" || query.Get("redirect_uri") != "https://ejemplo.test/meli_callback" || query.Get("client_id") != "123" {
		t.Errorf("url = %s", raw)
	}
}
