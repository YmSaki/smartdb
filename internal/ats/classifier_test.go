package ats

import (
	"fmt"
	"testing"
)

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

func TestClassifySQLAttachRejected(t *testing.T) {
	tests := []string{
		"ATTACH DATABASE '/data/project-b/database.db' AS victim",
		"attach database 'x.db' as x",
		"SELECT 1; ATTACH DATABASE 'x.db' AS x",
	}
	for _, sql := range tests {
		t.Run(sql, func(t *testing.T) {
			_, err := ClassifySQL(sql)
			if err == nil {
				t.Errorf("ClassifySQL(%q) should error, got nil", sql)
			}
		})
	}
}

func TestClassifySQLVacuumIntoRejected(t *testing.T) {
	tests := []string{
		"VACUUM INTO '/data/project-b/database.db'",
		"vacuum into 'x.db'",
		"VACUUM main INTO 'x.db'",
		// A quoted schema-name (any of SQLite's three quoting styles) or a
		// single-quoted string used as an identifier (SQLite's documented
		// fallback) must not let VACUUM INTO slip past as a bare VACUUM.
		`VACUUM "main" INTO '/data/project-b/database.db'`,
		"VACUUM `main` INTO '/data/project-b/database.db'",
		"VACUUM [main] INTO '/data/project-b/database.db'",
		"VACUUM 'main' INTO '/data/project-b/database.db'",
	}
	for _, sql := range tests {
		t.Run(sql, func(t *testing.T) {
			_, err := ClassifySQL(sql)
			if err == nil {
				t.Errorf("ClassifySQL(%q) should error, got nil", sql)
			}
		})
	}
}

func TestClassifySQLBareVacuumAllowed(t *testing.T) {
	tests := []string{
		"VACUUM",
		"VACUUM main",
		`VACUUM "main"`,
		"VACUUM `main`",
		"VACUUM [main]",
		"VACUUM 'main'",
	}
	for _, sql := range tests {
		t.Run(sql, func(t *testing.T) {
			cat, err := ClassifySQL(sql)
			if err != nil {
				t.Fatalf("ClassifySQL(%q) unexpected error: %v", sql, err)
			}
			if cat != CategoryAdmin {
				t.Errorf("ClassifySQL(%q) = %q, want %q", sql, cat, CategoryAdmin)
			}
		})
	}
}

func TestClassifySQLControlCharLeaderRejected(t *testing.T) {
	// \v (0x0B) and \f (0x0C) are whitespace to SQLite's own tokenizer
	// (sqlite3Isspace covers 0x09-0x0D) but were not to ours, so a stray
	// SYMBOL token for one of them could silently consume the
	// statement-leader slot and let the real ATTACH/VACUUM right after it
	// dodge the checks entirely. skipSpace now recognizes them (so they
	// behave as ordinary leading whitespace); this also exercises the
	// fail-closed guard for any other unrecognized leading byte.
	tests := []string{
		"\fVACUUM INTO '/data/project-b/database.db'",
		"\vVACUUM INTO '/data/project-b/database.db'",
		"\fATTACH DATABASE '/data/project-b/database.db' AS x",
	}
	for _, sql := range tests {
		t.Run(sql, func(t *testing.T) {
			_, err := ClassifySQL(sql)
			if err == nil {
				t.Errorf("ClassifySQL(%q) should error, got nil", sql)
			}
		})
	}
}

func TestClassifySQLLegitLeadingWhitespaceAllowed(t *testing.T) {
	for _, ws := range []string{" ", "\t", "\n", "\r", "\v", "\f"} {
		t.Run(fmt.Sprintf("%q", ws), func(t *testing.T) {
			cat, err := ClassifySQL(ws + "SELECT 1")
			if err != nil {
				t.Fatalf("ClassifySQL(%q) unexpected error: %v", ws+"SELECT 1", err)
			}
			if cat != CategoryRead {
				t.Errorf("ClassifySQL(%q) = %q, want %q", ws+"SELECT 1", cat, CategoryRead)
			}
		})
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
