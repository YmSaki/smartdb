package backup

import (
	"os"
	"path/filepath"
	"sort"
)

func PruneBackups(dataDir string, projectID string, maxGenerations int) (int, error) {
	backupDir := filepath.Join(dataDir, projectID, "backups")

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var backups []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".db" {
			backups = append(backups, e.Name())
		}
	}

	sort.Strings(backups)

	if len(backups) <= maxGenerations {
		return 0, nil
	}

	toRemove := backups[:len(backups)-maxGenerations]
	removed := 0
	for _, name := range toRemove {
		if err := os.Remove(filepath.Join(backupDir, name)); err != nil {
			return removed, err
		}
		removed++
	}

	return removed, nil
}
