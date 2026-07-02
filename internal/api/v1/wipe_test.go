package v1

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"smartdb/internal/auth"
	"smartdb/internal/config"
	"smartdb/internal/domain"
	"smartdb/internal/project"

	_ "modernc.org/sqlite"
)

func setupWipeTestApp(t *testing.T) *domain.App {
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
	_, err = db.Exec(`
		CREATE TABLE api_keys (
			id           TEXT PRIMARY KEY,
			project_id   TEXT REFERENCES projects(id),
			name         TEXT NOT NULL,
			token_hash   TEXT NOT NULL UNIQUE,
			role         TEXT NOT NULL CHECK (role IN ('system', 'admin', 'read_write', 'read_only')),
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			revoked_at   DATETIME
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.LoadDefaults()
	cfg.DataDir = t.TempDir()

	return &domain.App{SystemDB: db, Config: cfg}
}

func TestWipeRequiresDeletedState(t *testing.T) {
	app := setupWipeTestApp(t)

	projectID, err := project.Create("wipe-me", app.SystemDB, app.Config.DataDir)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/wipe", nil)
	req.SetPathValue("project", projectID)
	w := httptest.NewRecorder()

	WipeProjectHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("wiping an inactive (non-deleted) project: got %d, want %d", w.Code, http.StatusConflict)
	}

	if _, err := os.Stat(app.Config.DataDir + "/" + projectID); err != nil {
		t.Errorf("project directory should still exist: %v", err)
	}
}

func TestWipeRemovesFilesAndRevokesKeys(t *testing.T) {
	app := setupWipeTestApp(t)

	projectID, err := project.Create("wipe-me-2", app.SystemDB, app.Config.DataDir)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	keyID, err := auth.CreateKey(app.SystemDB, &projectID, "some-key", "hash-abc", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	if err := project.UpdateProjectState(app.SystemDB, projectID, domain.StateActive); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if err := project.UpdateProjectState(app.SystemDB, projectID, domain.StateDeleted); err != nil {
		t.Fatalf("mark deleted: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/wipe", nil)
	req.SetPathValue("project", projectID)
	w := httptest.NewRecorder()

	WipeProjectHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("wipe: got %d, want %d, body=%s", w.Code, http.StatusNoContent, w.Body.String())
	}

	if _, err := os.Stat(app.Config.DataDir + "/" + projectID); !os.IsNotExist(err) {
		t.Errorf("project directory should be removed, stat err=%v", err)
	}

	p, err := project.GetProject(app.SystemDB, projectID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if p.State != domain.StateWiped {
		t.Errorf("state: got %q, want %q", p.State, domain.StateWiped)
	}

	keys, err := auth.ListKeys(app.SystemDB, &projectID)
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	found := false
	for _, k := range keys {
		if k.ID == keyID {
			found = true
			if k.RevokedAt == nil {
				t.Error("key should be revoked after wipe")
			}
		}
	}
	if !found {
		t.Fatal("expected key not found")
	}
}

func TestWipeNonExistentProject(t *testing.T) {
	app := setupWipeTestApp(t)

	req := httptest.NewRequest("POST", "/api/v1/projects/nope/wipe", nil)
	req.SetPathValue("project", "nope")
	w := httptest.NewRecorder()

	WipeProjectHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusNotFound)
	}
}
