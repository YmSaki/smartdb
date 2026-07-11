package backup

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"smartdb/internal/domain"

	_ "modernc.org/sqlite"
)

// TestRestoreRemovesStaleSidecarFiles verifies that RestoreBackup cleans up
// leftover -wal/-shm files next to the database it replaces, instead of
// leaving them for the next connection to (mis)replay.
func TestRestoreRemovesStaleSidecarFiles(t *testing.T) {
	dataDir, projectID := setupBackupTestProject(t)

	dbPath := filepath.Join(dataDir, projectID, "database.db")
	backupName, err := CreateBackup(dataDir, projectID)
	if err != nil {
		t.Fatal(err)
	}

	walPath := dbPath + "-wal"
	shmPath := dbPath + "-shm"
	if err := os.WriteFile(walPath, []byte("stale wal content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shmPath, []byte("stale shm content"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := RestoreBackup(dataDir, projectID, backupName); err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}

	if _, err := os.Stat(walPath); !os.IsNotExist(err) {
		t.Errorf("stale -wal file should have been removed, stat err=%v", err)
	}
	if _, err := os.Stat(shmPath); !os.IsNotExist(err) {
		t.Errorf("stale -shm file should have been removed, stat err=%v", err)
	}
}

// TestRestoreDoesNotReplayStaleWAL reproduces the actual failure mode: a
// real WAL frame written *after* the backup was taken must not resurface
// once the database has been restored to the pre-that-write backup.
//
// modernc.org/sqlite (like SQLite generally) checkpoints and deletes the
// -wal/-shm sidecars when the last connection to a database closes
// cleanly, which is also this project's normal per-request pattern (see
// internal/project/operate.go - every Query/Execute opens and closes its
// own connection). So a stale WAL sitting next to database.db isn't the
// everyday case; it's what's left behind after an unclean shutdown
// (process crash, OOM kill, power loss) mid-write - a realistic scenario
// for a self-hosted server, and exactly the case docs/spec.md §11 "DB破損
// 対策" exists for. This test simulates that: it captures a real WAL
// containing a real uncommitted-to-the-main-file row, and puts it back on
// disk after the writer that created it has closed, standing in for "the
// process died before it could checkpoint."
func TestRestoreDoesNotReplayStaleWAL(t *testing.T) {
	dataDir, projectID := setupBackupTestProject(t)

	dbPath := filepath.Join(dataDir, projectID, "database.db")
	dsn := domain.GetDataBaseDSN(dbPath)
	walPath := dbPath + "-wal"
	shmPath := dbPath + "-shm"

	db1, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db1.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, val TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db1.Exec("INSERT INTO t (id, val) VALUES (1, 'from-backup')"); err != nil {
		t.Fatal(err)
	}
	// Force a clean checkpoint so the backup we take next isn't
	// incidentally affected by this connection's own WAL state.
	if _, err := db1.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatal(err)
	}
	if err := db1.Close(); err != nil {
		t.Fatal(err)
	}

	backupName, err := CreateBackup(dataDir, projectID)
	if err != nil {
		t.Fatal(err)
	}

	// A second, later write that must NOT survive the restore below.
	db2, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db2.Exec("INSERT INTO t (id, val) VALUES (2, 'post-backup-should-be-lost')"); err != nil {
		t.Fatal(err)
	}

	// Capture the real, valid WAL/SHM bytes before closing db2 - closing
	// cleanly would checkpoint-and-delete them, which is not the scenario
	// under test (see doc comment above).
	walBytes, err := os.ReadFile(walPath)
	if err != nil || len(walBytes) == 0 {
		t.Fatalf("expected a non-empty -wal file before closing, err=%v len=%d", err, len(walBytes))
	}
	shmBytes, shmErr := os.ReadFile(shmPath)

	if err := db2.Close(); err != nil {
		t.Fatal(err)
	}

	// Put the captured WAL/SHM back, standing in for "the writer never
	// got to close cleanly."
	if err := os.WriteFile(walPath, walBytes, 0644); err != nil {
		t.Fatal(err)
	}
	if shmErr == nil {
		if err := os.WriteFile(shmPath, shmBytes, 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := RestoreBackup(dataDir, projectID, backupName); err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}

	db3, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db3.Close()

	rows, err := db3.Query("SELECT id, val FROM t ORDER BY id")
	if err != nil {
		t.Fatalf("query after restore: %v", err)
	}
	defer rows.Close()

	type row struct {
		id  int
		val string
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.val); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || got[0].id != 1 || got[0].val != "from-backup" {
		t.Errorf("post-restore rows = %+v, want exactly [{1 from-backup}] (row 2 should not have been replayed from the stale WAL)", got)
	}
}
