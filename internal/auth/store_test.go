package auth

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestAuthDB(t *testing.T) *sql.DB {
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

func TestCreateKey(t *testing.T) {
	db := setupTestAuthDB(t)

	id, err := CreateKey(db, nil, "System Admin", "hash_abc123", RoleAdmin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Error("returned empty id")
	}

	// Duplicate token_hash should error
	_, err = CreateKey(db, nil, "Dup", "hash_abc123", RoleAdmin)
	if err == nil {
		t.Error("duplicate token_hash should error")
	}
}

func TestCreateProjectKey(t *testing.T) {
	db := setupTestAuthDB(t)

	projectID := "test-project"
	id, err := CreateKey(db, &projectID, "Project Key", "hash_proj1", RoleReadWrite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Error("returned empty id")
	}
}

func TestGetKeyByHash(t *testing.T) {
	db := setupTestAuthDB(t)

	tokenHash := "test_hash_lookup"
	_, err := db.Exec(`
		INSERT INTO api_keys (id, project_id, name, token_hash, role)
		VALUES ('key-1', NULL, 'Test Key', ?, 'admin')
	`, tokenHash)
	if err != nil {
		t.Fatal(err)
	}

	key, err := GetKeyByHash(db, tokenHash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key == nil {
		t.Fatal("got nil key")
	}
	if key.ID != "key-1" {
		t.Errorf("id: got %q, want key-1", key.ID)
	}
	if key.Role != RoleAdmin {
		t.Errorf("role: got %q, want admin", key.Role)
	}
}

func TestGetKeyByHashNotFound(t *testing.T) {
	db := setupTestAuthDB(t)

	key, err := GetKeyByHash(db, "nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
	if key != nil {
		t.Error("expected nil key")
	}
}

func TestGetKeyByHashRevoked(t *testing.T) {
	db := setupTestAuthDB(t)

	_, err := db.Exec(`
		INSERT INTO api_keys (id, project_id, name, token_hash, role, revoked_at)
		VALUES ('key-r', NULL, 'Revoked', 'hash_revoked', 'admin', datetime('now'))
	`)
	if err != nil {
		t.Fatal(err)
	}

	key, err := GetKeyByHash(db, "hash_revoked")
	if key != nil {
		t.Error("revoked key should not be returned")
	}
	if err == nil {
		t.Error("expected error for revoked key")
	}
}

func TestListKeys(t *testing.T) {
	db := setupTestAuthDB(t)

	// Insert system keys
	for _, k := range []struct{ id, hash string }{
		{"k1", "h1"}, {"k2", "h2"}, {"k3", "h3"},
	} {
		_, err := db.Exec(`
			INSERT INTO api_keys (id, project_id, name, token_hash, role)
			VALUES (?, NULL, 'Key', ?, 'admin')
		`, k.id, k.hash)
		if err != nil {
			t.Fatal(err)
		}
	}

	list, err := ListKeys(db, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 keys, got %d", len(list))
	}
}

func TestListKeysProject(t *testing.T) {
	db := setupTestAuthDB(t)

	projA := "proj-a"
	projB := "proj-b"
	_, _ = db.Exec(`INSERT INTO api_keys (id, project_id, name, token_hash, role) VALUES ('k1', ?, 'A1', 'ha1', 'read_write')`, projA)
	_, _ = db.Exec(`INSERT INTO api_keys (id, project_id, name, token_hash, role) VALUES ('k2', ?, 'A2', 'ha2', 'read_only')`, projA)
	_, _ = db.Exec(`INSERT INTO api_keys (id, project_id, name, token_hash, role) VALUES ('k3', ?, 'B1', 'hb1', 'admin')`, projB)

	list, err := ListKeys(db, &projA)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 keys for proj-a, got %d", len(list))
	}
}

func TestRevokeKey(t *testing.T) {
	db := setupTestAuthDB(t)

	_, err := db.Exec(`
		INSERT INTO api_keys (id, project_id, name, token_hash, role)
		VALUES ('key-1', NULL, 'Test', 'hash_to_revoke', 'admin')
	`)
	if err != nil {
		t.Fatal(err)
	}

	if err := RevokeKey(db, "key-1"); err != nil {
		t.Fatalf("revoke failed: %v", err)
	}

	var revokedAt sql.NullTime
	if err := db.QueryRow("SELECT revoked_at FROM api_keys WHERE id = 'key-1'").Scan(&revokedAt); err != nil {
		t.Fatal(err)
	}
	if !revokedAt.Valid {
		t.Error("revoked_at should be set")
	}
}

func TestRevokeKeyNotFound(t *testing.T) {
	db := setupTestAuthDB(t)

	err := RevokeKey(db, "nonexistent")
	if err == nil {
		t.Error("revoking nonexistent key should error")
	}
}
