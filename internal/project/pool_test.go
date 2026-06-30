package project

import (
	"database/sql"
	"os"
	"sync"
	"testing"

	_ "modernc.org/sqlite"
)

func setupPoolTest(t *testing.T, projectIDs ...string) {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	for _, id := range projectIDs {
		dir := "data/" + id
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		db, err := sql.Open("sqlite", dir+"/database.db")
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Ping(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}

func TestPoolGet(t *testing.T) {
	setupPoolTest(t, "test-project")
	pool := NewConnectionPool()
	defer pool.Close()

	db, err := pool.Get("test-project")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if db == nil {
		t.Fatal("got nil connection")
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}

func TestPoolReuse(t *testing.T) {
	setupPoolTest(t, "test-project")
	pool := NewConnectionPool()
	defer pool.Close()

	db1, err := pool.Get("test-project")
	if err != nil {
		t.Fatal(err)
	}
	db2, err := pool.Get("test-project")
	if err != nil {
		t.Fatal(err)
	}
	if db1 != db2 {
		t.Error("pool did not reuse connection")
	}
}

func TestPoolClose(t *testing.T) {
	setupPoolTest(t, "test-project")
	pool := NewConnectionPool()

	db, err := pool.Get("test-project")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}

	pool.Close()

	if err := db.Ping(); err == nil {
		t.Error("ping after pool close should fail")
	}
}

func TestPoolConcurrency(t *testing.T) {
	setupPoolTest(t, "test-project")
	pool := NewConnectionPool()
	defer pool.Close()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5 {
				db, err := pool.Get("test-project")
				if err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()
					continue
				}
				if err := db.Ping(); err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	if len(errs) > 0 {
		t.Errorf("concurrent errors: %v", errs)
	}
}

func TestPoolDifferentProjects(t *testing.T) {
	setupPoolTest(t, "project-a", "project-b")
	pool := NewConnectionPool()
	defer pool.Close()

	dbA, err := pool.Get("project-a")
	if err != nil {
		t.Fatal(err)
	}
	dbB, err := pool.Get("project-b")
	if err != nil {
		t.Fatal(err)
	}
	dbA2, err := pool.Get("project-a")
	if err != nil {
		t.Fatal(err)
	}

	if dbA != dbA2 {
		t.Error("same project should reuse connection")
	}
	if dbA == dbB {
		t.Error("different projects should have different connections")
	}
}

func TestPoolConnectionValidity(t *testing.T) {
	setupPoolTest(t, "test-project")
	pool := NewConnectionPool()
	defer pool.Close()

	db, err := pool.Get("test-project")
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Ping(); err != nil {
		t.Errorf("ping failed: %v", err)
	}
	if _, err := db.Exec("SELECT 1"); err != nil {
		t.Errorf("exec failed: %v", err)
	}
	rows, err := db.Query("SELECT 1")
	if err != nil {
		t.Errorf("query failed: %v", err)
	} else {
		rows.Close()
	}
}
