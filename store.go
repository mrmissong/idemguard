// This file defines the shared storage contract used by memory and Redis stores.
package idemguard

import (
	"context"
	"errors"
	"net/http"
	"time"
)

var (
	ErrRecordNotFound       = errors.New("idempotency record not found")
	ErrRecordAlreadyStarted = errors.New("idempotency record already started")
	ErrMemoryStoreFull      = errors.New("idempotency memory store full")
)

type RecordStatus string

const (
	StatusProcessing RecordStatus = "processing"
	StatusCompleted  RecordStatus = "completed"
	StatusFailed     RecordStatus = "failed"
)

type StartStatus string

const (
	StartStatusStarted    StartStatus = "started"
	StartStatusProcessing StartStatus = "processing"
	StartStatusCompleted  StartStatus = "completed"
	StartStatusMismatch   StartStatus = "mismatch"
)

type StartResult struct {
	Status StartStatus
	Record Record
}

type Record struct {
	Key         string
	Fingerprint string
	Status      RecordStatus
	Response    StoredResponse
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

type StoredResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// Store is the interface any storage backend must implement to participate in
// the idempotency flow.
type Store interface {
	Start(ctx context.Context, key string, fingerprint string, ttl time.Duration) (StartResult, error)
	Complete(ctx context.Context, key string, response StoredResponse) error
	Get(ctx context.Context, key string) (Record, error)
}
