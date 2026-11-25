package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// Respaldo automático y restauración de la configuración de MercadoLibre de
// una conexión (ADR 0011).

const (
	backupReasonAppUpdate   = "app_update"
	backupReasonAccountLink = "account_link"
	backupReasonRestore     = "restore"

	// visibleBackupsPerConnection: los anteriores se ocultan.
	visibleBackupsPerConnection = 5
)

// mercadoLibreBackupKeys son los ajustes que se respaldan y restauran juntos.
// El resto de la configuración de la conexión no se toca.
var mercadoLibreBackupKeys = []string{
	"app_id", "client_secret", "redirect_url",
	"access_token", "refresh_token", "expiration_time", mercadoLibreUserIDSetting,
}

// ErrBackupNotRestorable indica un respaldo sin los datos mínimos.
var ErrBackupNotRestorable = errors.New("el respaldo no tiene los datos de la aplicación y no se puede restaurar")

type MercadoLibreRestoreResult struct {
	MercadoLibreAppSettings
	// AccessVerified: tras restaurar, MercadoLibre aceptó los tokens.
	AccessVerified bool `json:"accessVerified"`
}

// mutateWithBackup ejecuta un cambio de los ajustes de autenticación dentro de
// una transacción que antes los respalda: si el respaldo falla, no se cambia
// nada. Quien llama debe tener el candado de la conexión.
func (s *MercadoLibreOAuthService) mutateWithBackup(
	ctx context.Context, connectionID, actorID int64, reason string,
	change func(repository *mysqlInfra.ConnectionSettingsRepository, settingsByKey map[string]mysqlInfra.ConnectionSettingDTO) error,
) error {
	return mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		repository := mysqlInfra.NewConnectionSettingsRepository(tx)
		if err := repository.LockByConnectionID(ctx, connectionID); err != nil {
			return err
		}
		settings, err := repository.FindByConnectionID(ctx, connectionID)
		if err != nil {
			return fmt.Errorf("error loading connection settings: %w", err)
		}
		settingsByKey := mercadoLibreSettingsByKey(settings)

		backups := mysqlInfra.NewConnectionSettingsBackupRepository(tx)
		if snapshot := backupSnapshot(settingsByKey); len(snapshot) > 0 {
			if _, err := backups.Create(ctx, mysqlInfra.CreateConnectionBackupInput{
				ConnectionID: connectionID,
				Reason:       reason,
				AppID:        strings.TrimSpace(settingsByKey["app_id"].Value),
				Settings:     snapshot,
				CreatedBy:    actorID,
			}); err != nil {
				return err
			}
			if err := backups.HideOlderThan(ctx, connectionID, visibleBackupsPerConnection, actorID); err != nil {
				return err
			}
		}
		return change(repository, settingsByKey)
	})
}

func backupSnapshot(settingsByKey map[string]mysqlInfra.ConnectionSettingDTO) []mysqlInfra.BackupSetting {
	snapshot := make([]mysqlInfra.BackupSetting, 0, len(mercadoLibreBackupKeys))
	for _, key := range mercadoLibreBackupKeys {
		if setting, ok := settingsByKey[key]; ok {
			snapshot = append(snapshot, mysqlInfra.BackupSetting{Key: key, Value: setting.Value, IsEncrypted: setting.IsEncrypted})
		}
	}
	return snapshot
}

// restorePlan decide qué escribir y qué dar de baja. Lo que existe hoy y no
// estaba en el respaldo se quita: p. ej. una cuenta registrada después no
// corresponde a los tokens restaurados.
func restorePlan(backup []mysqlInfra.BackupSetting, current map[string]mysqlInfra.ConnectionSettingDTO) (write []mysqlInfra.BackupSetting, remove []int64, err error) {
	restored := make(map[string]bool, len(backup))
	for _, setting := range backup {
		if !slices.Contains(mercadoLibreBackupKeys, setting.Key) {
			continue
		}
		restored[setting.Key] = true
		write = append(write, setting)
	}
	if !restored["app_id"] || !restored["client_secret"] {
		return nil, nil, ErrBackupNotRestorable
	}
	for _, key := range mercadoLibreBackupKeys {
		if existing, ok := current[key]; ok && !restored[key] {
			remove = append(remove, existing.ID)
		}
	}
	return write, remove, nil
}

