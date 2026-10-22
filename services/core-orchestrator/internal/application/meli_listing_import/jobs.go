package meli_listing_import

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"
)

// Job states. A job is "done" when the import ran to the end, whatever its
// steps reported (Result.Linked and Result.Steps say how it went); "failed" is
// reserved for an import that couldn't finish at all (a panic, or the service
// restarting mid-way).
const (
	JobRunning = "running"
	JobDone    = "done"
	JobFailed  = "failed"
)

// staleJobAfter is how long a "running" job may go without an update before it
// is reported as failed: its goroutine died with the process.
const staleJobAfter = importTimeout + time.Minute

var (
	ErrJobNotFound = errors.New("import job not found")
	ErrJobStore    = errors.New("could not save the import job")
)

// JobStore keeps the progress of imports, shared by every replica of the service
// so any of them can answer a status request. It stores opaque payloads and
// expires them on its own (see infrastructure/redis.ImportJobStore).
type JobStore interface {
	Save(ctx context.Context, jobID string, payload []byte) error
	// Load reports found=false when there is no such job (never saved, or expired).
	Load(ctx context.Context, jobID string) (payload []byte, found bool, err error)
}

// Job is an import in progress or finished. The dashboard polls it, which is
// what lets an import take longer than any proxy in front of the API (nginx,
// Cloudflare's 100 s limit) is willing to wait for a single response.
type Job struct {
	ID           string    `json:"jobId"`
	ProductID    int64     `json:"productId"`
	ConnectionID int64     `json:"connectionId"`
	Status       string    `json:"status"`
	Error        string    `json:"error,omitempty"`
	Result       *Result   `json:"result,omitempty"`
	StartedAt    time.Time `json:"startedAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func newJobID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("error generating job id: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// snapshot returns a copy that shares nothing with the job the import goroutine
// keeps mutating, so it can be handed to an HTTP handler safely.
func (j *Job) snapshot() *Job {
	copied := *j
	if j.Result != nil {
		result := *j.Result
		result.Steps = append([]StepResult(nil), j.Result.Steps...)
		copied.Result = &result
	}
	return &copied
}

// save persists the job's current state. Failing to save progress is logged and
// never interrupts the import itself.
func (s *Service) saveJob(job *Job) {
	job.UpdatedAt = time.Now()
	payload, err := json.Marshal(job)
	if err != nil {
		log.Printf("meli listing import: job %s: error encoding progress: %v", job.ID, err)
		return
	}

	// A fresh short context: the import's own may already be expired or cancelled
	// by the time the final state is saved, and that save is the one that matters.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.jobs.Save(ctx, job.ID, payload); err != nil {
		log.Printf("meli listing import: job %s: error saving progress: %v", job.ID, err)
	}
}

// GetJob returns the import job jobID of productID. A job that claims to be
// running but hasn't been updated for longer than any import can last is
// reported as failed: the process that was running it is gone.
func (s *Service) GetJob(ctx context.Context, productID int64, jobID string) (*Job, error) {
	payload, found, err := s.jobs.Load(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("error loading import job %s: %w", jobID, err)
	}
	if !found {
		return nil, ErrJobNotFound
	}

	var job Job
	if err := json.Unmarshal(payload, &job); err != nil {
		return nil, fmt.Errorf("error decoding import job %s: %w", jobID, err)
	}
	if job.ProductID != productID {
		return nil, ErrJobNotFound
	}

	if job.Status == JobRunning && time.Since(job.UpdatedAt) > staleJobAfter {
		job.Status = JobFailed
		job.Error = "La importación se interrumpió (el servicio se reinició mientras corría). Es seguro volver a intentarla."
	}
	return &job, nil
}
