package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"smartdb/internal/domain"
	"time"
)

// CreateKey inserts a new API key record. projectID nil means a system key.
func CreateKey(db domain.DBTX, projectID *string, name string, tokenHash string, role Role) (string, error) {
	id := generateKeyID()
	_, err := db.Exec(`
		INSERT INTO api_keys (id, project_id, name, token_hash, role)
		VALUES (?, ?, ?, ?, ?)
	`, id, projectID, name, tokenHash, role)
	if err != nil {
		return "", fmt.Errorf("create api key: %w", err)
	}
	return id, nil
}

// GetKeyByHash looks up an active (not revoked) key by its token hash.
func GetKeyByHash(db domain.DBTX, hash string) (*APIKey, error) {
	row := db.QueryRow(`
		SELECT id, project_id, name, token_hash, role, created_at, revoked_at
		FROM api_keys
		WHERE token_hash = ? AND revoked_at IS NULL
	`, hash)
	return scanAPIKey(row)
}

// ListKeys returns API keys for a project (non-nil projectID) or all system keys (nil projectID).
func ListKeys(db domain.DBTX, projectID *string) ([]APIKey, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if projectID == nil {
		rows, err = db.Query(`
			SELECT id, project_id, name, token_hash, role, created_at, revoked_at
			FROM api_keys
			WHERE project_id IS NULL
			ORDER BY created_at DESC
		`)
	} else {
		rows, err = db.Query(`
			SELECT id, project_id, name, token_hash, role, created_at, revoked_at
			FROM api_keys
			WHERE project_id = ?
			ORDER BY created_at DESC
		`, *projectID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		key, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, *key)
	}
	if keys == nil {
		keys = []APIKey{}
	}
	return keys, rows.Err()
}

// RevokeKey marks a key as revoked by setting revoked_at to now.
func RevokeKey(db domain.DBTX, id string) error {
	res, err := db.Exec(`
		UPDATE api_keys SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL
	`, time.Now().UTC(), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// RevokeKeyForProject revokes a key only if it belongs to the specified project.
func RevokeKeyForProject(db domain.DBTX, id string, projectID string) error {
	res, err := db.Exec(`
		UPDATE api_keys SET revoked_at = ? WHERE id = ? AND project_id = ? AND revoked_at IS NULL
	`, time.Now().UTC(), id, projectID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAPIKey(s rowScanner) (*APIKey, error) {
	var key APIKey
	var projectID sql.NullString
	var revokedAt sql.NullTime
	err := s.Scan(
		&key.ID, &projectID, &key.Name, &key.TokenHash,
		&key.Role, &key.CreatedAt, &revokedAt,
	)
	if err != nil {
		return nil, err
	}
	if projectID.Valid {
		key.ProjectID = &projectID.String
	}
	if revokedAt.Valid {
		key.RevokedAt = &revokedAt.Time
	}
	return &key, nil
}

func generateKeyID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
