// This file builds storage keys and request fingerprints from headers, query
// params, path params, and JSON body fields.
package idemguard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/pretty"
)

type pathParamsContextKey struct{}

// WithPathParam attaches one router path value to the request for later
// fingerprint or scope extraction.
func WithPathParam(r *http.Request, key string, value string) *http.Request {
	params := PathParams(r)
	params[key] = value
	return r.WithContext(context.WithValue(r.Context(), pathParamsContextKey{}, params))
}

// WithPathParams attaches multiple router path values to the request at once.
func WithPathParams(r *http.Request, params map[string]string) *http.Request {
	current := PathParams(r)
	for key, value := range params {
		current[key] = value
	}

	return r.WithContext(context.WithValue(r.Context(), pathParamsContextKey{}, current))
}

// PathParams reads a copy of the path values previously attached to the request.
func PathParams(r *http.Request) map[string]string {
	params, ok := r.Context().Value(pathParamsContextKey{}).(map[string]string)
	if !ok {
		return map[string]string{}
	}

	copied := make(map[string]string, len(params))
	for key, value := range params {
		copied[key] = value
	}

	return copied
}

// buildStorageKey creates the internal lookup key from the client key plus any
// configured scope fields.
func buildStorageKey(r *http.Request, idempotencyKey string, body []byte, fields []string) string {
	if len(fields) == 0 {
		return idempotencyKey
	}

	parts := []string{idempotencyKey}
	for _, field := range fields {
		parts = append(parts, extractField(r, body, field))
	}

	return hashParts(parts)
}

// buildFingerprint creates the request identity used to reject reused keys with
// different request content.
func buildFingerprint(r *http.Request, body []byte, fields []string) string {
	parts := []string{r.Method, r.URL.Path}

	if len(fields) == 0 {
		parts = append(parts, hashBytes(canonicalJSON(body)))
		return hashParts(parts)
	}

	for _, field := range fields {
		parts = append(parts, extractField(r, body, field))
	}

	return hashParts(parts)
}

// extractField pulls a configured value from headers, query params, body JSON,
// or path params.
func extractField(r *http.Request, body []byte, field string) string {
	switch {
	case strings.HasPrefix(field, "header."):
		return r.Header.Get(strings.TrimPrefix(field, "header."))
	case strings.HasPrefix(field, "query."):
		return r.URL.Query().Get(strings.TrimPrefix(field, "query."))
	case strings.HasPrefix(field, "body."):
		return extractBodyField(body, strings.TrimPrefix(field, "body."))
	case strings.HasPrefix(field, "path."):
		return PathParams(r)[strings.TrimPrefix(field, "path.")]
	default:
		return ""
	}
}

// extractBodyField reads one JSON value using gjson path syntax.
func extractBodyField(body []byte, path string) string {
	result := gjson.GetBytes(body, path)
	if !result.Exists() {
		return ""
	}

	if result.Raw != "" {
		return string(canonicalJSON([]byte(result.Raw)))
	}

	return result.String()
}

// canonicalJSON makes equivalent JSON stable before hashing, especially when
// object keys arrive in a different order.
func canonicalJSON(value []byte) []byte {
	if !json.Valid(value) {
		return value
	}

	sorted := pretty.PrettyOptions(value, &pretty.Options{
		SortKeys: true,
	})
	return pretty.Ugly(sorted)
}

// hashBytes returns the SHA-256 hex digest for raw bytes.
func hashBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

// hashString returns the SHA-256 hex digest for a string.
func hashString(value string) string {
	return hashBytes([]byte(value))
}

// hashParts hashes multiple values safely without relying on fragile string
// separators.
func hashParts(parts []string) string {
	encoded, err := json.Marshal(parts)
	if err != nil {
		return hashString(strings.Join(parts, "\x00"))
	}

	return hashBytes(encoded)
}
