// This file keeps all user-facing setup knobs for the middleware in one place.
package idemguard

import (
	"errors"
	"net/http"
	"time"
)

const (
	defaultHeaderName = "Idg-Key"
	defaultTTL        = 24 * time.Hour
)

// Config describes how the middleware should identify requests, choose storage,
// and respond when duplicate traffic comes in.
type Config struct {
	Production bool

	Store Store

	Redis *RedisConfig

	HeaderName string

	TTL time.Duration

	ScopeFields []string

	FingerprintFields []string

	InProgressStatusCode int

	MismatchStatusCode int

	MissingKeyStatusCode int
}

// withDefaults fills in the practical defaults so users can configure only the
// parts they care about.
func (c Config) withDefaults() (Config, error) {
	if c.HeaderName == "" {
		c.HeaderName = defaultHeaderName
	}

	if c.TTL == 0 {
		c.TTL = defaultTTL
	}

	if c.Store == nil && c.Production {
		if c.Redis == nil {
			return Config{}, errors.New("redis config is required when production is true and store is nil")
		}

		c.Store = NewRedisStore(*c.Redis)
	}

	if c.Store == nil && !c.Production {
		c.Store = NewMemoryStore()
	}

	if c.InProgressStatusCode == 0 {
		c.InProgressStatusCode = http.StatusConflict
	}

	if c.MismatchStatusCode == 0 {
		c.MismatchStatusCode = http.StatusConflict
	}

	if c.MissingKeyStatusCode == 0 {
		c.MissingKeyStatusCode = http.StatusBadRequest
	}

	return c, nil
}
