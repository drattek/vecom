package mysql

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

var (
	ErrConnectionSettingNotFound    = errors.New("connection setting not found")
	ErrInvalidSettingsEncryptionKey = errors.New("invalid CONNECTION_CREDENTIALS_ENCRYPTION_KEY: must be 32-byte raw text or base64-encoded 32-byte key")
)

type ConnectionSettingDTO struct {
	ID           int64     `json:"id"`
	ConnectionID int64     `json:"connectionId"`
	KeyName      string    `json:"keyName"`
	Value        string    `json:"value"`
	IsEncrypted  bool      `json:"isEncrypted"`
	CreatedBy    int64     `json:"createdBy"`
	UpdatedBy    *int64    `json:"updatedBy"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type PaginatedConnectionSettings struct {
	Data  []ConnectionSettingDTO `json:"data"`
	Total int                    `json:"total"`
}

type CreateConnectionSettingInput struct {
	ConnectionID int64
	KeyName      string
	Value        string
	IsEncrypted  bool
	CreatedBy    int64
}

type UpdateConnectionSettingInput struct {
	Value       string
	IsEncrypted bool
	UpdatedBy   int64
}

type ConnectionSettingsRepository struct {
	db            *sql.DB
	encryptionKey []byte
	encryptionErr error
}

func NewConnectionSettingsRepository(db *sql.DB) *ConnectionSettingsRepository {
	key, err := loadConnectionSettingsEncryptionKey()

	return &ConnectionSettingsRepository{
		db:            db,
		encryptionKey: key,
		encryptionErr: err,
	}
}

func (r *ConnectionSettingsRepository) FindPaginated(offset, pageSize int) (*PaginatedConnectionSettings, error) {
	query := `
		SELECT id, connection_id, key_name, value, is_encrypted, created_by, updated_by, created_at, updated_at
		FROM ecom_connection_settings
		WHERE deleted_at IS NULL
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []ConnectionSettingDTO
	for rows.Next() {
		var c ConnectionSettingDTO
		if err := rows.Scan(&c.ID, &c.ConnectionID, &c.KeyName, &c.Value, &c.IsEncrypted, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}

		decryptedValue, err := r.decryptIfNeeded(c.Value, c.IsEncrypted)
		if err != nil {
			return nil, err
		}
		c.Value = decryptedValue
		settings = append(settings, c)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_connection_settings WHERE deleted_at IS NULL"
	var total int
	if err := r.db.QueryRow(countQuery).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedConnectionSettings{Data: settings, Total: total}, nil
}

func (r *ConnectionSettingsRepository) FindByID(id int64) (*ConnectionSettingDTO, error) {
	query := `
		SELECT id, connection_id, key_name, value, is_encrypted, created_by, updated_by, created_at, updated_at
		FROM ecom_connection_settings
		WHERE id = ? AND deleted_at IS NULL
	`

	var c ConnectionSettingDTO
	if err := r.db.QueryRow(query, id).Scan(&c.ID, &c.ConnectionID, &c.KeyName, &c.Value, &c.IsEncrypted, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrConnectionSettingNotFound
		}
		return nil, err
	}

	decryptedValue, err := r.decryptIfNeeded(c.Value, c.IsEncrypted)
	if err != nil {
		return nil, err
	}
	c.Value = decryptedValue

	return &c, nil
}

func (r *ConnectionSettingsRepository) FindByConnectionID(connectionID int64) ([]ConnectionSettingDTO, error) {
	query := `
		SELECT id, connection_id, key_name, value, is_encrypted, created_by, updated_by, created_at, updated_at
		FROM ecom_connection_settings
		WHERE connection_id = ? AND deleted_at IS NULL
	`

	rows, err := r.db.Query(query, connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []ConnectionSettingDTO
	for rows.Next() {
		var c ConnectionSettingDTO
		if err := rows.Scan(&c.ID, &c.ConnectionID, &c.KeyName, &c.Value, &c.IsEncrypted, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}

		decryptedValue, err := r.decryptIfNeeded(c.Value, c.IsEncrypted)
		if err != nil {
			return nil, err
		}
		c.Value = decryptedValue
		settings = append(settings, c)
	}

	return settings, nil
}

func (r *ConnectionSettingsRepository) Create(input CreateConnectionSettingInput) (*ConnectionSettingDTO, error) {
	valueToStore, err := r.encryptIfNeeded(input.Value, input.IsEncrypted)
	if err != nil {
		return nil, err
	}

	query := `
		INSERT INTO ecom_connection_settings (connection_id, key_name, value, is_encrypted, created_by)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := r.db.Exec(query, input.ConnectionID, input.KeyName, valueToStore, input.IsEncrypted, input.CreatedBy)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.FindByID(id)
}

func (r *ConnectionSettingsRepository) Update(id int64, input UpdateConnectionSettingInput) (*ConnectionSettingDTO, error) {
	valueToStore, err := r.encryptIfNeeded(input.Value, input.IsEncrypted)
	if err != nil {
		return nil, err
	}

	query := `
		UPDATE ecom_connection_settings
		SET value = ?, is_encrypted = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, valueToStore, input.IsEncrypted, input.UpdatedBy, id)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, ErrConnectionSettingNotFound
	}

	return r.FindByID(id)
}

func (r *ConnectionSettingsRepository) SoftDelete(id int64) error {
	query := "UPDATE ecom_connection_settings SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL"

	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrConnectionSettingNotFound
	}

	return nil
}

func (r *ConnectionSettingsRepository) encryptIfNeeded(value string, isEncrypted bool) (string, error) {
	if !isEncrypted {
		return value, nil
	}

	if err := r.ensureEncryptionReady(); err != nil {
		return "", err
	}

	block, err := aes.NewCipher(r.encryptionKey)
	if err != nil {
		return "", err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aead.Seal(nil, nonce, []byte(value), nil)
	payload := append(nonce, ciphertext...)

	return base64.StdEncoding.EncodeToString(payload), nil
}

func (r *ConnectionSettingsRepository) decryptIfNeeded(value string, isEncrypted bool) (string, error) {
	if !isEncrypted {
		return value, nil
	}

	if err := r.ensureEncryptionReady(); err != nil {
		return "", err
	}

	payload, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted setting value: %w", err)
	}

	block, err := aes.NewCipher(r.encryptionKey)
	if err != nil {
		return "", err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aead.NonceSize()
	if len(payload) < nonceSize {
		return "", errors.New("invalid encrypted setting payload")
	}

	nonce := payload[:nonceSize]
	ciphertext := payload[nonceSize:]

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt setting value: %w", err)
	}

	return string(plaintext), nil
}

func (r *ConnectionSettingsRepository) ensureEncryptionReady() error {
	if r.encryptionErr != nil {
		return r.encryptionErr
	}

	if len(r.encryptionKey) != 32 {
		return ErrInvalidSettingsEncryptionKey
	}

	return nil
}

func loadConnectionSettingsEncryptionKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("CONNECTION_CREDENTIALS_ENCRYPTION_KEY"))
	if raw == "" {
		return nil, ErrInvalidSettingsEncryptionKey
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err == nil {
		if len(decoded) == 32 {
			return decoded, nil
		}
		return nil, ErrInvalidSettingsEncryptionKey
	}

	if len(raw) == 32 {
		return []byte(raw), nil
	}

	return nil, ErrInvalidSettingsEncryptionKey
}
