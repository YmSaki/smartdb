package project

import (
	"database/sql"
	"fmt"
	"log"
	"os"
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
func Create(name string, db *sql.DB) (string, error) {
	log.Printf("Creating project: %s\n", name)
	projectInfo := getProjectPath(name)

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

// getProjectPath generates a unique project path based on the project name and current timestamp.
func getProjectPath(name string) ProjectInfo {
	projectName := strings.ToLower(name)
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
		return err
	}

	if err := db.Ping(); err != nil {
		return err
	}

	defer db.Close()
	return nil
}
