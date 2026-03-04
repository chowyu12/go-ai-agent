package mysql

import (
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/chowyu12/go-ai-agent/internal/config"
)

type MySQLStore struct {
	db *sql.DB
}

func New(cfg config.DatabaseConfig) (*MySQLStore, error) {
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &MySQLStore{db: db}, nil
}

func (s *MySQLStore) Close() error {
	return s.db.Close()
}
