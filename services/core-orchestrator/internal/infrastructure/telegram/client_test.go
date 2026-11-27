package telegram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testToken = "123456:AAbbCCddEEffGGhhIIjjKKll"

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := NewClient(server.Client())
	client.baseURL = server.URL
	return client
}

func TestGetMe(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot"+testToken+"/getMe" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"username":"vegusa_bot"}}`))
	})

	me, err := client.GetMe(context.Background(), testToken)
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if me.Username != "vegusa_bot" {
		t.Fatalf("username = %q", me.Username)
	}
}

func TestAPIErrorCarriesRetryAfter(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":7}}`))
	})

	err := client.SendMessage(context.Background(), testToken, 10, "hola")

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %v", err)
	}
	if apiErr.Code != 429 || apiErr.RetryAfter != 7*time.Second {
		t.Fatalf("unexpected error: %+v", apiErr)
	}
}

func TestTransportErrorHidesToken(t *testing.T) {
	client := NewClient(nil)
	// Puerto cerrado: net/http devuelve un *url.Error con la URL completa.
	client.baseURL = "http://127.0.0.1:1"

	err := client.SendMessage(context.Background(), testToken, 10, "hola")
	if err == nil {
		t.Fatal("expected an error")
	}
	for current := err; current != nil; current = errors.Unwrap(current) {
		if strings.Contains(current.Error(), testToken) {
			t.Fatalf("the token leaked in the error: %v", current)
		}
	}
}

func TestMalformedTokenIsRejectedWithoutRequest(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Error("no request expected")
	})

	for _, token := range []string{"", "abc", "123:short", "123456:AAbbCCddEEffGGhh/../../x"} {
		if _, err := client.GetMe(context.Background(), token); !errors.Is(err, ErrMalformedToken) {
			t.Errorf("token %q: got %v", token, err)
		}
	}
}
