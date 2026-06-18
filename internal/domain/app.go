package domain

import "database/sql"

// App はシステムデータベースへのポインタを保持する型です
type App struct {
	SystemDB *sql.DB
}
