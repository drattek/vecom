package sync

import (
	"context"
	"fmt"
	"strings"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// Lo que usa el reporte de diferencias (meli_report), que no puede escribir.

type connectionSettingsReader interface {
	FindByConnectionID(ctx context.Context, connectionID int64) ([]mysqlInfra.ConnectionSettingDTO, error)
}

// StoredMercadoLibreAccessToken devuelve el token guardado y cuándo vence, sin
// renovarlo. A diferencia de EnsureValidAccessToken no escribe nada: renovar
// cambia el refresh_token, y hacerlo desde un segundo proceso podría invalidar
// el que usa el servidor.
func StoredMercadoLibreAccessToken(ctx context.Context, settings connectionSettingsReader, connectionID int64) (string, time.Time, error) {
	values, err := settings.FindByConnectionID(ctx, connectionID)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("error loading connection settings: %w", err)
	}
	byKey := mercadoLibreSettingsByKey(values)

	token := strings.TrimSpace(byKey["access_token"].Value)
	if token == "" {
		return "", time.Time{}, fmt.Errorf("%w: access_token", ErrMissingMercadoLibreSettings)
	}
	expiresAt, err := parseMercadoLibreExpirationTime(byKey["expiration_time"].Value)
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// MercadoLibreListingPrice aplica al precio calculado las excepciones que usan
// los envíos a MercadoLibre.
func MercadoLibreListingPrice(sku string, calculated float64) float64 {
	return applyPinnedSKUPriceOverride(sku, calculated)
}
