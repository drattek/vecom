package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Respaldos de la configuración de una conexión (ADR 0011). El contenido se
// guarda como un único JSON cifrado con la misma clave que los ajustes.

var ErrConnectionBackupNotFound = errors.New("connection settings backup not found")

// BackupSetting es un ajuste respaldado. IsEncrypted es cómo estaba guardado,
// para restaurarlo igual.
type BackupSetting struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	IsEncrypted bool   `json:"isEncrypted"`
}

type CreateConnectionBackupInput struct {
	ConnectionID int64
	Reason       string
	AppID        string
	Settings     []BackupSetting
	CreatedBy    int64
}

// ConnectionBackupDTO es lo que se puede mostrar: no incluye el contenido.
type ConnectionBackupDTO struct {
	ID          int64     `json:"id"`
	Reason      string    `json:"reason"`
	AppID       *string   `json:"appId"`
	CreatedAt   time.Time `json:"createdAt"`
	CreatedByID int64     `json:"createdById"`
	CreatedBy   *string   `json:"createdBy"`
}

type ConnectionSettingsBackupRepository struct {
	db Querier
	// settings aporta el cifrado.
	settings *ConnectionSettingsRepository
}

func NewConnectionSettingsBackupRepository(db Querier) *ConnectionSettingsBackupRepository {
	return &ConnectionSettingsBackupRepository{db: db, settings: NewConnectionSettingsRepository(db)}
}

func (r *ConnectionSettingsBackupRepository) Create(ctx context.Context, input CreateConnectionBackupInput) (int64, error) {
	encoded, err := json.Marshal(input.Settings)
	if err != nil {
		return 0, fmt.Errorf("error encoding connection backup: %w", err)
	}
	payload, err := r.settings.encryptIfNeeded(string(encoded), true)
	if err != nil {
		return 0, fmt.Errorf("error encrypting connection backup: %w", err)
	}
	var appID *string
	if input.AppID != "" {
		clipped := truncate(input.AppID, 64)
		appID = &clipped
	}
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_connection_settings_backups (connection_id, reason, app_id, payload, created_by)
		VALUES (?, ?, ?, ?, ?)`,
		input.ConnectionID, input.Reason, appID, payload, input.CreatedBy,
	)
	if err != nil {
		return 0, fmt.Errorf("error saving connection backup: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("error reading connection backup id: %w", err)
	}
	return id, nil
}

// HideOlderThan deja visibles los keep respaldos más recientes de la conexión.
// El resto se da de baja de forma lógica.
func (r *ConnectionSettingsBackupRepository) HideOlderThan(ctx context.Context, connectionID int64, keep int, actorID int64) error {
	// La subconsulta envuelta es necesaria: MySQL no permite LIMIT dentro de IN
	// ni leer en una subconsulta la tabla que se actualiza.
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_connection_settings_backups
		SET deleted_at = CURRENT_TIMESTAMP, updated_by = ?
		WHERE connection_id = ? AND deleted_at IS NULL AND id NOT IN (
			SELECT id FROM (
				SELECT id FROM ecom_connection_settings_backups
				WHERE connection_id = ? AND deleted_at IS NULL
				ORDER BY id DESC LIMIT ?
			) AS recent
		)`,
		actorID, connectionID, connectionID, keep,
	)
	if err != nil {
		return fmt.Errorf("error hiding old connection backups: %w", err)
	}
	return nil
}

func (r *ConnectionSettingsBackupRepository) FindByConnectionID(ctx context.Context, connectionID int64) ([]ConnectionBackupDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT b.id, b.reason, b.app_id, b.created_at, b.created_by, u.username
		FROM ecom_connection_settings_backups b
		LEFT JOIN ecom_api_user u ON u.id = b.created_by
		WHERE b.connection_id = ? AND b.deleted_at IS NULL
		ORDER BY b.id DESC`, connectionID)
	if err != nil {
		return nil, fmt.Errorf("error querying connection backups: %w", err)
	}
	return scanRows(rows, func(rows *sql.Rows, b *ConnectionBackupDTO) error {
		return rows.Scan(&b.ID, &b.Reason, &b.AppID, &b.CreatedAt, &b.CreatedByID, &b.CreatedBy)
	})
}

// FindSettings descifra un respaldo visible de esa conexión.
func (r *ConnectionSettingsBackupRepository) FindSettings(ctx context.Context, connectionID, backupID int64) ([]BackupSetting, error) {
	var payload string
	err := r.db.QueryRowContext(ctx, `
		SELECT payload FROM ecom_connection_settings_backups
		WHERE id = ? AND connection_id = ? AND deleted_at IS NULL`,
		backupID, connectionID,
	).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrConnectionBackupNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error reading connection backup: %w", err)
	}
	decoded, err := r.settings.decryptIfNeeded(payload, true)
	if err != nil {
		return nil, fmt.Errorf("error decrypting connection backup: %w", err)
	}
	var settings []BackupSetting
	if err := json.Unmarshal([]byte(decoded), &settings); err != nil {
		return nil, errors.New("el contenido del respaldo no es válido")
	}
	return settings, nil
}

// LockByConnectionID bloquea los ajustes de la conexión hasta que termine la
// transacción; solo tiene efecto sobre un *sql.Tx.
func (r *ConnectionSettingsRepository) LockByConnectionID(ctx context.Context, connectionID int64) error {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id FROM ecom_connection_settings WHERE connection_id = ? AND deleted_at IS NULL FOR UPDATE`,
		connectionID,
	)
	if err != nil {
		return fmt.Errorf("error locking connection settings: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error locking connection settings: %w", err)
	}
	return nil
}
