package project

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"
)

// setupStatsTestProject creates a project database on disk at
// <dataDir>/<projectID>/database.db containing the tables:
//   - users        (normal, multi-char name)
//   - posts        (normal, multi-char name)
//   - t             (normal, 1-char name)
//   - __migrations (internal table, should always be excluded)
//
// It returns the dataDir/projectID pair to pass into GetTables/GetProjectStats,
// which open the DB themselves via GetProjectDNS.
func setupStatsTestProject(t *testing.T) (dataDir string, projectID string) {
	t.Helper()

	dataDir = t.TempDir()
	projectID = "proj1"

	projectDir := filepath.Join(dataDir, projectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	dsn := GetProjectDNS(dataDir, projectID)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("failed to open project database: %v", err)
	}
	defer db.Close()

	stmts := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT)`,
		`CREATE TABLE t (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE __migrations (version TEXT, applied_at TEXT)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("failed to execute %q: %v", stmt, err)
		}
	}

	return dataDir, projectID
}

func TestGetTablesExcludesOnlyInternalTables(t *testing.T) {
	dataDir, projectID := setupStatsTestProject(t)

	tables, err := GetTables(context.Background(), dataDir, projectID)
	if err != nil {
		t.Fatalf("GetTables returned error: %v", err)
	}

	want := []string{"posts", "t", "users"}
	if !reflect.DeepEqual(tables, want) {
		t.Errorf("GetTables() = %v, want %v", tables, want)
	}

	for _, name := range tables {
		if name == "__migrations" {
			t.Errorf("GetTables() unexpectedly included internal table __migrations: %v", tables)
		}
	}
}

func TestGetProjectStatsTableCount(t *testing.T) {
	dataDir, projectID := setupStatsTestProject(t)

	stats, err := GetProjectStats(context.Background(), dataDir, projectID)
	if err != nil {
		t.Fatalf("GetProjectStats returned error: %v", err)
	}

	if stats.Tables != 3 {
		t.Errorf("GetProjectStats().Tables = %d, want 3", stats.Tables)
	}
}
