package httpui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

const maxBodyBytes = 64 << 10

type requestIDKey struct{}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := randomHex(16)
		if err != nil {
			id = fmt.Sprintf("fallback-%d", time.Now().UnixNano())
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

func requestID(r *http.Request) string {
	id, _ := r.Context().Value(requestIDKey{}).(string)
	if id == "" {
		return "unavailable"
	}
	return id
}

type successEnvelope struct {
	Data      any    `json:"data"`
	RequestID string `json:"request_id"`
}

type errorEnvelope struct {
	Error     apiError `json:"error"`
	RequestID string   `json:"request_id"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type clientError struct {
	status  int
	code    string
	message string
}

func (e *clientError) Error() string { return e.message }

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeData(w http.ResponseWriter, r *http.Request, data any) {
	writeDataStatus(w, r, http.StatusOK, data)
}

func writeDataStatus(w http.ResponseWriter, r *http.Request, status int, data any) {
	writeJSON(w, status, successEnvelope{Data: data, RequestID: requestID(r)})
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{
		Error:     apiError{Code: code, Message: message},
		RequestID: requestID(r),
	})
}

func writeClientError(w http.ResponseWriter, r *http.Request, err *clientError) {
	writeError(w, r, err.status, err.code, err.message)
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request, allow string) {
	w.Header().Set("Allow", allow)
	writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "This method is not allowed.")
}

func onlyMethod(method string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			methodNotAllowed(w, r, method)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requireJSON(w http.ResponseWriter, r *http.Request) *clientError {
	ct := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		return &clientError{status: http.StatusBadRequest, code: "invalid_content_type", message: "Content-Type must be application/json."}
	}
	return nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) *clientError {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return &clientError{status: http.StatusBadRequest, code: "body_too_large", message: "Request body is too large."}
		}
		return &clientError{status: http.StatusBadRequest, code: "invalid_json", message: "Request body is not valid JSON or has unknown fields."}
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return &clientError{status: http.StatusBadRequest, code: "invalid_json", message: "Request body must contain exactly one JSON value."}
	}
	return nil
}
