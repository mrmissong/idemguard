package idemguard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMiddlewareReplaysCompletedResponse(t *testing.T) {
	var calls int64
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt64(&calls, 1)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"payment_id": count,
			"status":     "created",
		})
	})

	idem := MustNew(Config{
		Store: NewMemoryStore(),
		TTL:   time.Hour,
	})
	server := idem.Middleware(handler)

	first := performRequest(server, "abc", `{"amount":200,"cart_id":"cart_1"}`)
	second := performRequest(server, "abc", `{"cart_id":"cart_1","amount":200}`)

	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusCreated)
	}
	if second.Code != http.StatusCreated {
		t.Fatalf("second status = %d, want %d", second.Code, http.StatusCreated)
	}
	if first.Body.String() != second.Body.String() {
		t.Fatalf("second response should replay first response\nfirst: %s\nsecond: %s", first.Body.String(), second.Body.String())
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d, want 1", calls)
	}
}

func TestMiddlewareRejectsFingerprintMismatch(t *testing.T) {
	var calls int64
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})

	idem := MustNew(Config{
		Store: NewMemoryStore(),
		TTL:   time.Hour,
	})
	server := idem.Middleware(handler)

	first := performRequest(server, "abc", `{"amount":200,"cart_id":"cart_1"}`)
	second := performRequest(server, "abc", `{"amount":500,"cart_id":"cart_1"}`)

	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusCreated)
	}
	if second.Code != http.StatusConflict {
		t.Fatalf("second status = %d, want %d", second.Code, http.StatusConflict)
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d, want 1", calls)
	}
}

func TestMiddlewareScopesStorageKey(t *testing.T) {
	var calls int64
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt64(&calls, 1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(fmt.Sprintf("created-%d", count)))
	})

	idem := MustNew(Config{
		Store:       NewMemoryStore(),
		TTL:         time.Hour,
		ScopeFields: []string{"body.cart_id"},
	})
	server := idem.Middleware(handler)

	first := performRequest(server, "abc", `{"amount":200,"cart_id":"cart_1"}`)
	second := performRequest(server, "abc", `{"amount":200,"cart_id":"cart_2"}`)

	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusCreated)
	}
	if second.Code != http.StatusCreated {
		t.Fatalf("second status = %d, want %d", second.Code, http.StatusCreated)
	}
	if calls != 2 {
		t.Fatalf("handler calls = %d, want 2", calls)
	}
}

func TestMiddlewareReturnsProcessingForInProgressRecord(t *testing.T) {
	store := NewMemoryStore()
	idem := MustNew(Config{
		Store: store,
		TTL:   time.Hour,
	})
	server := idem.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run for in-progress duplicate")
	}))

	req := httptest.NewRequest(http.MethodPost, "/payments", strings.NewReader(`{"amount":200}`))
	req.Header.Set("Idg-Key", "abc")
	body := []byte(`{"amount":200}`)
	storageKey := buildStorageKey(req, "abc", body, nil)
	fingerprint := buildFingerprint(req, body, nil)

	if _, err := store.Start(req.Context(), storageKey, fingerprint, time.Hour); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
}

func TestMiddlewareRequiresIdempotencyKey(t *testing.T) {
	idem := MustNew(Config{})
	server := idem.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run without idempotency key")
	}))

	req := httptest.NewRequest(http.MethodPost, "/payments", strings.NewReader(`{"amount":200}`))
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestExtractBodyFieldSupportsNestedArrays(t *testing.T) {
	body := []byte(`{
		"payment": {
			"items": [
				{"product_id": "p1", "amount": 100},
				{"product_id": "p2", "amount": 200}
			]
		}
	}`)

	got := extractBodyField(body, "payment.items.1.product_id")
	want := `"p2"`
	if got != want {
		t.Fatalf("nested array value = %s, want %s", got, want)
	}
}

// performRequest keeps the HTTP setup in tests short and consistent.
func performRequest(handler http.Handler, key string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/payments", strings.NewReader(body))
	req.Header.Set("Idg-Key", key)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}
