package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"smartdb/internal/auth"
)

func TestExecuteSQLBlockedWhileRestoreLockHeld(t *testing.T) {
	app, projectID := setupBackupTestApp(t)

	release, ok := app.ProjectLocks.TryWriteLock(projectID)
	if !ok {
		t.Fatal("failed to simulate an in-progress restore")
	}
	defer release()

	req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/sql", strings.NewReader(`{"sql":"SELECT 1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("project", projectID)
	req = withRole(req, projectID, auth.RoleAdmin)
	w := httptest.NewRecorder()

	ExecuteSQLHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("SQL during restore lock: got %d, want %d, body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
}

func TestExecuteSQLAllowedAfterRestoreLockReleased(t *testing.T) {
	app, projectID := setupBackupTestApp(t)

	release, ok := app.ProjectLocks.TryWriteLock(projectID)
	if !ok {
		t.Fatal("failed to acquire write lock")
	}
	release()

	req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/sql", strings.NewReader(`{"sql":"SELECT 1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("project", projectID)
	req = withRole(req, projectID, auth.RoleAdmin)
	w := httptest.NewRecorder()

	ExecuteSQLHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("SQL after lock released: got %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestRestoreBlockedWhileSQLReadLockHeld(t *testing.T) {
	app, projectID := setupBackupTestApp(t)

	// Seed a backup to restore from.
	backupReq := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/backup", nil)
	backupReq.SetPathValue("project", projectID)
	backupReq = withRole(backupReq, projectID, auth.RoleAdmin)
	w := httptest.NewRecorder()
	BackupHandler(app).ServeHTTP(w, backupReq)
	if w.Code != http.StatusOK {
		t.Fatalf("seed backup failed: %d %s", w.Code, w.Body.String())
	}
	var backupResp map[string]string
	json.NewDecoder(w.Body).Decode(&backupResp)

	// Simulate an in-flight SQL query holding the shared/read lock.
	release, ok := app.ProjectLocks.TryReadLock(projectID)
	if !ok {
		t.Fatal("failed to simulate an in-flight SQL query")
	}
	defer release()

	body := `{"backup":"` + backupResp["backup"] + `"}`
	req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/restore", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("project", projectID)
	req = withRole(req, projectID, auth.RoleAdmin)
	w2 := httptest.NewRecorder()

	RestoreHandler(app).ServeHTTP(w2, req)

	if w2.Code != http.StatusConflict {
		t.Errorf("restore while SQL read lock held: got %d, want %d, body=%s", w2.Code, http.StatusConflict, w2.Body.String())
	}
}

func TestRestoreBlockedByAnotherRestoreInProgress(t *testing.T) {
	app, projectID := setupBackupTestApp(t)

	backupReq := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/backup", nil)
	backupReq.SetPathValue("project", projectID)
	backupReq = withRole(backupReq, projectID, auth.RoleAdmin)
	w := httptest.NewRecorder()
	BackupHandler(app).ServeHTTP(w, backupReq)
	var backupResp map[string]string
	json.NewDecoder(w.Body).Decode(&backupResp)

	release, ok := app.ProjectLocks.TryWriteLock(projectID)
	if !ok {
		t.Fatal("failed to simulate an in-progress restore")
	}
	defer release()

	body := `{"backup":"` + backupResp["backup"] + `"}`
	req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/restore", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("project", projectID)
	req = withRole(req, projectID, auth.RoleAdmin)
	w2 := httptest.NewRecorder()

	RestoreHandler(app).ServeHTTP(w2, req)

	if w2.Code != http.StatusConflict {
		t.Errorf("concurrent restore: got %d, want %d, body=%s", w2.Code, http.StatusConflict, w2.Body.String())
	}
}

func TestBackupAllowedWhileSQLReadLockHeld(t *testing.T) {
	app, projectID := setupBackupTestApp(t)

	// Simulate an in-flight SQL query — backup should still be allowed
	// (both are shared/read operations).
	release, ok := app.ProjectLocks.TryReadLock(projectID)
	if !ok {
		t.Fatal("failed to simulate an in-flight SQL query")
	}
	defer release()

	req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/backup", nil)
	req.SetPathValue("project", projectID)
	req = withRole(req, projectID, auth.RoleAdmin)
	w := httptest.NewRecorder()

	BackupHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("backup while SQL read lock held: got %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestBackupBlockedWhileRestoreLockHeld(t *testing.T) {
	app, projectID := setupBackupTestApp(t)

	release, ok := app.ProjectLocks.TryWriteLock(projectID)
	if !ok {
		t.Fatal("failed to simulate an in-progress restore")
	}
	defer release()

	req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/backup", nil)
	req.SetPathValue("project", projectID)
	req = withRole(req, projectID, auth.RoleAdmin)
	w := httptest.NewRecorder()

	BackupHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("backup during restore lock: got %d, want %d, body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
}

func TestRestoreLockReleasedEvenOnFailure(t *testing.T) {
	app, projectID := setupBackupTestApp(t)

	// Restore a backup that doesn't exist — the handler should still
	// release its write lock afterwards instead of leaking it.
	req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/restore", strings.NewReader(`{"backup":"does-not-exist.db"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("project", projectID)
	req = withRole(req, projectID, auth.RoleAdmin)
	w := httptest.NewRecorder()

	RestoreHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("restore of missing backup: got %d, want %d, body=%s", w.Code, http.StatusInternalServerError, w.Body.String())
	}

	release, ok := app.ProjectLocks.TryWriteLock(projectID)
	if !ok {
		t.Fatal("write lock should be free after a failed restore, but it's still held")
	}
	release()
}

func TestBackupLockReleasedEvenOnFailure(t *testing.T) {
	app, projectID := setupBackupTestApp(t)

	// Replace the project's backups/ directory with a regular file so
	// os.MkdirAll inside CreateBackup fails, then confirm the read lock
	// was still released.
	backupsPath := app.Config.DataDir + "/" + projectID + "/backups"
	if err := os.WriteFile(backupsPath, []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/backup", nil)
	req.SetPathValue("project", projectID)
	req = withRole(req, projectID, auth.RoleAdmin)
	w := httptest.NewRecorder()

	BackupHandler(app).ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("backup with missing database: got %d, want %d, body=%s", w.Code, http.StatusInternalServerError, w.Body.String())
	}

	release, ok := app.ProjectLocks.TryWriteLock(projectID)
	if !ok {
		t.Fatal("lock should be free after a failed backup, but it's still held")
	}
	release()
}
