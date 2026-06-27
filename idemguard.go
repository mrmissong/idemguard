// This file exposes the small public entry point users create before wiring middleware.
package idemguard

type Idempotency struct {
	config Config
}

// New prepares an idempotency middleware instance and validates the storage
// setup before requests start flowing.
func New(config Config) (*Idempotency, error) {
	config, err := config.withDefaults()
	if err != nil {
		return nil, err
	}

	return &Idempotency{
		config: config,
	}, nil
}

// MustNew is the panic-on-error version for examples, tests, or apps that want
// startup to fail immediately on bad config.
func MustNew(config Config) *Idempotency {
	idem, err := New(config)
	if err != nil {
		panic(err)
	}

	return idem
}
