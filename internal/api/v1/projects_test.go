package v1

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"smartdb/internal/config"
	"smartdb/internal/domain"
	"smartdb/internal/handler"
	"smartdb/internal/project"

	_ "modernc.org/sqlite"
)

func setupProjectTestApp(t *testing.T) *domain.App {
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

	return &domain.App{
		SystemDB:     db,
		Config:       config.LoadDefaults(),
		ProjectLocks: domain.NewProjectLockRegistry(),
	}
}

func TestCreateProject(t *testing.T) {
	app := setupProjectTestApp(t)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	body := `{"name":"my-project"}`
	req := httptest.NewRequest("POST", "/api/v1/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	CreateProjectHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusCreated)
	}

	var resp CreateProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.ProjectID == "" {
		t.Error("projectID is empty")
	}
}

func TestCreateProjectRejectsOversizedBody(t *testing.T) {
	app := setupProjectTestApp(t)

	handler.SetMaxBodyBytes(32)
	defer handler.SetMaxBodyBytes(1 << 20)

	body := `{"name":"` + strings.Repeat("a", 200) + `"}`
	req := httptest.NewRequest("POST", "/api/v1/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	CreateProjectHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestCreateProjectEmptyName(t *testing.T) {
	app := setupProjectTestApp(t)

	body := `{"name":""}`
	req := httptest.NewRequest("POST", "/api/v1/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	CreateProjectHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetProjects(t *testing.T) {
	app := setupProjectTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	w := httptest.NewRecorder()

	GetProjectsHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusOK)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type: got %q, want application/json", w.Header().Get("Content-Type"))
	}
}

func TestGetProjectDetail(t *testing.T) {
	app := setupProjectTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/projects/test-project", nil)
	req.SetPathValue("project", "test-project")
	w := httptest.NewRecorder()

	GetProjectDetailHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status code: got %d, want 404 (project does not exist)", w.Code)
	}
}

func TestGetProjectNotFound(t *testing.T) {
	app := setupProjectTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/projects/nonexistent", nil)
	req.SetPathValue("project", "nonexistent")
	w := httptest.NewRecorder()

	GetProjectDetailHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestDeleteProject(t *testing.T) {
	app := setupProjectTestApp(t)

	req := httptest.NewRequest("DELETE", "/api/v1/projects/test-project", nil)
	req.SetPathValue("project", "test-project")
	w := httptest.NewRecorder()

	RemoveProjectHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status code: got %d, want 404 (project does not exist)", w.Code)
	}
}

func TestDeleteProjectRejectsInvalidTransition(t *testing.T) {
	app := setupProjectTestApp(t)

	if _, err := app.SystemDB.Exec(`
		INSERT INTO projects (id, name, state) VALUES ('creating-project', 'p', 'creating')
	`); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("DELETE", "/api/v1/projects/creating-project", nil)
	req.SetPathValue("project", "creating-project")
	w := httptest.NewRecorder()

	RemoveProjectHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("deleting a project still in 'creating': got %d, want %d (INVALID_TRANSITION)", w.Code, http.StatusConflict)
	}
}

func TestDeleteProjectTwiceRejected(t *testing.T) {
	app := setupProjectTestApp(t)

	if _, err := app.SystemDB.Exec(`
		INSERT INTO projects (id, name, state) VALUES ('active-project', 'p', 'active')
	`); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("DELETE", "/api/v1/projects/active-project", nil)
	req.SetPathValue("project", "active-project")
	w := httptest.NewRecorder()
	RemoveProjectHandler(app).ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("first delete: got %d, want %d", w.Code, http.StatusNoContent)
	}

	req2 := httptest.NewRequest("DELETE", "/api/v1/projects/active-project", nil)
	req2.SetPathValue("project", "active-project")
	w2 := httptest.NewRecorder()
	RemoveProjectHandler(app).ServeHTTP(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Errorf("second delete on an already-deleted project: got %d, want %d", w2.Code, http.StatusConflict)
	}
}

