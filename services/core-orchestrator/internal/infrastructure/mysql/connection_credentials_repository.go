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
	ErrConnectionCredentialNotFound    = errors.New("connection credential not found")
	ErrInvalidCredentialsEncryptionKey = errors.New("invalid CONNECTION_CREDENTIALS_ENCRYPTION_KEY: must be 32-byte raw text or base64-encoded 32-byte key")
)

type ConnectionCredentialDTO struct {
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

type PaginatedConnectionCredentials struct {
	Data  []ConnectionCredentialDTO `json:"data"`
	Total int                       `json:"total"`
}

type CreateConnectionCredentialInput struct {
	ConnectionID int64
	KeyName      string
	Value        string
	IsEncrypted  bool
	CreatedBy    int64
}

type UpdateConnectionCredentialInput struct {
	Value       string
	IsEncrypted bool
	UpdatedBy   int64
}

type ConnectionCredentialsRepository struct {
	db            *sql.DB
	encryptionKey []byte
	encryptionErr error
}

func NewConnectionCredentialsRepository(db *sql.DB) *ConnectionCredentialsRepository {
	key, err := loadConnectionCredentialsEncryptionKey()

	return &ConnectionCredentialsRepository{
		db:            db,
		encryptionKey: key,
		encryptionErr: err,
	}
}

func (r *ConnectionCredentialsRepository) FindPaginated(offset, pageSize int) (*PaginatedConnectionCredentials, error) {
	query := `
		SELECT id, connection_id, key_name, value, is_encrypted, created_by, updated_by, created_at, updated_at
		FROM ecom_connection_credentials
		WHERE deleted_at IS NULL
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var credentials []ConnectionCredentialDTO
	for rows.Next() {
		var c ConnectionCredentialDTO
		if err := rows.Scan(&c.ID, &c.ConnectionID, &c.KeyName, &c.Value, &c.IsEncrypted, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}

		decryptedValue, err := r.decryptIfNeeded(c.Value, c.IsEncrypted)
		if err != nil {
			return nil, err
		}
		c.Value = decryptedValue

		credentials = append(credentials, c)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_connection_credentials WHERE deleted_at IS NULL"
	var total int
	if err := r.db.QueryRow(countQuery).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedConnectionCredentials{Data: credentials, Total: total}, nil
}

func (r *ConnectionCredentialsRepository) FindByID(id int64) (*ConnectionCredentialDTO, error) {
	query := `
		SELECT id, connection_id, key_name, value, is_encrypted, created_by, updated_by, created_at, updated_at
		FROM ecom_connection_credentials
		WHERE id = ? AND deleted_at IS NULL
	`

	var c ConnectionCredentialDTO
	if err := r.db.QueryRow(query, id).Scan(&c.ID, &c.ConnectionID, &c.KeyName, &c.Value, &c.IsEncrypted, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrConnectionCredentialNotFound
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

func (r *ConnectionCredentialsRepository) FindByConnectionID(connectionID int64) ([]ConnectionCredentialDTO, error) {
	query := `
		SELECT id, connection_id, key_name, value, is_encrypted, created_by, updated_by, created_at, updated_at
		FROM ecom_connection_credentials
		WHERE connection_id = ? AND deleted_at IS NULL
	`

	rows, err := r.db.Query(query, connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var credentials []ConnectionCredentialDTO
	for rows.Next() {
		var c ConnectionCredentialDTO
		if err := rows.Scan(&c.ID, &c.ConnectionID, &c.KeyName, &c.Value, &c.IsEncrypted, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}

		decryptedValue, err := r.decryptIfNeeded(c.Value, c.IsEncrypted)
		if err != nil {
			return nil, err
		}
		c.Value = decryptedValue

		credentials = append(credentials, c)
	}

	return credentials, nil
}

func (r *ConnectionCredentialsRepository) Create(input CreateConnectionCredentialInput) (*ConnectionCredentialDTO, error) {
	valueToStore, err := r.encryptIfNeeded(input.Value, input.IsEncrypted)
	if err != nil {
		return nil, err
	}

	query := `
		INSERT INTO ecom_connection_credentials (connection_id, key_name, value, is_encrypted, created_by)
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

func (r *ConnectionCredentialsRepository) Update(id int64, input UpdateConnectionCredentialInput) (*ConnectionCredentialDTO, error) {
	valueToStore, err := r.encryptIfNeeded(input.Value, input.IsEncrypted)
	if err != nil {
		return nil, err
	}

	query := `
		UPDATE ecom_connection_credentials
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
		return nil, ErrConnectionCredentialNotFound
	}

	return r.FindByID(id)
}

func (r *ConnectionCredentialsRepository) SoftDelete(id int64) error {
	query := "UPDATE ecom_connection_credentials SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL"

	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrConnectionCredentialNotFound
	}

	return nil
}

func (r *ConnectionCredentialsRepository) encryptIfNeeded(value string, isEncrypted bool) (string, error) {
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

func (r *ConnectionCredentialsRepository) decryptIfNeeded(value string, isEncrypted bool) (string, error) {
	if !isEncrypted {
		return value, nil
	}

	if err := r.ensureEncryptionReady(); err != nil {
		return "", err
	}

	payload, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted credential value: %w", err)
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
		return "", errors.New("invalid encrypted credential payload")
	}

	nonce := payload[:nonceSize]
	ciphertext := payload[nonceSize:]

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt credential value: %w", err)
	}

	return string(plaintext), nil
}

func (r *ConnectionCredentialsRepository) ensureEncryptionReady() error {
	if r.encryptionErr != nil {
		return r.encryptionErr
	}

	if len(r.encryptionKey) != 32 {
		return ErrInvalidCredentialsEncryptionKey
	}

	return nil
}

func loadConnectionCredentialsEncryptionKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("CONNECTION_CREDENTIALS_ENCRYPTION_KEY"))
	if raw == "" {
		return nil, ErrInvalidCredentialsEncryptionKey
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err == nil {
		if len(decoded) == 32 {
			return decoded, nil
		}
		return nil, ErrInvalidCredentialsEncryptionKey
	}

	if len(raw) == 32 {
		return []byte(raw), nil
	}

	return nil, ErrInvalidCredentialsEncryptionKey
}