// Backups lista los respaldos disponibles, sin su contenido.
func (s *MercadoLibreOAuthService) Backups(ctx context.Context, connectionID int64) ([]mysqlInfra.ConnectionBackupDTO, error) {
	if err := s.requireConnection(ctx, connectionID); err != nil {
		return nil, err
	}
	return mysqlInfra.NewConnectionSettingsBackupRepository(s.db).FindByConnectionID(ctx, connectionID)
}

// Restore repone un respaldo de la conexión, respaldando antes lo actual, y
// comprueba si los tokens restaurados todavía sirven. Que no sirvan no
// deshace la restauración.
func (s *MercadoLibreOAuthService) Restore(ctx context.Context, connectionID, backupID, actorID int64) (*MercadoLibreRestoreResult, error) {
	if err := s.requireConnection(ctx, connectionID); err != nil {
		return nil, err
	}
	if err := s.applyBackup(ctx, connectionID, backupID, actorID); err != nil {
		return nil, err
	}
	log.Printf("mercadolibre backups: connection %d restored from backup %d by user %d", connectionID, backupID, actorID)

	// Fuera del candado: renovar el token vuelve a pedirlo.
	verified := s.verifyAccess(ctx, connectionID)

	current, err := s.AppSettings(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	return &MercadoLibreRestoreResult{MercadoLibreAppSettings: *current, AccessVerified: verified}, nil
}

func (s *MercadoLibreOAuthService) applyBackup(ctx context.Context, connectionID, backupID, actorID int64) error {
	release, err := s.tokens.locks.acquire(ctx, connectionID)
	if err != nil {
		return err
	}
	defer release()

	// Se lee antes de respaldar lo actual: ese respaldo nuevo podría ocultar el
	// que se está restaurando si es el más antiguo.
	backup, err := mysqlInfra.NewConnectionSettingsBackupRepository(s.db).FindSettings(ctx, connectionID, backupID)
	if err != nil {
		return err
	}
	return s.mutateWithBackup(ctx, connectionID, actorID, backupReasonRestore,
		func(repository *mysqlInfra.ConnectionSettingsRepository, settingsByKey map[string]mysqlInfra.ConnectionSettingDTO) error {
			write, remove, err := restorePlan(backup, settingsByKey)
			if err != nil {
				return err
			}
			for _, setting := range write {
				if err := upsertSettingValue(ctx, repository, settingsByKey, connectionID, setting.Key, setting.Value, setting.IsEncrypted, actorID); err != nil {
					return fmt.Errorf("error restoring %s: %w", setting.Key, err)
				}
			}
			for _, id := range remove {
				if err := repository.SoftDelete(ctx, id); err != nil {
					return fmt.Errorf("error removing setting %d: %w", id, err)
				}
			}
			return nil
		})
}

// verifyAccess usa los tokens restaurados (renovándolos si ya vencieron) para
// preguntar a MercadoLibre quién es la cuenta.
func (s *MercadoLibreOAuthService) verifyAccess(ctx context.Context, connectionID int64) bool {
	token, err := s.tokens.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		log.Printf("mercadolibre backups: connection %d — restored tokens are not usable: %v", connectionID, err)
		return false
	}
	if _, err := s.users.GetMe(ctx, token); err != nil {
		log.Printf("mercadolibre backups: connection %d — restored tokens were rejected: %v", connectionID, err)
		return false
	}

	// El renovador programado usa la expiración del estado: debe coincidir con
	// la de los tokens que quedaron.
	s.syncStatusExpiration(ctx, connectionID)
	return true
}

func (s *MercadoLibreOAuthService) syncStatusExpiration(ctx context.Context, connectionID int64) {
	if s.tokens.statusRepository == nil {
		return
	}
	settings, err := s.tokens.settingsRepository.FindByConnectionID(ctx, connectionID)
	if err != nil {
		log.Printf("mercadolibre backups: connection %d — could not read expiration: %v", connectionID, err)
		return
	}
	expiresAt, err := parseMercadoLibreExpirationTime(mercadoLibreSettingsByKey(settings)["expiration_time"].Value)
	if err != nil {
		return
	}
	if _, err := s.tokens.statusRepository.Upsert(ctx, mysqlInfra.UpsertConnectionStatusInput{
		ConnectionID: connectionID, Authenticated: true, ExpiresAt: &expiresAt,
	}); err != nil {
		log.Printf("Error recording connection status for connection %d: %v", connectionID, err)
	}
}
