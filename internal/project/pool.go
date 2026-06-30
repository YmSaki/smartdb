package project

import (
	"database/sql"
	"sync"

	_ "modernc.org/sqlite"
)

type ConnectionPool struct {
	mu    sync.Mutex
	conns map[string]*sql.DB
}

func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		conns: make(map[string]*sql.DB),
	}
}

func (p *ConnectionPool) Get(dataDir string, projectID string) (*sql.DB, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if db, ok := p.conns[projectID]; ok {
		return db, nil
	}

	dsn := GetProjectDNS(dataDir, projectID)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	p.conns[projectID] = db
	return db, nil
}

func (p *ConnectionPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, db := range p.conns {
		db.Close()
		delete(p.conns, id)
	}
}
