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
	"smartdb/internal/project"

	_ "modernc.org/sqlite"
)

func setupBackupTestApp(t *testing.T) (*domain.App, string) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			state TEXT NOT NULL CHECK (state IN ('creating', 'inactive', 'active', 'deleting', 'deleted', 'wiped')) DEFAULT 'creating',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.LoadDefaults()
	cfg.DataDir = t.TempDir()

	projectID, err := project.Create("backup-role-test", db, cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}

	app := &domain.App{SystemDB: db, Config: cfg}
	return app, projectID
}

func withRole(r *http.Request, projectID string, role auth.Role) *http.Request {
	ac := &auth.AuthContext{KeyID: "test-key", ProjectID: &projectID, Role: role}
	return r.WithContext(auth.WithAuthContext(r.Context(), ac))
}

func TestBackupHandlerRoleEnforcement(t *testing.T) {
	app, projectID := setupBackupTestApp(t)

	tests := []struct {
		name       string
		role       auth.Role
		wantStatus int
	}{
		{"admin can backup", auth.RoleAdmin, http.StatusOK},
		{"read_write cannot backup", auth.RoleReadWrite, http.StatusForbidden},
		{"read_only cannot backup", auth.RoleReadOnly, http.StatusForbidden},
		{"system cannot backup", auth.RoleSystem, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/backup", nil)
			req.SetPathValue("project", projectID)
			req = withRole(req, projectID, tt.role)

			w := httptest.NewRecorder()
			BackupHandler(app).ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("role=%s: got %d, want %d, body=%s", tt.role, w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestRestoreHandlerRoleEnforcement(t *testing.T) {
	app, projectID := setupBackupTestApp(t)

	// Seed a backup as admin first.
	req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/backup", nil)
	req.SetPathValue("project", projectID)
	req = withRole(req, projectID, auth.RoleAdmin)
	w := httptest.NewRecorder()
	BackupHandler(app).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("seed backup failed: %d %s", w.Code, w.Body.String())
	}
	var backupResp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&backupResp); err != nil {
		t.Fatal(err)
	}
	backupName := backupResp["backup"]

	tests := []struct {
		name       string
		role       auth.Role
		wantStatus int
	}{
		{"read_only cannot restore", auth.RoleReadOnly, http.StatusForbidden},
		{"read_write cannot restore", auth.RoleReadWrite, http.StatusForbidden},
		{"system cannot restore", auth.RoleSystem, http.StatusForbidden},
		{"admin can restore", auth.RoleAdmin, http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"backup":"` + backupName + `"}`
			req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/restore", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("project", projectID)
			req = withRole(req, projectID, tt.role)

			w := httptest.NewRecorder()
			RestoreHandler(app).ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("role=%s: got %d, want %d, body=%s", tt.role, w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
