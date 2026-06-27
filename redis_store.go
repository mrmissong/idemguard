// This file handles Redis-backed storage for multi-instance applications.
package idemguard

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Addr      string
	Password  string
	DB        int
	KeyPrefix string
}

type RedisStore struct {
	client *redis.Client
	prefix string
}

// NewRedisStore creates a Redis store from simple connection settings.
func NewRedisStore(config RedisConfig) *RedisStore {
	return NewRedisStoreWithClient(redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
	}), config.KeyPrefix)
}

// NewRedisStoreWithClient lets applications reuse an existing Redis client.
func NewRedisStoreWithClient(client *redis.Client, prefix string) *RedisStore {
	if prefix == "" {
		prefix = "idempotency"
	}

	return &RedisStore{
		client: client,
		prefix: strings.TrimRight(prefix, ":"),
	}
}

// Start uses Redis SETNX to let only the first request for a key begin
// processing.
func (s *RedisStore) Start(ctx context.Context, key string, fingerprint string, ttl time.Duration) (StartResult, error) {
	now := time.Now()
	record := Record{
		Key:         key,
		Fingerprint: fingerprint,
		Status:      StatusProcessing,
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
	}

	encoded, err := json.Marshal(record)
	if err != nil {
		return StartResult{}, err
	}

	created, err := s.client.SetNX(ctx, s.redisKey(key), encoded, ttl).Result()
	if err != nil {
		return StartResult{}, err
	}

	if created {
		return StartResult{Status: StartStatusStarted, Record: record}, nil
	}

	existing, err := s.Get(ctx, key)
	if err != nil {
		return StartResult{}, err
	}

	if existing.Fingerprint != fingerprint {
		return StartResult{Status: StartStatusMismatch, Record: existing}, nil
	}

	switch existing.Status {
	case StatusCompleted:
		return StartResult{Status: StartStatusCompleted, Record: existing}, nil
	default:
		return StartResult{Status: StartStatusProcessing, Record: existing}, nil
	}
}

// Complete updates a Redis record to completed and stores the response for
// future retries.
func (s *RedisStore) Complete(ctx context.Context, key string, response StoredResponse) error {
	record, err := s.Get(ctx, key)
	if err != nil {
		return err
	}

	record.Status = StatusCompleted
	record.Response = response

	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}

	ttl := time.Until(record.ExpiresAt)
	if ttl <= 0 {
		return ErrRecordNotFound
	}

	return s.client.Set(ctx, s.redisKey(key), encoded, ttl).Err()
}

// Get loads an idempotency record from Redis.
func (s *RedisStore) Get(ctx context.Context, key string) (Record, error) {
	value, err := s.client.Get(ctx, s.redisKey(key)).Bytes()
	if err == redis.Nil {
		return Record{}, ErrRecordNotFound
	}
	if err != nil {
		return Record{}, err
	}

	var record Record
	if err := json.Unmarshal(value, &record); err != nil {
		return Record{}, err
	}

	return record, nil
}

// redisKey adds the configured namespace prefix to avoid collisions with other
// Redis keys.
func (s *RedisStore) redisKey(key string) string {
	return s.prefix + ":" + key
}
