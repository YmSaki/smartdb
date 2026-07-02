package backup

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"smartdb/internal/domain"
	"time"

	_ "modernc.org/sqlite"
)

// backupSuffix returns a short random hex string appended to backup
// filenames so two backups requested within the same second don't collide.
func backupSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func CreateBackup(dataDir string, projectID string) (string, error) {
	backupDir := filepath.Join(dataDir, projectID, "backups")

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	backupName := fmt.Sprintf("%s-%s.db", timestamp, backupSuffix())
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
