package odoo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func withFastRetries(t *testing.T) {
	t.Helper()
	prev := gatewayRetryDelays
	gatewayRetryDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { gatewayRetryDelays = prev })
}

func TestCallRetriesIdempotentMethodOnBadGateway(t *testing.T) {
	withFastRetries(t)

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("true"))
	}))
	defer server.Close()

	client := NewClient(server.Client(), server.URL, nil)
	if err := client.Call(context.Background(), Credentials{}, "product.template", "write", map[string]any{}, nil); err != nil {
		t.Fatalf("write should succeed after retries: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestCallDoesNotRetryCreate(t *testing.T) {
	withFastRetries(t)

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewClient(server.Client(), server.URL, nil)
	err := client.Call(context.Background(), Credentials{}, "product.image", "create", map[string]any{}, nil)
	if !IsGatewayTimeout(err) {
		t.Fatalf("expected a gateway error, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("create must not be retried, got %d attempts", got)
	}
}

func TestCallGivesUpAfterMaxRetries(t *testing.T) {
	withFastRetries(t)

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewClient(server.Client(), server.URL, nil)
	err := client.Call(context.Background(), Credentials{}, "product.template", "write", map[string]any{}, nil)
	if !IsGatewayTimeout(err) {
		t.Fatalf("expected a gateway error, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 4 {
		t.Fatalf("expected 1 + 3 retries = 4 attempts, got %d", got)
	}
}

func TestCallCapsInFlightRequests(t *testing.T) {
	var current, peak int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&current, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&current, -1)
		_, _ = w.Write([]byte("true"))
	}))
	defer server.Close()

	limiter := NewRateLimiter(1000, time.Second)
	client := NewClient(server.Client(), server.URL, limiter)

	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			_ = client.Call(context.Background(), Credentials{}, "product.template", "write", map[string]any{}, nil)
			done <- struct{}{}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}

	if got := atomic.LoadInt32(&peak); got > MaxInFlightCalls {
		t.Fatalf("peak in-flight calls %d exceeds cap %d", got, MaxInFlightCalls)
	}
}
