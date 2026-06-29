package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

// RequestIDKey is the context key for the request ID.
const RequestIDKey contextKey = "request_id"

// RequestIDMiddleware generates or passes through an X-Request-ID header,
// sets it on the response, and adds it to the request context and slog.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}

		w.Header().Set("X-Request-ID", id)

		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		slog.With("request_id", id).Debug("incoming request", "method", r.Method, "path", r.URL.Path)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
