// This file handles the in-process store for tests, local usage, and small
// single-instance applications.
package idemguard

import (
	"context"
	"sync"
	"time"
)

type MemoryStore struct {
	mu         sync.RWMutex
	records    map[string]Record
	maxRecords int
}

// NewMemoryStore creates an unlimited in-memory store.
func NewMemoryStore() *MemoryStore {
	return NewMemoryStoreWithLimit(0)
}

// NewMemoryStoreWithLimit creates an in-memory store that stops accepting new
// keys after maxRecords, which helps small apps avoid unbounded memory growth.
func NewMemoryStoreWithLimit(maxRecords int) *MemoryStore {
	return &MemoryStore{
		records:    make(map[string]Record),
		maxRecords: maxRecords,
	}
}

// Start checks whether a key can begin processing, should replay, is still
// running, or was reused with different request data.
func (s *MemoryStore) Start(ctx context.Context, key string, fingerprint string, ttl time.Duration) (StartResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	record, ok := s.records[key]
	if ok && now.After(record.ExpiresAt) {
		delete(s.records, key)
		ok = false
	}

	if ok {
		if record.Fingerprint != fingerprint {
			return StartResult{Status: StartStatusMismatch, Record: record}, nil
		}

		switch record.Status {
		case StatusCompleted:
			return StartResult{Status: StartStatusCompleted, Record: record}, nil
		default:
			return StartResult{Status: StartStatusProcessing, Record: record}, nil
		}
	}

	if s.maxRecords > 0 && len(s.records) >= s.maxRecords {
		return StartResult{}, ErrMemoryStoreFull
	}

	record = Record{
		Key:         key,
		Fingerprint: fingerprint,
		Status:      StatusProcessing,
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
	}
	s.records[key] = record

	return StartResult{Status: StartStatusStarted, Record: record}, nil
}

// Complete marks a processing key as finished and stores the response that
// future retries should receive.
func (s *MemoryStore) Complete(ctx context.Context, key string, response StoredResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.records[key]
	if !ok {
		return ErrRecordNotFound
	}

	record.Status = StatusCompleted
	record.Response = response
	s.records[key] = record

	return nil
}

// Get returns a stored idempotency record if it exists and has not expired.
func (s *MemoryStore) Get(ctx context.Context, key string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.records[key]
	if !ok || time.Now().After(record.ExpiresAt) {
		return Record{}, ErrRecordNotFound
	}

	return record, nil
}
