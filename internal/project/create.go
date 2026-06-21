package project

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"smartdb/internal/domain"
	"strings"

	_ "modernc.org/sqlite"
)

type ProjectInfo struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

// Create creates a new project with the given name.
//
// The function creates:
//
//   - project directory
//   - metadata.json
//   - database.db
//
// It also creates:
//
//   - migrations/
func Create(name string, db *sql.DB) (id string, err error) {
	slog.Info("Creating project", "projectName", name)
	projectInfo := genProjectSeed(name)

	defer func() {
		if err != nil {
			slog.Warn("rollback: remove directory", "error", err)
			os.RemoveAll(projectInfo.Path)
			if delErr := DeleteProjectRow(db, projectInfo.ID); delErr != nil && errors.Is(delErr, sql.ErrNoRows) {
				slog.Error("cleanup: failed to delete row", "error", delErr)
			}
		}
	}()

	if err := AddProject(db, projectInfo.ID, name); err != nil {
		return "", err
	}

	if err := os.MkdirAll(projectInfo.Path, 0755); err != nil {
		return "", err
	}

	if err := createBlankDatabase(projectInfo.Path); err != nil {
		return "", err
	}

	if err := os.MkdirAll(projectInfo.Path+"/migrations", 0755); err != nil {
		return "", err
	}

	if err := UpdateProjectState(db, projectInfo.ID, domain.StateInactive); err != nil {
		return "", err
	}

	return projectInfo.ID, nil
}

var projectNameReg = regexp.MustCompile("[^a-z0-9_-]")

// genProjectSeed generates a unique project path based on the project name and current timestamp.
func genProjectSeed(name string) ProjectInfo {
	projectName := strings.TrimSpace(strings.ToLower(name))
	projectName = strings.Trim(projectNameReg.ReplaceAllString(projectName, ""), "_-")
	if projectName == "" {
		projectName = "project"
	}
	slug := slug(projectName)
	ID := fmt.Sprintf("%s-%s", projectName, slug)
	return ProjectInfo{
		ID:   ID,
		Path: "data/" + ID,
	}
}

// createBlankDatabase creates an empty SQLite database file at the specified project path.
func createBlankDatabase(projectPath string) error {
	dbPath := projectPath + "/database.db"
	dsn := domain.GetDataBaseDSN(dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		return err
	}

	defer db.Close()
	return nil
}
