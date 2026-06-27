// This example shows a minimal payment API using the idempotency middleware.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	idemguard "github.com/mrmissong/idemguard"
)

// main wires application-level dependencies, middleware, routes, and the HTTP
// server, which is the usual Go equivalent of a global app bootstrap file.
func main() {
	production := os.Getenv("APP_ENV") == "production"

	idem, err := idemguard.New(idemguard.Config{
		Production: production,
		Redis: &idemguard.RedisConfig{
			Addr:      env("REDIS_ADDR", "localhost:6379"),
			Password:  os.Getenv("REDIS_PASSWORD"),
			DB:        0,
			KeyPrefix: "payments",
		},
		HeaderName: "Idg-Key",
		TTL:        24 * time.Hour,
		ScopeFields: []string{
			"body.cart_id",
		},
		FingerprintFields: []string{
			"body.cart_id",
			"body.amount",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("POST /payments", idem.Middleware(http.HandlerFunc(createPayment)))

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

// createPayment represents the side-effecting handler that should only run once
// per idempotency key and scope.
func createPayment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"payment_id": "pay_123",
		"status":     "created",
	})
}

// env reads optional environment config with a local-development fallback.
func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
