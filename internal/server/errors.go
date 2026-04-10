// Package server implements the HTTP layer for dbseer, composing the db,
// wire, safety, discover, and ui packages into a chi router.
package server

import (
	"encoding/json"
	"net/http"
)

// ErrorBody is the shape of every error response body.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  any    `json:"detail,omitempty"`
}

// errorEnvelope wraps ErrorBody in the {"error": ...} envelope per api.md.
type errorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// writeError writes a JSON error response with the given HTTP status code.
func writeError(w http.ResponseWriter, status int, code, message string, detail any) {
	writeJSON(w, status, errorEnvelope{
		Error: ErrorBody{
			Code:    code,
			Message: message,
			Detail:  detail,
		},
	})
}

// writeJSON encodes body as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// ErrorCode constants for error responses.
const (
	ErrCodeInvalidRequest   = "invalid_request"
	ErrCodeNotFound         = "not_found"
	ErrCodeUnscopedMutation = "unscoped_mutation"
	ErrCodeTableReadonly    = "table_readonly"
	ErrCodeServerReadonly   = "server_readonly"
	ErrCodeDBError          = "db_error"
	ErrCodeInternal         = "internal"
	ErrCodePayloadTooLarge  = "payload_too_large"
)
