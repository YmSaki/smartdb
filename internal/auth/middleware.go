package auth

import (
	"context"
	"database/sql"
	"net/http"
	"smartdb/internal/handler"
	"strings"
)

type authContextKey struct{}

func GetAuthContext(ctx context.Context) *AuthContext {
	ac, _ := ctx.Value(authContextKey{}).(*AuthContext)
	return ac
}

func requireAuth(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearer(r)
		if token == "" {
			handler.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid Authorization header")
			return
		}

		hash := HashToken(token)
		key, err := GetKeyByHash(db, hash)
		if err != nil {
			handler.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid API key")
			return
		}

		ac := &AuthContext{
			KeyID:     key.ID,
			ProjectID: key.ProjectID,
			Role:      key.Role,
		}
		ctx := context.WithValue(r.Context(), authContextKey{}, ac)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireAuth(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return requireAuth(db, next)
	}
}

func RequireSystemKey(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return requireAuth(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac := GetAuthContext(r.Context())
			if ac == nil || ac.ProjectID != nil {
				handler.WriteError(w, http.StatusForbidden, "FORBIDDEN", "System admin key required")
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}

func RequireProjectAccess(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return requireAuth(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac := GetAuthContext(r.Context())
			if ac == nil {
				handler.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
				return
			}
			if ac.ProjectID == nil {
				next.ServeHTTP(w, r)
				return
			}
			projectID := r.PathValue("project")
			if projectID != "" && *ac.ProjectID != projectID {
				handler.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Access denied for this project")
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}
