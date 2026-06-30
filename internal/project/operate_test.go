package project

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestProjectDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Create a simple test table
	_, err = db.Exec(`
		CREATE TABLE test_table (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			value INTEGER
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func TestQueryJudgeAllSQLTypes(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		expectedType SQLType
		shouldErr   bool
	}{
		{
			name:        "SELECT query",
			sql:         "SELECT * FROM users",
			expectedType: SQLTypeRead,
			shouldErr:   false,
		},
		{
			name:        "SELECT with whitespace",
			sql:         "   SELECT * FROM users   ",
			expectedType: SQLTypeRead,
			shouldErr:   false,
		},
		{
			name:        "INSERT query",
			sql:         "INSERT INTO users (name) VALUES ('John')",
			expectedType: SQLTypeEdit,
			shouldErr:   false,
		},
		{
			name:        "UPDATE query",
			sql:         "UPDATE users SET name = 'Jane' WHERE id = 1",
			expectedType: SQLTypeEdit,
			shouldErr:   false,
		},
		{
			name:        "DELETE query",
			sql:         "DELETE FROM users WHERE id = 1",
			expectedType: SQLTypeEdit,
			shouldErr:   false,
		},
		{
			name:        "WITH CTE (Common Table Expression)",
			sql:         "WITH cte AS (SELECT 1) SELECT * FROM cte",
			expectedType: SQLTypeRead,
			shouldErr:   false,
		},
		{
			name:        "PRAGMA query",
			sql:         "PRAGMA journal_mode",
			expectedType: SQLTypeManage,
			shouldErr:   false,
		},
		{
			name:        "EXPLAIN query",
			sql:         "EXPLAIN SELECT * FROM users",
			expectedType: SQLTypeManage,
			shouldErr:   false,
		},
		{
			name:        "CREATE TABLE",
			sql:         "CREATE TABLE new_table (id INTEGER PRIMARY KEY)",
			expectedType: SQLTypeAdmin,
			shouldErr:   false,
		},
		{
			name:        "DROP TABLE",
			sql:         "DROP TABLE users",
			expectedType: SQLTypeAdmin,
			shouldErr:   false,
		},
		{
			name:        "empty query",
			sql:         "",
			expectedType: "",
			shouldErr:   true,
		},
		{
			name:        "only whitespace",
			sql:         "   \n\t  ",
			expectedType: "",
			shouldErr:   true,
		},
		{
			name:        "lowercase select",
			sql:         "select * from users",
			expectedType: SQLTypeRead,
			shouldErr:   false,
		},
		{
			name:        "lowercase insert",
			sql:         "insert into users values (1)",
			expectedType: SQLTypeEdit,
			shouldErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryType, err := QueryJudge(tt.sql)

			if tt.shouldErr && err == nil {
				t.Errorf("expected error for %q, got nil", tt.sql)
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tt.sql, err)
			}

			if !tt.shouldErr && queryType != tt.expectedType {
				t.Errorf("query type: got %q, want %q", queryType, tt.expectedType)
			}
		})
	}
}

func TestQueryTimeout(t *testing.T) {
	db := setupTestProjectDB(t)

	// Insert test data
	_, err := db.Exec("INSERT INTO test_table (name, value) VALUES ('test', 1)")
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	tests := []struct {
		name       string
		sql        string
		timeout    time.Duration
		shouldErr  bool
		description string
	}{
		{
			name:        "query completes within timeout",
			sql:         "SELECT * FROM test_table",
			timeout:     5 * time.Second,
			shouldErr:   false,
			description: "simple query should complete quickly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			rows, err := db.QueryContext(ctx, tt.sql)
			if rows != nil {
				rows.Close()
			}

			if tt.shouldErr && err == nil {
				t.Errorf("expected timeout error (%s)", tt.description)
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error (%s): %v", tt.description, err)
			}
		})
	}

	t.Run("query with already-cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		rows, err := db.QueryContext(ctx, "SELECT * FROM test_table")
		if rows != nil {
			rows.Close()
		}
		if err == nil {
			t.Error("cancelled context should cause error")
		}
	})
}

func TestExecuteTimeout(t *testing.T) {
	db := setupTestProjectDB(t)

	tests := []struct {
		name       string
		sql        string
		timeout    time.Duration
		shouldErr  bool
		description string
	}{
		{
			name:        "execute completes within timeout",
			sql:         "INSERT INTO test_table (name, value) VALUES ('test', 1)",
			timeout:     5 * time.Second,
			shouldErr:   false,
			description: "simple insert should complete quickly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			_, err := db.ExecContext(ctx, tt.sql)

			if tt.shouldErr && err == nil {
				t.Errorf("expected timeout error (%s)", tt.description)
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error (%s): %v", tt.description, err)
			}
		})
	}

	t.Run("execute with already-cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := db.ExecContext(ctx, "INSERT INTO test_table (name, value) VALUES ('x', 1)")
		if err == nil {
			t.Error("cancelled context should cause error")
		}
	})
}
