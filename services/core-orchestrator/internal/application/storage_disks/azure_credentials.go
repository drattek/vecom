package storage_disks

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	azureBlob "core-orchestrator/internal/infrastructure/storage/azureblob"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrInvalidAzureCredentials = errors.New("accountName, accountKey and containerName are required")

// SetAzureCredentials stores the Azure Blob Storage access data of a disk,
// encrypted, replacing whatever it had. The three values are written in one
// transaction so a disk never ends up pointing at an account with another
// account's key. They are never returned by the API.
func (s *StorageDiskService) SetAzureCredentials(ctx context.Context, diskID int64, accountName, accountKey, containerName string, actorID int64) error {
	accountName = strings.TrimSpace(accountName)
	accountKey = strings.TrimSpace(accountKey)
	containerName = strings.TrimSpace(containerName)
	if diskID <= 0 || accountName == "" || accountKey == "" || containerName == "" {
		return ErrInvalidAzureCredentials
	}

	if _, err := s.repository.FindByID(ctx, diskID); err != nil {
		if errors.Is(err, mysqlInfra.ErrStorageDiskNotFound) {
			return ErrStorageDiskNotFound
		}
		return err
	}

	return mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		repo := s.credentialsRepository.WithQuerier(tx)
		for key, value := range map[string]string{
			azureBlob.KeyAccountName:   accountName,
			azureBlob.KeyAccountKey:    accountKey,
			azureBlob.KeyContainerName: containerName,
		} {
			if err := repo.Upsert(ctx, diskID, key, value, actorID); err != nil {
				return err
			}
		}
		return nil
	})
}
