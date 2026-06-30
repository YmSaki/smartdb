package main

import (
	"database/sql"
	"smartdb/internal/domain"

	_ "modernc.org/sqlite"
)

func InitializeSystemDB(dbPath string) (*sql.DB, error) {
	dsn := domain.GetDataBaseDSN(dbPath)

	db, err := sql.Open("sqlite", dsn)
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

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS api_keys (
			id           TEXT PRIMARY KEY,
			project_id   TEXT REFERENCES projects(id),
			name         TEXT NOT NULL,
			token_hash   TEXT NOT NULL UNIQUE,
			role         TEXT NOT NULL CHECK (role IN ('admin', 'read_write', 'read_only')),
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			revoked_at   DATETIME
		)
	`)
	if err != nil {
		return nil, err
	}

	return db, nil
}
