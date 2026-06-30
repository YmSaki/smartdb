package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func RestoreBackup(dataDir string, projectID string, backupName string) error {
	if strings.Contains(backupName, "/") || strings.Contains(backupName, "\\") || strings.Contains(backupName, "..") {
		return fmt.Errorf("invalid backup name: %s", backupName)
	}

	backupPath := filepath.Join(dataDir, projectID, "backups", backupName)
	expectedDir := filepath.Clean(filepath.Join(dataDir, projectID, "backups"))
	if !strings.HasPrefix(filepath.Clean(backupPath), expectedDir+string(filepath.Separator)) {
		return fmt.Errorf("invalid backup name: %s", backupName)
	}

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

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
