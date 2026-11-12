package odoo

import (
	"context"
	"sync"
	"time"
)

// RateLimiter caps how many Odoo API calls may be made within a sliding
// window, regardless of query type (products, categories, etc.). A single
// instance is meant to be shared across every Client built for Odoo calls,
// since Odoo enforces its own per-account rate limit independent of which
// model/method is being invoked.
type RateLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	calls  []time.Time
	// inFlight es un semáforo con las llamadas simultáneas permitidas: el rate
	// limit solo cuenta cuándo EMPIEZA cada llamada, así que varias llamadas
	// lentas (subir fotos, unlink) podían acumularse y saturar los workers de
	// Odoo (502/524).
	inFlight chan struct{}
}

// MaxInFlightCalls es cuántas llamadas a Odoo puede haber en curso a la vez.
const MaxInFlightCalls = 2

// Acquire reserves one in-flight slot, blocking until one is free or ctx is
// cancelled, and returns the function that releases it.
func (r *RateLimiter) Acquire(ctx context.Context) (func(), error) {
	if r == nil {
		return func() {}, nil
	}

	select {
	case r.inFlight <- struct{}{}:
		return func() { <-r.inFlight }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// NewRateLimiter builds a limiter allowing at most max calls within window.
func NewRateLimiter(max int, window time.Duration) *RateLimiter {
	return &RateLimiter{max: max, window: window, inFlight: make(chan struct{}, MaxInFlightCalls)}
}

// Wait blocks until a call is allowed under the limiter's budget, or returns
// ctx.Err() if ctx is cancelled first.
func (r *RateLimiter) Wait(ctx context.Context) error {
	if r == nil {
		return nil
	}

	for {
		wait, ok := r.reserve()
		if ok {
			return nil
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// reserve records a call and reports ok=true if it fits within the window;
// otherwise it reports how long to wait before the oldest call ages out.
func (r *RateLimiter) reserve() (time.Duration, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	kept := r.calls[:0]
	for _, t := range r.calls {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	r.calls = kept

	if len(r.calls) < r.max {
		r.calls = append(r.calls, now)
		return 0, true
	}

	return r.calls[0].Add(r.window).Sub(now), false
}
