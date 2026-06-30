package domain

import (
	"database/sql"
	"smartdb/internal/config"
)

// App はシステムデータベースへのポインタと設定を保持する型です
type App struct {
	SystemDB *sql.DB
	Config   *config.Config
}
