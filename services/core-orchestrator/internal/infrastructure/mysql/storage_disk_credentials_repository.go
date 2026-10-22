package mysql

import (
	"context"
	"fmt"
)

// StorageDiskCredentialsRepository keeps the secrets a storage disk needs to
// be written to (for Azure Blob: account_name, account_key, container_name) in
// ecom_storage_disk_credentials, one row per key. Every value is stored
// encrypted (see CredentialCipher), so a database leak doesn't expose them and
// pointing the disk at another container is just updating these rows.
type StorageDiskCredentialsRepository struct {
	db     Querier
	cipher *CredentialCipher
}

func NewStorageDiskCredentialsRepository(db Querier, cipher *CredentialCipher) *StorageDiskCredentialsRepository {
	return &StorageDiskCredentialsRepository{db: db, cipher: cipher}
}

// WithQuerier returns a repository that runs on q (typically a *sql.Tx inside
// mysql.WithinTx) and shares this one's cipher.
func (r *StorageDiskCredentialsRepository) WithQuerier(q Querier) *StorageDiskCredentialsRepository {
	return &StorageDiskCredentialsRepository{db: q, cipher: r.cipher}
}

// FindByDiskID returns the disk's credentials decrypted, keyed by key_name. An
// empty map means the disk has none.
func (r *StorageDiskCredentialsRepository) FindByDiskID(ctx context.Context, diskID int64) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT key_name, value
		FROM ecom_storage_disk_credentials
		WHERE disk_id = ? AND deleted_at IS NULL
	`, diskID)
	if err != nil {
		return nil, fmt.Errorf("error loading credentials for storage disk %d: %w", diskID, err)
	}
	defer rows.Close()

	credentials := make(map[string]string)
	for rows.Next() {
		var keyName, stored string
		if err := rows.Scan(&keyName, &stored); err != nil {
			return nil, fmt.Errorf("error scanning storage disk credential: %w", err)
		}
		value, err := r.cipher.Decrypt(stored)
		if err != nil {
			return nil, fmt.Errorf("error decrypting storage disk credential %q: %w", keyName, err)
		}
		credentials[keyName] = value
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return credentials, nil
}

// Upsert encrypts value and stores it under (diskID, keyName), replacing the
// previous value. ecom_storage_disk_credentials has a unique key on
// (disk_id, key_name); a soft-deleted row is revived.
func (r *StorageDiskCredentialsRepository) Upsert(ctx context.Context, diskID int64, keyName, value string, actorID int64) error {
	encrypted, err := r.cipher.Encrypt(value)
	if err != nil {
		return err
	}

	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_storage_disk_credentials (disk_id, key_name, value, is_encrypted, created_by)
		VALUES (?, ?, ?, 1, ?)
		ON DUPLICATE KEY UPDATE value = VALUES(value), is_encrypted = 1, updated_by = VALUES(created_by), deleted_at = NULL
	`, diskID, keyName, encrypted, actorID); err != nil {
		return fmt.Errorf("error saving credential %q for storage disk %d: %w", keyName, diskID, err)
	}
	return nil
}
