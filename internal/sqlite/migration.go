package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"smartdb/internal/domain"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

type Migration struct {
	Version  string
	UpFile   string
	DownFile string
}

type MigrationType string

const (
	MigrationTypeUp   MigrationType = "up"
	MigrationTypeDown MigrationType = "down"
)

var versionPrefixReg = regexp.MustCompile(`^(\d+)`)

func LoadMigration(db domain.DBTX, migrationsPath string) (map[string]*Migration, error) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS __migrations (
			version text PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, err
	}

	files, err := os.ReadDir(migrationsPath)
	if err != nil {
		return nil, err
	}
	migrations := map[string]*Migration{}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		var state MigrationType
		var version string
		switch {
		case strings.HasSuffix(file.Name(), ".down.sql"):
			version = strings.TrimSuffix(file.Name(), ".down.sql")
			state = MigrationTypeDown
		case strings.HasSuffix(file.Name(), ".up.sql"):
			version = strings.TrimSuffix(file.Name(), ".up.sql")
			state = MigrationTypeUp
		case strings.HasSuffix(file.Name(), ".sql"):
			version = strings.TrimSuffix(file.Name(), ".sql")
			state = MigrationTypeUp
		}

		if version == "" {
			continue
		}

		m := migrations[version]
		if m == nil {
			m = &Migration{Version: version}
			migrations[version] = m
		}

		switch state {
		case MigrationTypeDown:
			m.DownFile = file.Name()
		case MigrationTypeUp:
			m.UpFile = file.Name()
		}
	}
	return migrations, nil
}

func ApplyMigration(db *sql.DB, migrationsPath string, migrations []Migration) error {
	sort.Slice(migrations, func(i, j int) bool {
		numI, errI := strconv.Atoi(versionPrefixReg.FindString(migrations[i].Version))
		numJ, errJ := strconv.Atoi(versionPrefixReg.FindString(migrations[j].Version))

		if errI == nil && errJ == nil && numI != numJ {
			return numI < numJ
		}

		return migrations[i].Version < migrations[j].Version
	})

	for _, migration := range migrations {
		alreadyApplied, err := isMigrationApplied(db, migration.Version)
		if err != nil {
			return err
		} else if alreadyApplied {
			continue
		}

		if err := executeMigration(db, migration.Version, filepath.Join(migrationsPath, migration.UpFile), MigrationTypeUp); err != nil {
			return err
		}
	}
	return nil
}

func Migrate(db *sql.DB, migrationsPath string, migrationType MigrationType) error {
	mapMigrations, err := LoadMigration(db, migrationsPath)
	if err != nil {
		return err
	}

	switch migrationType {
	case MigrationTypeDown:
		downVersion, err := getFinalMigrationAppliedVersion(db)
		if err != nil {
			return errors.Join(err, fmt.Errorf("There are no items to migrate."))
		}

		if mapMigrations[downVersion].DownFile == "" {
			return fmt.Errorf("migration %s has not down file", downVersion)
		}

		if err := executeMigration(db, downVersion, filepath.Join(migrationsPath, mapMigrations[downVersion].DownFile), MigrationTypeDown); err != nil {
			return err
		}
	case MigrationTypeUp:

		listMigrations := make([]Migration, 0, len(mapMigrations))
		for _, m := range mapMigrations {
			if m != nil {
				listMigrations = append(listMigrations, *m)
			}
		}
		if err := ApplyMigration(db, migrationsPath, listMigrations); err != nil {
			return err
		}
	}

	return nil
}

func isMigrationApplied(db domain.DBTX, version string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM __migrations WHERE version = ?", version).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func getFinalMigrationAppliedVersion(db domain.DBTX) (string, error) {
	var version string
	err := db.QueryRow(`
		SELECT version
		FROM __migrations
		ORDER BY applied_at DESC, version DESC
		LIMIT 1
	`).Scan(&version)

	if err != nil {
		return "", err
	}
	return version, nil
}

func executeMigration(db *sql.DB, version string, migrationFile string, migrationType MigrationType) error {
	content, err := os.ReadFile(migrationFile)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec(string(content))
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			return errors.Join(err, err2)
		}
		return err
	}

	if migrationType == MigrationTypeUp {
		_, err = tx.Exec("INSERT INTO __migrations (version) VALUES (?)", version)
		if err != nil {
			err2 := tx.Rollback()
			if err2 != nil {
				return errors.Join(err, err2)
			}
			return err
		}
	} else {
		_, err = tx.Exec("DELETE FROM __migrations WHERE version = ?", version)
		if err != nil {
			err2 := tx.Rollback()
			if err2 != nil {
				return errors.Join(err, err2)
			}
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}
