package auth

import (
	"database/sql"
	"log/slog"
)

func BootstrapAdminKey(db *sql.DB) error {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE project_id IS NULL AND revoked_at IS NULL`).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	plaintext, err := GenerateToken()
	if err != nil {
		return err
	}
	hash := HashToken(plaintext)

	_, err = CreateKey(db, nil, "bootstrap-admin", hash, RoleAdmin)
	if err != nil {
		return err
	}

	slog.Warn("=== INITIAL ADMIN API KEY (save this, shown only once) ===")
	slog.Warn("Admin API Key", "token", plaintext)
	slog.Warn("============================================================")

	return nil
}
