// This file captures handler output so the same response can be replayed later.
package idemguard

import (
	"bytes"
	"net/http"
)

type responseRecorder struct {
	header     http.Header
	body       bytes.Buffer
	statusCode int
}

// newResponseRecorder prepares a lightweight response writer that stores output
// instead of sending it immediately.
func newResponseRecorder() *responseRecorder {
	return &responseRecorder{
		header:     make(http.Header),
		statusCode: http.StatusOK,
	}
}

// Header returns the mutable response headers written by the wrapped handler.
func (r *responseRecorder) Header() http.Header {
	return r.header
}

// WriteHeader records the status code the handler wants to send.
func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

// Write records the response body the handler writes.
func (r *responseRecorder) Write(body []byte) (int, error) {
	return r.body.Write(body)
}

// storedResponse converts captured handler output into the format stores persist.
func (r *responseRecorder) storedResponse() StoredResponse {
	return StoredResponse{
		StatusCode: r.statusCode,
		Header:     r.header.Clone(),
		Body:       r.body.Bytes(),
	}
}

// writeStoredResponse writes a previously captured response back to the client.
func writeStoredResponse(w http.ResponseWriter, response StoredResponse) {
	for key, values := range response.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(response.Body)
}
