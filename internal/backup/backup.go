package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func CreateBackup(dataDir string, projectID string) (string, error) {
	srcPath := filepath.Join(dataDir, projectID, "database.db")
	backupDir := filepath.Join(dataDir, projectID, "backups")

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	backupName := fmt.Sprintf("%s.db", timestamp)
	dstPath := filepath.Join(backupDir, backupName)

	if err := copyFile(srcPath, dstPath); err != nil {
		return "", fmt.Errorf("copy database: %w", err)
	}

	return backupName, nil
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
