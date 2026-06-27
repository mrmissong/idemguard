# IdemGuard

`idemguard` is a Go HTTP middleware for protecting retry-prone APIs from duplicate side effects.

Use it when a request should be safe to retry, but should only execute once:

- payment creation
- order creation
- shipment booking
- webhook processing
- reward distribution
- any `POST`, `PUT`, or `PATCH` flow where duplicate execution can hurt

The client sends an `Idg-Key`, `idemguard` stores the first response, and later matching retries receive the same response without running the handler again.

## Features

- `net/http` middleware
- default header: `Idg-Key`
- response replay for completed duplicate requests
- in-progress duplicate protection
- fingerprint mismatch detection
- configurable scope/context fields
- configurable fingerprint fields
- nested JSON body extraction with `gjson`
- support for body, header, query, and path values
- in-memory store for tests, local dev, and single-instance apps
- Redis store for multi-instance production apps
- TTL-based record expiry
- optional in-memory max record limit
- custom status codes for missing key, mismatch, and in-progress requests

## Install

```bash
go get github.com/mrmissong/idemguard
```

## Quick Idea

```text
same Idg-Key + same fingerprint      = return saved response
same Idg-Key + different fingerprint = reject as conflict
new Idg-Key or new scope             = run as new operation
```

`Idg-Key` identifies the operation.

The fingerprint verifies that a retry still matches the original request.

Scope fields help separate different operation contexts, such as different carts, checkout sessions, tenants, or orders.

## Configure In `main.go`

In Go applications, global setup usually happens in `main.go`: create stores, configure middleware, register routes, then start the server.

```go
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/mrmissong/idemguard"
)

func main() {
	idem := idemguard.MustNew(idemguard.Config{
		Store:      idemguard.NewMemoryStoreWithLimit(10000),
		HeaderName: "Idg-Key",
		TTL:        24 * time.Hour,
		ScopeFields: []string{
			"body.cart_id",
		},
		FingerprintFields: []string{
			"body.cart_id",
			"body.amount",
			"body.currency",
		},
	})

	mux := http.NewServeMux()
	mux.Handle("POST /payments", idem.Middleware(http.HandlerFunc(createPayment)))

	log.Fatal(http.ListenAndServe(":8080", mux))
}

func createPayment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"payment_id": "pay_123",
		"status":     "created",
	})
}
```

## Request Example

```bash
curl -X POST http://localhost:8080/payments \
  -H "Content-Type: application/json" \
  -H "Idg-Key: checkout_abc123" \
  -d '{
    "cart_id": "cart_1",
    "amount": 200,
    "currency": "USD"
  }'
```

Send the same request again with the same `Idg-Key`; the saved response is returned and the handler is not executed again.

## Scope Fields

Scope fields are combined with `Idg-Key` to create the internal storage key.

```go
ScopeFields: []string{
	"body.cart_id",
	"header.X-Tenant-ID",
}
```

This helps avoid accidental collisions when the same raw key appears in different operation contexts.

Example:

```text
hash(Idg-Key + cart_id)
```

So the same raw `Idg-Key` can still represent different operations if the scope changes.

## Fingerprint Fields

Fingerprint fields define what makes the request content the same.

```go
FingerprintFields: []string{
	"body.payment.amount",
	"body.payment.currency",
	"query.region",
	"header.X-Tenant-ID",
	"path.user_id",
}
```

If `FingerprintFields` is empty, `idemguard` uses:

```text
method + path + full body hash
```

Nested JSON is supported:

```text
body.payment.amount
body.items.0.product_id
body.items.#.product_id
```

## Path Params

`idemguard` stays router-neutral. If your router has path params, attach them to the request before the middleware checks fields:

```go
r = idemguard.WithPathParam(r, "user_id", "42")
```

Then use:

```go
FingerprintFields: []string{"path.user_id"}
```

## Storage Options

### In-Memory

Use memory for tests, local development, demos, and small single-instance applications.

```go
Store: idemguard.NewMemoryStoreWithLimit(10000)
```

Memory storage is not shared between app instances. If your service runs behind a load balancer with multiple instances, use Redis.

### Redis

Use Redis for production systems with multiple app instances.

```go
idem := idemguard.MustNew(idemguard.Config{
	Production: true,
	Redis: &idemguard.RedisConfig{
		Addr:      "localhost:6379",
		Password:  "",
		DB:        0,
		KeyPrefix: "payments",
	},
	TTL: 24 * time.Hour,
})
```

You can also reuse an existing Redis client:

```go
store := idemguard.NewRedisStoreWithClient(redisClient, "payments")

idem := idemguard.MustNew(idemguard.Config{
	Store: store,
})
```

## Demo APIs

Run the example app:

```bash
go run ./examples/three-apis
```

It starts:

```text
http://localhost:8080
```

Available endpoints:

```text
POST /payments
POST /orders
POST /shipments
```

Example payment request:

```json
{
  "cart_id": "cart_1",
  "amount": 200,
  "currency": "USD"
}
```

Use this header:

```text
Idg-Key: demo-key-1
```

## Tests

```bash
go test ./...
```

## License

MIT

