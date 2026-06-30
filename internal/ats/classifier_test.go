package ats

import "testing"

func TestClassifySQL(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want SQLCategory
	}{
		{"SELECT", "SELECT * FROM users", CategoryRead},
		{"select lowercase", "select 1", CategoryRead},
		{"INSERT", "INSERT INTO users (name) VALUES ('a')", CategoryEdit},
		{"UPDATE", "UPDATE users SET name='b'", CategoryEdit},
		{"DELETE", "DELETE FROM users WHERE id=1", CategoryEdit},
		{"PRAGMA", "PRAGMA journal_mode", CategoryManage},
		{"EXPLAIN", "EXPLAIN SELECT 1", CategoryManage},
		{"CREATE TABLE", "CREATE TABLE t (id INT)", CategoryAdmin},
		{"DROP TABLE", "DROP TABLE t", CategoryAdmin},
		{"ALTER TABLE", "ALTER TABLE t ADD COLUMN c TEXT", CategoryAdmin},

		// WITH CTE - the critical fix for the QueryJudge vulnerability
		{"WITH + SELECT", "WITH cte AS (SELECT 1) SELECT * FROM cte", CategoryRead},
		{"WITH + DELETE", "WITH cte AS (SELECT id FROM users) DELETE FROM users WHERE id IN (SELECT id FROM cte)", CategoryEdit},
		{"WITH + INSERT", "WITH cte AS (SELECT 1) INSERT INTO t SELECT * FROM cte", CategoryEdit},
		{"WITH + UPDATE", "WITH cte AS (SELECT 1) UPDATE t SET x=1", CategoryEdit},

		// Stacked queries: highest privilege wins
		{"stacked SELECT;DROP", "SELECT 1; DROP TABLE t", CategoryAdmin},
		{"stacked SELECT;DELETE", "SELECT 1; DELETE FROM t", CategoryEdit},
		{"stacked INSERT;SELECT", "INSERT INTO t VALUES(1); SELECT 1", CategoryEdit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ClassifySQL(tt.sql)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ClassifySQL(%q) = %q, want %q", tt.sql, got, tt.want)
			}
		})
	}
}

func TestClassifySQLEmpty(t *testing.T) {
	_, err := ClassifySQL("")
	if err == nil {
		t.Error("empty query should error")
	}
	_, err = ClassifySQL("   ")
	if err == nil {
		t.Error("whitespace-only query should error")
	}
}

func TestClassifySQLStringInjection(t *testing.T) {
	// DELETE inside string literal should not affect classification
	cat, err := ClassifySQL("SELECT * FROM t WHERE name = 'DELETE FROM t'")
	if err != nil {
		t.Fatal(err)
	}
	if cat != CategoryRead {
		t.Errorf("string injection: got %q, want read", cat)
	}
}
