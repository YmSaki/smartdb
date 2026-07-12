package auth

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"
)

func setupAuthMiddlewareTest(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS api_keys (
			id           TEXT PRIMARY KEY,
			project_id   TEXT,
			name         TEXT NOT NULL,
			token_hash   TEXT NOT NULL UNIQUE,
			role         TEXT NOT NULL CHECK (role IN ('system', 'admin', 'read_write', 'read_only')),
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			revoked_at   DATETIME
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS projects (
			id           TEXT PRIMARY KEY,
			name         TEXT NOT NULL,
			state        TEXT NOT NULL CHECK (state IN ('creating', 'inactive', 'active', 'deleting', 'deleted', 'wiped')) DEFAULT 'creating',
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("failed to create projects table: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func insertTestProject(t *testing.T, db *sql.DB, id string, state string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO projects (id, name, state) VALUES (?, ?, ?)`, id, id, state)
	if err != nil {
		t.Fatalf("failed to insert project %s: %v", id, err)
	}
}

func TestRequireAuthNoHeader(t *testing.T) {
	db := setupAuthMiddlewareTest(t)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	w := httptest.NewRecorder()

	handler := RequireAuth(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuthInvalidToken(t *testing.T) {
	db := setupAuthMiddlewareTest(t)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer invalid_token_hash")
	w := httptest.NewRecorder()

	handler := RequireAuth(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuthValidToken(t *testing.T) {
	db := setupAuthMiddlewareTest(t)

	// Create a valid key
	token := "sdb_valid_token_12345678"
	hash := HashToken(token)
	_, err := db.Exec(`
		INSERT INTO api_keys (id, project_id, name, token_hash, role, created_at, revoked_at)
		VALUES ('key-1', NULL, 'Test Key', ?, 'admin', datetime('now'), NULL)
	`, hash)
	if err != nil {
		t.Fatalf("failed to insert key: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler := RequireAuth(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequireAuthRevokedToken(t *testing.T) {
	db := setupAuthMiddlewareTest(t)

	// Create a revoked key
	token := "sdb_revoked_token_1234567"
	hash := HashToken(token)
	_, err := db.Exec(`
		INSERT INTO api_keys (id, project_id, name, token_hash, role, created_at, revoked_at)
		VALUES ('key-1', NULL, 'Revoked Key', ?, 'admin', datetime('now'), datetime('now'))
	`, hash)
	if err != nil {
		t.Fatalf("failed to insert key: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler := RequireAuth(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("revoked token should be rejected: got %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthContextInjected(t *testing.T) {
	db := setupAuthMiddlewareTest(t)

	// Create a valid key
	token := "sdb_context_token_1234567"
	hash := HashToken(token)
	_, err := db.Exec(`
		INSERT INTO api_keys (id, project_id, name, token_hash, role, created_at, revoked_at)
		VALUES ('key-1', NULL, 'Test Key', ?, 'admin', datetime('now'), NULL)
	`, hash)
	if err != nil {
		t.Fatalf("failed to insert key: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	var capturedCtx context.Context

	handler := RequireAuth(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("request failed: status %d", w.Code)
	}

	if capturedCtx == nil {
		t.Fatal("context is nil")
	}

	ac := GetAuthContext(capturedCtx)
	if ac == nil {
		t.Error("auth context not found")
	}
	if ac != nil && ac.Role != RoleAdmin {
		t.Errorf("role: got %s, want admin", ac.Role)
	}
}

func TestRequireSystemKeyWithProjectKey(t *testing.T) {
	db := setupAuthMiddlewareTest(t)

	// Create a project-scoped key
	token := "sdb_project_token_1234567"
	hash := HashToken(token)
	projectID := "test-project"
	_, err := db.Exec(`
		INSERT INTO api_keys (id, project_id, name, token_hash, role, created_at, revoked_at)
		VALUES ('key-1', ?, 'Project Key', ?, 'read_write', datetime('now'), NULL)
	`, projectID, hash)
	if err != nil {
		t.Fatalf("failed to insert key: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler := RequireSystemKey(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("project key on system endpoint: got %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRequireProjectAccessMismatch(t *testing.T) {
	db := setupAuthMiddlewareTest(t)

	// Create a key for project-a
	token := "sdb_proj_a_token_1234567"
	hash := HashToken(token)
	_, err := db.Exec(`
		INSERT INTO api_keys (id, project_id, name, token_hash, role, created_at, revoked_at)
		VALUES ('key-1', 'project-a', 'Project Key', ?, 'read_write', datetime('now'), NULL)
	`, hash)
	if err != nil {
		t.Fatalf("failed to insert key: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/projects/project-b/tables", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.SetPathValue("project", "project-b")
	w := httptest.NewRecorder()

	handler := RequireProjectAccess(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("cross-project access: got %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRequireProjectAccessWithSystemKey(t *testing.T) {
	db := setupAuthMiddlewareTest(t)

	// Create a system-level key
	token := "sdb_system_token_1234567"
	hash := HashToken(token)
	_, err := db.Exec(`
		INSERT INTO api_keys (id, project_id, name, token_hash, role, created_at, revoked_at)
		VALUES ('key-1', NULL, 'System Key', ?, 'system', datetime('now'), NULL)
	`, hash)
	if err != nil {
		t.Fatalf("failed to insert key: %v", err)
	}

	insertTestProject(t, db, "any-project", "active")

	// System key should access any project
	req := httptest.NewRequest("GET", "/api/projects/any-project/tables", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.SetPathValue("project", "any-project")
	w := httptest.NewRecorder()

	handler := RequireProjectAccess(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("system key access denied: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequireSystemKeyRejectsNonSystemRole(t *testing.T) {
	db := setupAuthMiddlewareTest(t)

	// A nil-ProjectID key that somehow isn't role=system (shouldn't happen
	// via the API, but the middleware must fail closed if it ever does).
	token := "sdb_admin_not_system_1234567"
	hash := HashToken(token)
	_, err := db.Exec(`
		INSERT INTO api_keys (id, project_id, name, token_hash, role, created_at, revoked_at)
		VALUES ('key-1', NULL, 'Not System', ?, 'admin', datetime('now'), NULL)
	`, hash)
	if err != nil {
		t.Fatalf("failed to insert key: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler := RequireSystemKey(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("non-system role on system endpoint: got %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRequireProjectAccessRejectsNonSystemNilProjectKey(t *testing.T) {
	db := setupAuthMiddlewareTest(t)

	token := "sdb_admin_not_system_7654321"
	hash := HashToken(token)
	_, err := db.Exec(`
		INSERT INTO api_keys (id, project_id, name, token_hash, role, created_at, revoked_at)
		VALUES ('key-1', NULL, 'Not System', ?, 'admin', datetime('now'), NULL)
	`, hash)
	if err != nil {
		t.Fatalf("failed to insert key: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/projects/any-project/tables", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.SetPathValue("project", "any-project")
	w := httptest.NewRecorder()

	handler := RequireProjectAccess(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("non-system nil-project key: got %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRequireProjectAccessDeletedOrWipedProjectReturns404(t *testing.T) {
	for _, state := range []string{"deleted", "wiped"} {
		t.Run(state, func(t *testing.T) {
			db := setupAuthMiddlewareTest(t)
			insertTestProject(t, db, "gone-project", state)

			token := "sdb_admin_" + state + "_1234567"
			hash := HashToken(token)
			_, err := db.Exec(`
				INSERT INTO api_keys (id, project_id, name, token_hash, role, created_at, revoked_at)
				VALUES ('key-1', 'gone-project', 'Project Key', ?, 'admin', datetime('now'), NULL)
			`, hash)
			if err != nil {
				t.Fatalf("failed to insert key: %v", err)
			}

			req := httptest.NewRequest("GET", "/api/projects/gone-project/tables", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			req.SetPathValue("project", "gone-project")
			w := httptest.NewRecorder()

			handler := RequireProjectAccess(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("%s project: got %d, want %d, body=%s", state, w.Code, http.StatusNotFound, w.Body.String())
			}
		})
	}
}

func TestRequireProjectAccessNonexistentProjectReturns404(t *testing.T) {
	db := setupAuthMiddlewareTest(t)

	// No row inserted into projects at all - the key references a
	// project_id that was never created (or was hard-deleted out from
	// under it).
	token := "sdb_admin_nonexistent_1234567"
	hash := HashToken(token)
	_, err := db.Exec(`
		INSERT INTO api_keys (id, project_id, name, token_hash, role, created_at, revoked_at)
		VALUES ('key-1', 'never-existed', 'Project Key', ?, 'admin', datetime('now'), NULL)
	`, hash)
	if err != nil {
		t.Fatalf("failed to insert key: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/projects/never-existed/tables", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.SetPathValue("project", "never-existed")
	w := httptest.NewRecorder()

	handler := RequireProjectAccess(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("nonexistent project: got %d, want %d, body=%s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

func TestRequireProjectAccessActiveAndInactiveProjectsAllowed(t *testing.T) {
	for _, state := range []string{"active", "inactive"} {
		t.Run(state, func(t *testing.T) {
			db := setupAuthMiddlewareTest(t)
			insertTestProject(t, db, "live-project", state)

			token := "sdb_admin_" + state + "_1234567"
			hash := HashToken(token)
			_, err := db.Exec(`
				INSERT INTO api_keys (id, project_id, name, token_hash, role, created_at, revoked_at)
				VALUES ('key-1', 'live-project', 'Project Key', ?, 'admin', datetime('now'), NULL)
			`, hash)
			if err != nil {
				t.Fatalf("failed to insert key: %v", err)
			}

			req := httptest.NewRequest("GET", "/api/projects/live-project/tables", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			req.SetPathValue("project", "live-project")
			w := httptest.NewRecorder()

			handler := RequireProjectAccess(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("%s project: got %d, want %d, body=%s", state, w.Code, http.StatusOK, w.Body.String())
			}
		})
	}
}

// TestRequireProjectAccessSystemKeyBypassesDeletedCheck guards the
// emergency-access invariant docs/spec.md §7 documents: System Key's
// project access (notably apikeys issue/revoke, see #14/#25) is
// unconditional and must not be blocked by the deleted/wiped check that
// applies to a project's own Project Key.
func TestRequireProjectAccessSystemKeyBypassesDeletedCheck(t *testing.T) {
	for _, state := range []string{"deleted", "wiped"} {
		t.Run(state, func(t *testing.T) {
			db := setupAuthMiddlewareTest(t)
			insertTestProject(t, db, "gone-project", state)

			token := "sdb_system_" + state + "_1234567"
			hash := HashToken(token)
			_, err := db.Exec(`
				INSERT INTO api_keys (id, project_id, name, token_hash, role, created_at, revoked_at)
				VALUES ('key-1', NULL, 'System Key', ?, 'system', datetime('now'), NULL)
			`, hash)
			if err != nil {
				t.Fatalf("failed to insert key: %v", err)
			}

			req := httptest.NewRequest("POST", "/api/projects/gone-project/apikeys", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			req.SetPathValue("project", "gone-project")
			w := httptest.NewRecorder()

			handler := RequireProjectAccess(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("system key on %s project: got %d, want %d, body=%s", state, w.Code, http.StatusOK, w.Body.String())
			}
		})
	}
}
