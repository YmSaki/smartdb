package auth

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupBootstrapTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE api_keys (
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
		t.Fatal(err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func TestBootstrapFirstRun(t *testing.T) {
	db := setupBootstrapTestDB(t)

	err := BootstrapAdminKey(db)
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM api_keys WHERE project_id IS NULL").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 system key, got %d", count)
	}

	var role string
	if err := db.QueryRow("SELECT role FROM api_keys WHERE project_id IS NULL").Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != "admin" {
		t.Errorf("role: got %q, want admin", role)
	}
}

func TestBootstrapSubsequentRun(t *testing.T) {
	db := setupBootstrapTestDB(t)

	if err := BootstrapAdminKey(db); err != nil {
		t.Fatalf("first bootstrap failed: %v", err)
	}

	if err := BootstrapAdminKey(db); err != nil {
		t.Fatalf("second bootstrap failed: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM api_keys WHERE project_id IS NULL").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 system key after second bootstrap, got %d", count)
	}
}

func TestBootstrapKeyIsSystemLevel(t *testing.T) {
	db := setupBootstrapTestDB(t)

	if err := BootstrapAdminKey(db); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	var projectID sql.NullString
	if err := db.QueryRow("SELECT project_id FROM api_keys WHERE project_id IS NULL").Scan(&projectID); err != nil {
		t.Fatal(err)
	}
	if projectID.Valid {
		t.Errorf("system key should have NULL project_id, got %q", projectID.String)
	}
}
