package channel_config

import (
	"context"
	"errors"
	"fmt"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// MaskedValue es lo que se devuelve en lugar de un valor sensible. El valor
// real nunca sale de este paquete hacia el navegador.
const MaskedValue = "••••••••"

// ConnectionConfigEntry es una fila de solo lectura de credenciales o ajustes
// de una conexión. Cuando Masked es true, Value es MaskedValue.
type ConnectionConfigEntry struct {
	KeyName     string    `json:"keyName"`
	Value       string    `json:"value"`
	Masked      bool      `json:"masked"`
	IsEncrypted bool      `json:"isEncrypted"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ConnectionConfigView es la vista de solo lectura de lo que una conexión
// tiene configurado: credenciales (siempre enmascaradas), ajustes (enmascarados
// solo si están marcados como cifrados) y el último estado de autenticación.
type ConnectionConfigView struct {
	ConnectionID int64                           `json:"connectionId"`
	Credentials  []ConnectionConfigEntry         `json:"credentials"`
	Settings     []ConnectionConfigEntry         `json:"settings"`
	Status       *mysqlInfra.ConnectionStatusDTO `json:"status"`
}

type ConnectionConfigViewService struct {
	credentials *ConnectionCredentialsService
	settings    *ConnectionSettingsService
	status      *ConnectionStatusService
}

func NewConnectionConfigViewService(
	credentials *ConnectionCredentialsService,
	settings *ConnectionSettingsService,
	status *ConnectionStatusService,
) *ConnectionConfigViewService {
	return &ConnectionConfigViewService{credentials: credentials, settings: settings, status: status}
}

func (s *ConnectionConfigViewService) GetView(ctx context.Context, connectionID int64) (*ConnectionConfigView, error) {
	credentials, err := s.credentials.GetCredentialsByConnection(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("loading connection credentials: %w", err)
	}

	settings, err := s.settings.GetSettingsByConnection(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("loading connection settings: %w", err)
	}

	status, err := s.status.GetStatusByConnection(ctx, connectionID)
	if err != nil {
		if !errors.Is(err, mysqlInfra.ErrConnectionStatusNotFound) {
			return nil, fmt.Errorf("loading connection status: %w", err)
		}
		status = nil
	}

	view := &ConnectionConfigView{
		ConnectionID: connectionID,
		Credentials:  make([]ConnectionConfigEntry, 0, len(credentials)),
		Settings:     make([]ConnectionConfigEntry, 0, len(settings)),
		Status:       status,
	}

	for _, c := range credentials {
		// Una credencial nunca se muestra, esté o no cifrada en BD.
		view.Credentials = append(view.Credentials, maskedEntry(c.KeyName, c.Value, true, c.IsEncrypted, c.UpdatedAt))
	}
	for _, st := range settings {
		view.Settings = append(view.Settings, maskedEntry(st.KeyName, st.Value, st.IsEncrypted, st.IsEncrypted, st.UpdatedAt))
	}

	return view, nil
}

func maskedEntry(keyName, value string, hide, isEncrypted bool, updatedAt time.Time) ConnectionConfigEntry {
	if hide && value != "" {
		return ConnectionConfigEntry{KeyName: keyName, Value: MaskedValue, Masked: true, IsEncrypted: isEncrypted, UpdatedAt: updatedAt}
	}
	return ConnectionConfigEntry{KeyName: keyName, Value: value, Masked: false, IsEncrypted: isEncrypted, UpdatedAt: updatedAt}
}
