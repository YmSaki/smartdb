package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"smartdb/internal/domain"
	"smartdb/internal/handler"
	"smartdb/internal/project"
	"strings"
)

type authContextKey struct{}

func GetAuthContext(ctx context.Context) *AuthContext {
	ac, _ := ctx.Value(authContextKey{}).(*AuthContext)
	return ac
}

// WithAuthContext attaches an AuthContext to ctx the same way requireAuth
// does. Mainly useful for tests that exercise a handler directly without
// going through the full RequireAuth/RequireProjectAccess middleware chain.
func WithAuthContext(ctx context.Context, ac *AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey{}, ac)
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
			if ac == nil || ac.ProjectID != nil || ac.Role != RoleSystem {
				handler.WriteError(w, http.StatusForbidden, "FORBIDDEN", "System key required")
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
				// Only a genuine system key (role=system) gets a universal
				// pass across all projects. Anything else with a nil
				// ProjectID would be a data invariant violation; fail closed.
				//
				// System Key's pass is intentionally unconditional, even
				// for deleted/wiped projects: docs/spec.md §7 documents its
				// apikeys issue/revoke access as an always-available
				// emergency path with no state check (see #14/#25). The
				// deleted/wiped block below is for the Project Key path
				// only - it must not re-impose a state check System Key
				// was deliberately exempted from.
				if ac.Role != RoleSystem {
					handler.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Access denied for this project")
					return
				}
			} else {
				projectID := r.PathValue("project")
				if projectID != "" && *ac.ProjectID != projectID {
					handler.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Access denied for this project")
					return
				}

				// A deleted/wiped project is treated as if it no longer
				// exists to its own Project Key: without this, a key
				// issued before DELETE keeps full SQL/Backup/Restore/
				// API-key access until wipe actually runs (see #26).
				if projectID != "" {
					proj, err := project.GetProject(db, projectID)
					if err != nil {
						if errors.Is(err, sql.ErrNoRows) {
							handler.WriteError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project does not exist")
							return
						}
						handler.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
						return
					}
					if proj.State == domain.StateDeleted || proj.State == domain.StateWiped {
						handler.WriteError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project does not exist")
						return
					}
				}
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
