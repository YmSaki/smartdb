package backup

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"smartdb/internal/domain"
	"time"

	_ "modernc.org/sqlite"
)

func CreateBackup(dataDir string, projectID string) (string, error) {
	backupDir := filepath.Join(dataDir, projectID, "backups")

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	backupName := fmt.Sprintf("%s.db", timestamp)
	dstPath := filepath.Join(backupDir, backupName)

	srcDSN := domain.GetDataBaseDSN(filepath.Join(dataDir, projectID, "database.db"))
	db, err := sql.Open("sqlite", srcDSN)
	if err != nil {
		return "", fmt.Errorf("open source db: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec("VACUUM INTO ?", dstPath); err != nil {
		os.Remove(dstPath)
		return "", fmt.Errorf("vacuum into: %w", err)
	}

	return backupName, nil
}
