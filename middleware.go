// This file connects HTTP requests to idempotency storage and response replay.
package idemguard

import (
	"bytes"
	"io"
	"net/http"
)

// Middleware wraps a handler so matching retries return the first saved response
// instead of running the handler again.
func (i *Idempotency) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idempotencyKey := r.Header.Get(i.config.HeaderName)
		if idempotencyKey == "" {
			http.Error(w, "missing idempotency key", i.config.MissingKeyStatusCode)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))

		storageKey := buildStorageKey(r, idempotencyKey, body, i.config.ScopeFields)
		fingerprint := buildFingerprint(r, body, i.config.FingerprintFields)

		result, err := i.config.Store.Start(r.Context(), storageKey, fingerprint, i.config.TTL)
		if err != nil {
			http.Error(w, "idempotency store error", http.StatusInternalServerError)
			return
		}

		switch result.Status {
		case StartStatusCompleted:
			writeStoredResponse(w, result.Record.Response)
			return
		case StartStatusProcessing:
			http.Error(w, "request already processing", i.config.InProgressStatusCode)
			return
		case StartStatusMismatch:
			http.Error(w, "idempotency key reused with different request", i.config.MismatchStatusCode)
			return
		}

		recorder := newResponseRecorder()
		next.ServeHTTP(recorder, r)

		response := recorder.storedResponse()
		if err := i.config.Store.Complete(r.Context(), storageKey, response); err != nil {
			http.Error(w, "idempotency store error", http.StatusInternalServerError)
			return
		}

		writeStoredResponse(w, response)
	})
}
