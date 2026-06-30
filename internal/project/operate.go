package project

import (
	"context"
	"database/sql"
	"log/slog"
	"smartdb/internal/ats"
)

type SQLType string

const (
	SQLTypeRead   SQLType = "read"
	SQLTypeEdit   SQLType = "edit"
	SQLTypeManage SQLType = "manage"
	SQLTypeAdmin  SQLType = "admin"
)

func QueryJudge(query string) (SQLType, error) {
	cat, err := ats.ClassifySQL(query)
	if err != nil {
		return "", err
	}
	return SQLType(cat), nil
}

func Query(ctx context.Context, dataDir string, projectID string, query string) ([]map[string]any, error) {
	dns := GetProjectDNS(dataDir, projectID)
	db, err := sql.Open("sqlite", dns)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		slog.Error(
			"SQL Query Execute Error",
			"projectID",
			projectID,
			"sql",
			query,
			"error",
			err,
		)
		return nil, err
	}

	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		slog.Error("Failed get cols", "error", err)
		return nil, err
	}

	var results []map[string]any
	for rows.Next() {
		rCols := make([]any, len(cols))
		rColsPtr := make([]any, len(cols))

		for i := range rCols {
			rColsPtr[i] = &rCols[i]
		}
		if err := rows.Scan(rColsPtr...); err != nil {
			slog.Error("failed parse result", "error", err)
			return nil, err
		}

		rowMap := make(map[string]any)
		for i, colName := range cols {
			val := rCols[i]

			if bytes, ok := val.([]byte); ok {
				rowMap[colName] = string(bytes)
			} else {
				rowMap[colName] = val
			}
		}

		results = append(results, rowMap)
	}

	if err := rows.Err(); err != nil {
		slog.Error("failed get data", "error", err)
		return nil, err
	}

	if results == nil {
		results = make([]map[string]any, 0)
	}

	return results, nil
}

func Execute(ctx context.Context, dataDir string, projectID string, query string) (int64, error) {
	dns := GetProjectDNS(dataDir, projectID)
	db, err := sql.Open("sqlite", dns)
	if err != nil {
		return 0, err
	}
	defer func() {
		err := db.Close()
		if err != nil {
			slog.Error("failed close DB", "projectid", projectID, "error", err)
		}
	}()

	result, err := db.ExecContext(ctx, query)
	if err != nil {
		slog.Error("failed to execute query", "projectID", projectID, "error", err)
		return 0, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		slog.Error("failed to get affected rows")
		return 0, err
	}

	return rows, nil

}
