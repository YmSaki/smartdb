package project

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	_ "modernc.org/sqlite"
)

type ProjectStats struct {
	Size             int64  `json:"size"`
	Tables           int    `json:"tables"`
	BackupCount      int    `json:"backup_count"`
	MigrationVersion string `json:"migration_version"`
}

type ColumnInfo struct {
	CID       int     `json:"cid"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	NotNull   int     `json:"notnull"`
	DfltValue *string `json:"dflt_value"`
	PK        int     `json:"pk"`
}

func GetProjectStats(ctx context.Context, projectID string) (*ProjectStats, error) {
	dsn := GetProjectDNS(projectID)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	stats := &ProjectStats{}

	var pageCount, pageSize int64
	if err := db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount); err != nil {
		return nil, err
	}
	if err := db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		return nil, err
	}
	stats.Size = pageCount * pageSize

	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE '__%%'",
	).Scan(&stats.Tables); err != nil {
		return nil, err
	}

	backupDir := fmt.Sprintf("data/%s/backups", projectID)
	entries, err := os.ReadDir(backupDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".db" {
				stats.BackupCount++
			}
		}
	}

	var version sql.NullString
	_ = db.QueryRowContext(ctx,
		"SELECT version FROM __migrations ORDER BY applied_at DESC, version DESC LIMIT 1",
	).Scan(&version)
	if version.Valid {
		stats.MigrationVersion = version.String
	}

	return stats, nil
}

func GetTables(ctx context.Context, projectID string) ([]string, error) {
	dsn := GetProjectDNS(projectID)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE '__%%' ORDER BY name",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	if tables == nil {
		tables = []string{}
	}
	return tables, rows.Err()
}

func GetTableSchema(ctx context.Context, projectID string, tableName string) ([]ColumnInfo, error) {
	dsn := GetProjectDNS(projectID)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := fmt.Sprintf("PRAGMA table_info(\"%s\")", tableName)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		if err := rows.Scan(&c.CID, &c.Name, &c.Type, &c.NotNull, &c.DfltValue, &c.PK); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	if cols == nil {
		cols = []ColumnInfo{}
	}
	return cols, rows.Err()
}
