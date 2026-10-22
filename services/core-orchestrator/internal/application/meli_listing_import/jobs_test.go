package meli_listing_import

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type memoryJobStore map[string][]byte

func (m memoryJobStore) Save(_ context.Context, id string, payload []byte) error {
	m[id] = payload
	return nil
}

func (m memoryJobStore) Load(_ context.Context, id string) ([]byte, bool, error) {
	payload, ok := m[id]
	return payload, ok, nil
}

func storeJob(t *testing.T, store memoryJobStore, job Job) {
	t.Helper()
	payload, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	store[job.ID] = payload
}

func TestGetJob(t *testing.T) {
	store := memoryJobStore{}
	service := &Service{jobs: store}
	ctx := context.Background()

	storeJob(t, store, Job{ID: "fresh", ProductID: 7, Status: JobRunning, UpdatedAt: time.Now()})
	storeJob(t, store, Job{ID: "stale", ProductID: 7, Status: JobRunning, UpdatedAt: time.Now().Add(-staleJobAfter - time.Minute)})
	storeJob(t, store, Job{ID: "done", ProductID: 7, Status: JobDone, UpdatedAt: time.Now().Add(-staleJobAfter - time.Hour)})

	if job, err := service.GetJob(ctx, 7, "fresh"); err != nil || job.Status != JobRunning {
		t.Errorf("un job reciente debe seguir running: %+v, %v", job, err)
	}
	if job, err := service.GetJob(ctx, 7, "stale"); err != nil || job.Status != JobFailed || job.Error == "" {
		t.Errorf("un job running sin actualizar debe reportarse como failed con motivo: %+v, %v", job, err)
	}
	if job, err := service.GetJob(ctx, 7, "done"); err != nil || job.Status != JobDone {
		t.Errorf("un job terminado nunca se marca como interrumpido: %+v, %v", job, err)
	}
	if _, err := service.GetJob(ctx, 8, "fresh"); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("un job de otro producto no debe verse: %v", err)
	}
	if _, err := service.GetJob(ctx, 7, "missing"); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("un job inexistente: %v", err)
	}
}

// El snapshot que recibe el handler no comparte pasos con el job que la
// goroutine de la importación sigue modificando.
func TestJobSnapshotIsIndependent(t *testing.T) {
	job := &Job{Result: &Result{Steps: []StepResult{{Step: StepName, Status: StatusOK}}}}
	snapshot := job.snapshot()

	job.Result.Steps = append(job.Result.Steps, StepResult{Step: StepCategory})
	job.Result.Steps[0].Status = StatusError

	if len(snapshot.Result.Steps) != 1 || snapshot.Result.Steps[0].Status != StatusOK {
		t.Errorf("el snapshot cambió: %+v", snapshot.Result.Steps)
	}
}
