package sync

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

func TestBackupSnapshotTakesOnlyAuthSettings(t *testing.T) {
	snapshot := backupSnapshot(mercadoLibreSettingsByKey([]mysqlInfra.ConnectionSettingDTO{
		{KeyName: "App_ID", Value: "111", IsEncrypted: false},
		{KeyName: "client_secret", Value: "s", IsEncrypted: true},
		{KeyName: "site_id", Value: "MLM"},
	}))
	if len(snapshot) != 2 || snapshot[0].Key != "app_id" || snapshot[0].IsEncrypted || !snapshot[1].IsEncrypted {
		t.Errorf("snapshot = %+v", snapshot)
	}
}

func TestRestorePlan(t *testing.T) {
	backup := []mysqlInfra.BackupSetting{
		{Key: "app_id", Value: "111"},
		{Key: "client_secret", Value: "s", IsEncrypted: true},
		{Key: "site_id", Value: "no debe restaurarse"},
	}
	current := mercadoLibreSettingsByKey([]mysqlInfra.ConnectionSettingDTO{
		{ID: 1, KeyName: "app_id", Value: "222"},
		{ID: 2, KeyName: "meli_user_id", Value: "9"},
		{ID: 3, KeyName: "site_id", Value: "MLM"},
	})
	write, remove, err := restorePlan(backup, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(write) != 2 || !slices.Equal(remove, []int64{2}) {
		t.Errorf("write = %+v, remove = %v", write, remove)
	}

	if _, _, err := restorePlan([]mysqlInfra.BackupSetting{{Key: "access_token", Value: "x"}}, current); !errors.Is(err, ErrBackupNotRestorable) {
		t.Errorf("err = %v", err)
	}
}

func TestConnectionLocksSerializePerConnection(t *testing.T) {
	var locks connectionLocks
	release, err := locks.acquire(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	// Otra conexión no espera.
	other, err := locks.acquire(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	other()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := locks.acquire(ctx, 1); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("la misma conexión debe esperar: %v", err)
	}

	release()
	again, err := locks.acquire(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	again()
}
