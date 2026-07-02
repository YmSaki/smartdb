package backup

import (
	"database/sql"
	"os"
	"path/filepath"
	"smartdb/internal/domain"
	"testing"

	_ "modernc.org/sqlite"
)

func setupBackupTestProject(t *testing.T) (dataDir, projectID string) {
	t.Helper()
	dataDir = t.TempDir()
	projectID = "proj"

	if err := os.MkdirAll(filepath.Join(dataDir, projectID), 0755); err != nil {
		t.Fatal(err)
	}

	dsn := domain.GetDataBaseDSN(filepath.Join(dataDir, projectID, "database.db"))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}

	return dataDir, projectID
}

func TestCreateBackupRapidCallsDoNotCollide(t *testing.T) {
	dataDir, projectID := setupBackupTestProject(t)

	names := map[string]bool{}
	for i := 0; i < 10; i++ {
		name, err := CreateBackup(dataDir, projectID)
		if err != nil {
			t.Fatalf("backup %d failed: %v", i, err)
		}
		if names[name] {
			t.Fatalf("duplicate backup name on call %d: %s", i, name)
		}
		names[name] = true
	}

	if len(names) != 10 {
		t.Errorf("expected 10 unique backups, got %d", len(names))
	}
}