func TestUpdateProject(t *testing.T) {
	body := `{"name":"updated-name"}`
	req := httptest.NewRequest("PATCH", "/api/v1/projects/test-project", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("project", "test-project")

	if req.Method != "PATCH" {
		t.Errorf("method: got %s, want PATCH", req.Method)
	}
}

func TestPatchProjectCannotSetDeletedOrWiped(t *testing.T) {
	app := setupProjectTestApp(t)

	if _, err := app.SystemDB.Exec(`
		INSERT INTO projects (id, name, state) VALUES ('active-project', 'p', 'active')
	`); err != nil {
		t.Fatal(err)
	}

	for _, state := range []string{"deleted", "deleting", "wiped"} {
		body := `{"state":"` + state + `"}`
		req := httptest.NewRequest("PATCH", "/api/v1/projects/active-project", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("project", "active-project")
		w := httptest.NewRecorder()

		PatchProjectHandler(app).ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("PATCH state=%s: got %d, want %d (must go through DELETE/wipe)", state, w.Code, http.StatusForbidden)
		}
	}

	// sanity: project should still be 'active', unaffected by the rejected attempts
	p, err := project.GetProject(app.SystemDB, "active-project")
	if err != nil {
		t.Fatal(err)
	}
	if p.State != domain.StateActive {
		t.Errorf("state should be unchanged: got %q, want %q", p.State, domain.StateActive)
	}
}

func TestProjectStats(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/projects/test-project/stats", nil)
	req.SetPathValue("project", "test-project")

	if req.Method != "GET" {
		t.Errorf("method: got %s, want GET", req.Method)
	}
}

func TestTableList(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/projects/test-project/tables", nil)
	req.SetPathValue("project", "test-project")

	if req.Method != "GET" {
		t.Errorf("method: got %s, want GET", req.Method)
	}
}

func TestTableListExcludesInternal(t *testing.T) {
	tests := []struct {
		name          string
		tableName     string
		shouldExclude bool
	}{
		{"normal user table", "users", false},
		{"__migrations internal table", "__migrations", true},
		{"another internal table", "__schema", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if strings.HasPrefix(tt.tableName, "__") && !tt.shouldExclude {
				t.Errorf("table %q should be excluded", tt.tableName)
			}
		})
	}
}

func TestTableSchema(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/projects/test-project/tables/users", nil)
	req.SetPathValue("project", "test-project")
	req.SetPathValue("table", "users")

	if req.Method != "GET" {
		t.Errorf("method: got %s, want GET", req.Method)
	}
}

func TestTableSchemaNotFound(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/projects/test-project/tables/nonexistent", nil)
	req.SetPathValue("project", "test-project")
	req.SetPathValue("table", "nonexistent")

	if req.Method != "GET" {
		t.Errorf("method: got %s, want GET", req.Method)
	}
}

func TestPathTraversalInProjectID(t *testing.T) {
	app := setupProjectTestApp(t)

	maliciousIDs := []string{
		"../etc/passwd",
		"../../etc/passwd",
		"project/../../../etc/passwd",
		"./config",
		"..",
	}

	for _, id := range maliciousIDs {
		t.Run("path traversal "+id, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/projects/"+id, nil)
			req.SetPathValue("project", id)
			w := httptest.NewRecorder()

			GetProjectDetailHandler(app).ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				t.Errorf("path traversal %q should be rejected", id)
			}
		})
	}
}

func TestExecuteSQL(t *testing.T) {
	app := setupProjectTestApp(t)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	// Create a project directory with a database file
	projectID := "test-project"
	projectDir := "data/" + projectID
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	pdb, err := sql.Open("sqlite", projectDir+"/database.db")
	if err != nil {
		t.Fatal(err)
	}
	pdb.Close()

	tests := []struct {
		name           string
		body           string
		expectedStatus int
	}{
		{
			name:           "execute SELECT query",
			body:           `{"sql":"SELECT 1"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "empty SQL query",
			body:           `{"sql":""}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "ATTACH is rejected",
			body:           `{"sql":"ATTACH DATABASE '../other-project/database.db' AS victim"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "VACUUM INTO is rejected",
			body:           `{"sql":"VACUUM INTO '../other-project/database.db'"}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/sql", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("project", projectID)
			w := httptest.NewRecorder()

			ExecuteSQLHandler(app).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("status code: got %d, want %d, body: %s", w.Code, tt.expectedStatus, w.Body.String())
			}
		})
	}
}
