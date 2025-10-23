// pkg/database/mysql.go
package database

import (
	"fmt"
	"time"

	"game-leaderboard/pkg/logger"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func NewMySQLConnection(dsn string, maxConns int) (*sqlx.DB, error) {
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mysql: %w", err)
	}

	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(maxConns / 2)
	db.SetConnMaxLifetime(30 * time.Minute)

	logger.NewLogger("database").Info("MySQL connection established")
	return db, nil
}
