// Package schedulers holds cron-driven background jobs for core-orchestrator.
//
// TokenRefreshScheduler is wired into cmd/main.go: it's constructed after
// mysqlRepos and mercadoLibreTokenService, registered for "MERCADOLIBRE", and
// started via `go tokenRefreshScheduler.Start(context.Background())`.
// Register additional marketplaces there as they gain OAuth refresh support —
// Odoo (API-key based) needs no registration: connections without a
// registered refresher, or without expires_at set, are simply skipped.
//
// The channel code passed to Register must match ecom_channels.code for the
// corresponding row (case-insensitive).
package schedulers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/shared/safe"
)

// TokenRefresher is implemented by any per-marketplace service capable of
// ensuring a connection's access token is valid, refreshing it (and
// persisting the result to ecom_connection_settings / ecom_connection_status)
// as a side effect when needed.
//
// *sync.MercadoLibreTokenService already satisfies this interface via its
// EnsureValidAccessToken method — no adapter needed to register it.
type TokenRefresher interface {
	EnsureValidAccessToken(ctx context.Context, connectionID int64) (string, error)
}

type TokenRefreshSchedulerConfig struct {
	// PollInterval is how often the scheduler checks for connections due for
	// refresh.
	PollInterval time.Duration
	// ExpiryLookahead marks a connection as "due" once its token expires
	// within this window. It should be greater than PollInterval so a
	// connection gets re-checked at least once before it actually expires.
	ExpiryLookahead time.Duration
}

// TokenRefreshScheduler periodically finds channel connections whose token is
// close to expiring (via ConnectionStatusRepository.FindDueForRefresh) and
// delegates the actual refresh to the TokenRefresher registered for that
// connection's channel code. Refresh timing is entirely data-driven — it
// comes from the expires_at each marketplace's own refresh response reports,
// not a hardcoded per-marketplace interval.
type TokenRefreshScheduler struct {
	statusRepository *mysqlInfra.ConnectionStatusRepository
	config           TokenRefreshSchedulerConfig
	refreshers       map[string]TokenRefresher
}

func NewTokenRefreshScheduler(statusRepository *mysqlInfra.ConnectionStatusRepository, config TokenRefreshSchedulerConfig) *TokenRefreshScheduler {
	return &TokenRefreshScheduler{
		statusRepository: statusRepository,
		config:           config,
		refreshers:       make(map[string]TokenRefresher),
	}
}

// Register associates a TokenRefresher with a channel code (ecom_channels.code).
// Connections whose channel has no registered refresher are left untouched.
func (s *TokenRefreshScheduler) Register(channelCode string, refresher TokenRefresher) {
	s.refreshers[normalizeChannelCode(channelCode)] = refresher
}

// Start runs the polling loop until ctx is cancelled. It is blocking — call it
// with `go scheduler.Start(ctx)`.
func (s *TokenRefreshScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	s.runOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *TokenRefreshScheduler) runOnce(ctx context.Context) {
	threshold := time.Now().UTC().Add(s.config.ExpiryLookahead)

	due, err := s.statusRepository.FindDueForRefresh(ctx, threshold)
	if err != nil {
		log.Printf("token refresh scheduler: error listing due connections: %v", err)
		return
	}

	log.Printf("token refresh scheduler: poll found %d connection(s) due for refresh (threshold=%s)", len(due), threshold.Format(time.RFC3339))

	for _, connection := range due {
		connection := connection

		refresher, ok := s.refreshers[normalizeChannelCode(connection.ChannelCode)]
		if !ok {
			log.Printf("token refresh scheduler: connection %d (channel %s) is due but has no registered refresher, skipping", connection.ConnectionID, connection.ChannelCode)
			continue
		}

		// Barrera de panic por conexión: un panic refrescando una no debe
		// impedir el refresh de las demás ni tumbar el scheduler.
		safe.Do(fmt.Sprintf("token refresh scheduler: connection %d", connection.ConnectionID), func() {
			log.Printf("token refresh scheduler: connection %d (channel %s) — token expires at %s, invoking refresher", connection.ConnectionID, connection.ChannelCode, connection.ExpiresAt.Format(time.RFC3339))

			if _, err := refresher.EnsureValidAccessToken(ctx, connection.ConnectionID); err != nil {
				log.Printf("token refresh scheduler: connection %d (channel %s) — error refreshing token: %v", connection.ConnectionID, connection.ChannelCode, err)
				return
			}

			log.Printf("token refresh scheduler: connection %d (channel %s) — token check complete", connection.ConnectionID, connection.ChannelCode)
		})
	}
}

func normalizeChannelCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}
