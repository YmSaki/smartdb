package auth

import (
	"database/sql"
	"log/slog"
)

// BootstrapSystemKey ensures a system-level key (project_id NULL, role=system)
// exists. If presetToken is non-empty (e.g. from SDB_SYSTEM_TOKEN), it is
// hashed and stored as-is instead of generating+logging a random one, so an
// operator can pin the system key via environment/secret management rather
// than relying on the one-time bootstrap log line.
func BootstrapSystemKey(db *sql.DB, presetToken string) error {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE project_id IS NULL AND revoked_at IS NULL`).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	if presetToken != "" {
		hash := HashToken(presetToken)
		_, err := CreateKey(db, nil, "env-system-key", hash, RoleSystem)
		if err != nil {
			return err
		}
		slog.Info("System key bootstrapped from SDB_SYSTEM_TOKEN")
		return nil
	}

	plaintext, err := GenerateToken()
	if err != nil {
		return err
	}
	hash := HashToken(plaintext)

	_, err = CreateKey(db, nil, "bootstrap-system", hash, RoleSystem)
	if err != nil {
		return err
	}

	slog.Warn("=== INITIAL SYSTEM API KEY (save this, shown only once) ===")
	slog.Warn("System API Key", "token", plaintext)
	slog.Warn("============================================================")

	return nil
}
