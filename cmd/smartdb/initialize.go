package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func InitializeSystemDB(dbPath string) (*sql.DB, error) {
	dns := fmt.Sprintf(
		"file:%s?"+
			"_pragma=journal_mode(WAL)&"+
			"_pragma=busy_timeout(5000)&"+
			"_pragma=foreign_keys(1)&"+
			"_pragma=synchronous(NORMAL)&"+
			"_pragma=temp_store(MEMORY)",
		dbPath,
	)

	db, err := sql.Open("sqlite", dns)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			state TEXT NOT NULL CHECK (state IN ('creating', 'inactive', 'active', 'deleting', 'deleted', 'wiped')) DEFAULT 'creating',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, err
	}

	return db, nil
}
