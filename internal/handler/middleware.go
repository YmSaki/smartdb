package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
)

type ApiHandler[T any] func(w http.ResponseWriter, r *http.Request, req T)

// defaultMaxBodyBytes is the request body size limit documented in
// docs/spec.md §11 (デフォルト1MB).
const defaultMaxBodyBytes int64 = 1 << 20

var maxBodyBytes atomic.Int64

func init() {
	maxBodyBytes.Store(defaultMaxBodyBytes)
}

// SetMaxBodyBytes overrides the request body size limit enforced by
// HandleJson. Intended to be called once at startup from config
// (SDB_MAX_BODY_BYTES); safe for concurrent use since it's also used to
// reset the limit between tests.
func SetMaxBodyBytes(n int64) {
	maxBodyBytes.Store(n)
}

func HandleJson[T any](next ApiHandler[T]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes.Load())
		defer r.Body.Close()

		var req T
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				WriteError(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Invalid JSON"))
			return
		}
		next(w, r, req)
	}
}
