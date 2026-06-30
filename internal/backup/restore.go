package backup

import (
	"fmt"
	"os"
	"path/filepath"
)

func RestoreBackup(dataDir string, projectID string, backupName string) error {
	backupPath := filepath.Join(dataDir, projectID, "backups", backupName)
	dbPath := filepath.Join(dataDir, projectID, "database.db")

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", backupName)
	}

	tmpPath := dbPath + ".restoring"
	if err := copyFile(backupPath, tmpPath); err != nil {
		return fmt.Errorf("copy backup: %w", err)
	}

	if err := os.Rename(tmpPath, dbPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace database: %w", err)
	}

	return nil
}
