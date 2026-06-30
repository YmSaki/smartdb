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
			role         TEXT NOT NULL CHECK (role IN ('admin', 'read_write', 'read_only')),
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			revoked_at   DATETIME
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
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
		VALUES ('key-1', NULL, 'System Key', ?, 'admin', datetime('now'), NULL)
	`, hash)
	if err != nil {
		t.Fatalf("failed to insert key: %v", err)
	}

	// System key should access any project
	req := httptest.NewRequest("GET", "/api/projects/any-project/tables", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler := RequireProjectAccess(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("system key access denied: got %d, want %d", w.Code, http.StatusOK)
	}
}
