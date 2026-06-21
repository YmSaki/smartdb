package handler

import (
	"encoding/json"
	"net/http"
)

type ApiHandler[T any] func(w http.ResponseWriter, r *http.Request, req T)

func HandleJson[T any](next ApiHandler[T]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req T
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Invalid JSON"))
			return
		}
		next(w, r, req)
	}
}
