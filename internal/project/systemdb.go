package project

import (
	"database/sql"
	"fmt"
	"log/slog"
	"smartdb/internal/domain"
	"strings"

	_ "modernc.org/sqlite"
)

func UpdateProjectState(systemDB *sql.DB, projectID string, state domain.ProjectState) error {
	if projectID == "" {
		return fmt.Errorf("ProjectID is empty")
	}
	if !state.IsValid() {
		return fmt.Errorf("invalid state: %s", state)
	}
	res, err := systemDB.Exec(`
		UPDATE projects
		SET 
			state = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, state, projectID)

	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func AddProject(systemDB *sql.DB, projectID string, name string) error {
	_, err := systemDB.Exec(`
		INSERT INTO projects (id, name) VALUES (?, ?)
	`, projectID, name)
	return err
}

type ProjectFilter struct {
	State []domain.ProjectState
}

func GetProjectList(systemDB *sql.DB, filter ProjectFilter) ([]domain.Project, error) {
	if len(filter.State) == 0 {
		return []domain.Project{}, nil
	}
	placeholders := strings.Repeat("?,", len(filter.State))
	placeholders = strings.TrimRight(placeholders, ",")
	query := fmt.Sprintf(`
		SELECT *
		FROM projects
		WHERE state IN (%s)
		ORDER BY state, name
	`, placeholders)

	args := make([]any, len(filter.State))
	for i, s := range filter.State {
		args[i] = s
	}

	rows, err := systemDB.Query(query, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	projectList := make([]domain.Project, 0)
	for rows.Next() {
		var project domain.Project
		if err := domain.ScanProject(rows, &project); err != nil {
			return nil, err
		}
		projectList = append(projectList, project)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return projectList, nil
}

func GetProject(systemDB *sql.DB, projectID string) (domain.Project, error) {
	var project domain.Project
	row := systemDB.QueryRow(`
		SELECT *
		FROM projects
		WHERE id = ?
	`, projectID)
	if err := domain.ScanProject(row, &project); err != nil {
		slog.Error("GetProject select error", "error", err)
		return project, err
	}
	return project, nil
}
