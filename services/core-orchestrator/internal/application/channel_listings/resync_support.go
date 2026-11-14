package channel_listings

import (
	"context"
	"errors"
	"fmt"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// ValidateResyncConnection comprueba que connectionID existe y que su canal tiene
// un FullRefresher registrado (hoy MercadoLibre y Odoo), o sea, que
// ResyncProductListings puede atenderla. Devuelve ErrInvalidChannelConnection o
// ErrNoFullRefresherForChannel (envuelto) y, si todo está bien, el nombre de la
// conexión para mostrarlo en mensajes y reportes.
func (s *Service) ValidateResyncConnection(ctx context.Context, connectionID int64) (name string, err error) {
	if connectionID <= 0 {
		return "", ErrInvalidChannelConnection
	}
	connection, err := s.channelConnectionRepository.FindByID(ctx, connectionID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
			return "", ErrInvalidChannelConnection
		}
		return "", fmt.Errorf("error loading connection %d: %w", connectionID, err)
	}
	channel, err := s.channelRepository.FindByID(ctx, connection.ChannelID)
	if err != nil {
		return "", fmt.Errorf("error loading channel %d: %w", connection.ChannelID, err)
	}
	if _, ok := s.fullRefreshers[normalizeChannelCode(channel.Code)]; !ok {
		return "", fmt.Errorf("%w: %s", ErrNoFullRefresherForChannel, channel.Code)
	}
	return connection.Name, nil
}

type resyncOmitMissingCategoryKey struct{}

// WithResyncOmitMissingCategory marks ctx so that a channel's Resync, when the
// product has no usable category on the connection (none chosen, or not mapped
// to an external category), still pushes the rest of the fields and just leaves
// the category out instead of skipping/failing the whole resync. Used by the
// bulk update; the manual button and the queue keep the strict behavior.
func WithResyncOmitMissingCategory(ctx context.Context) context.Context {
	return context.WithValue(ctx, resyncOmitMissingCategoryKey{}, true)
}

// ResyncOmitMissingCategory reports whether ctx carries WithResyncOmitMissingCategory.
func ResyncOmitMissingCategory(ctx context.Context) bool {
	v, _ := ctx.Value(resyncOmitMissingCategoryKey{}).(bool)
	return v
}
