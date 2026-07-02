package v1

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"smartdb/internal/auth"
	"smartdb/internal/config"
	"smartdb/internal/domain"

	_ "modernc.org/sqlite"
)

func setupAPIKeyTestApp(t *testing.T) *domain.App {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE api_keys (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			name TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			role TEXT NOT NULL CHECK (role IN ('system', 'admin', 'read_write', 'read_only')),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			revoked_at DATETIME
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	return &domain.App{
		SystemDB:     db,
		Config:       config.LoadDefaults(),
		ProjectLocks: domain.NewProjectLockRegistry(),
	}
}

func TestCreateAPIKeyEndpoint(t *testing.T) {
	app := setupAPIKeyTestApp(t)

	tests := []struct {
		name           string
		body           string
		expectedStatus int
		checkResponse  bool
	}{
		{
			name:           "POST create new API key",
			body:           `{"name":"Test Key","role":"read_write"}`,
			expectedStatus: http.StatusCreated,
			checkResponse:  true,
		},
		{
			name:           "POST with empty name",
			body:           `{"name":"","role":"admin"}`,
			expectedStatus: http.StatusBadRequest,
			checkResponse:  false,
		},
		{
			name:           "POST with invalid role",
			body:           `{"name":"Test Key","role":"superuser"}`,
			expectedStatus: http.StatusBadRequest,
			checkResponse:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/projects/test-project/apikeys", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("project", "test-project")
			w := httptest.NewRecorder()

			handler := CreateAPIKeyHandler(app)
			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("status code: got %d, want %d", w.Code, tt.expectedStatus)
			}

			if tt.checkResponse && tt.expectedStatus == http.StatusCreated {
				var resp CreateAPIKeyResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.ID == "" {
					t.Error("id is empty")
				}
				if resp.Token == "" {
					t.Error("token is empty")
				}
				if !strings.HasPrefix(resp.Token, "sdb_") {
					t.Errorf("token prefix: got %q, want sdb_", resp.Token[:4])
				}
			}
		})
	}
}

func TestListAPIKeysEndpoint(t *testing.T) {
	app := setupAPIKeyTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/projects/test-project/apikeys", nil)
	req.SetPathValue("project", "test-project")
	w := httptest.NewRecorder()

	handler := ListAPIKeysHandler(app)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestListAPIKeysNoTokenHash(t *testing.T) {
	app := setupAPIKeyTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/projects/test-project/apikeys", nil)
	req.SetPathValue("project", "test-project")
	w := httptest.NewRecorder()

	ListAPIKeysHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code: got %d", w.Code)
	}

	body := w.Body.String()
	if strings.Contains(body, "token_hash") {
		t.Error("response contains token_hash")
	}
	if strings.Contains(body, "tokenHash") {
		t.Error("response contains tokenHash")
	}
}

func TestRevokeAPIKeyEndpoint(t *testing.T) {
	app := setupAPIKeyTestApp(t)

	// Insert a key to revoke
	_, err := app.SystemDB.Exec(`
		INSERT INTO api_keys (id, project_id, name, token_hash, role)
		VALUES ('key-123', 'test-project', 'Test', 'hash123', 'admin')
	`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		keyID          string
		expectedStatus int
	}{
		{
			name:           "DELETE revoke existing key",
			keyID:          "key-123",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "DELETE non-existent key",
			keyID:          "nonexistent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("DELETE", "/api/v1/projects/test-project/apikeys/"+tt.keyID, nil)
			req.SetPathValue("project", "test-project")
			req.SetPathValue("id", tt.keyID)
			w := httptest.NewRecorder()

			RevokeAPIKeyHandler(app).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("status code: got %d, want %d", w.Code, tt.expectedStatus)
			}

			if tt.expectedStatus == http.StatusNoContent && w.Body.Len() > 0 {
				t.Errorf("204 should have no body, got %d bytes", w.Body.Len())
			}
		})
	}
}

func TestRevokeAPIKeyRoleEnforcement(t *testing.T) {
	projectID := "test-project"

	tests := []struct {
		name           string
		callerRole     auth.Role
		expectedStatus int
	}{
		{"read_only cannot revoke", auth.RoleReadOnly, http.StatusForbidden},
		{"read_write cannot revoke", auth.RoleReadWrite, http.StatusForbidden},
		{"admin can revoke", auth.RoleAdmin, http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupAPIKeyTestApp(t)
			_, err := app.SystemDB.Exec(`
				INSERT INTO api_keys (id, project_id, name, token_hash, role)
				VALUES ('key-target', ?, 'Target', 'hash-target', 'admin')
			`, projectID)
			if err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequest("DELETE", "/api/v1/projects/"+projectID+"/apikeys/key-target", nil)
			req.SetPathValue("project", projectID)
			req.SetPathValue("id", "key-target")
			ac := &auth.AuthContext{KeyID: "caller", ProjectID: &projectID, Role: tt.callerRole}
			req = req.WithContext(auth.WithAuthContext(req.Context(), ac))
			w := httptest.NewRecorder()

			RevokeAPIKeyHandler(app).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("callerRole=%s: got %d, want %d, body=%s", tt.callerRole, w.Code, tt.expectedStatus, w.Body.String())
			}
		})
	}

	t.Run("system key can revoke", func(t *testing.T) {
		app := setupAPIKeyTestApp(t)
		_, err := app.SystemDB.Exec(`
			INSERT INTO api_keys (id, project_id, name, token_hash, role)
			VALUES ('key-target-2', ?, 'Target', 'hash-target-2', 'admin')
		`, projectID)
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("DELETE", "/api/v1/projects/"+projectID+"/apikeys/key-target-2", nil)
		req.SetPathValue("project", projectID)
		req.SetPathValue("id", "key-target-2")
		ac := &auth.AuthContext{KeyID: "sys-caller", ProjectID: nil, Role: auth.RoleSystem}
		req = req.WithContext(auth.WithAuthContext(req.Context(), ac))
		w := httptest.NewRecorder()

		RevokeAPIKeyHandler(app).ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("system key revoke: got %d, want %d, body=%s", w.Code, http.StatusNoContent, w.Body.String())
		}
	})
}

func TestAPIKeyResponseHasTokenOnce(t *testing.T) {
	app := setupAPIKeyTestApp(t)

	body := `{"name":"Test Key","role":"admin"}`
	req := httptest.NewRequest("POST", "/api/v1/projects/test-project/apikeys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("project", "test-project")
	w := httptest.NewRecorder()

	CreateAPIKeyHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var resp CreateAPIKeyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Token == "" {
		t.Error("plaintext token missing")
	}
	if len(resp.Token) < 40 {
		t.Errorf("token length: got %d, want at least 40", len(resp.Token))
	}
}
