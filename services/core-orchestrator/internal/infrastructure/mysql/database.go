package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"core-orchestrator/internal/config"
	_ "github.com/go-sql-driver/mysql"
)

// Querier is satisfied by both *sql.DB and *sql.Tx (identical method sets), so repos
// built against it can run against the plain pool or be re-scoped to a transaction by
// constructing a new repo instance with a *sql.Tx instead — no repo code duplication.
type Querier interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

func NewConnection() *sql.DB {
	cfg := config.Load()

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Local&tls=preferred",
		cfg.MySQLUser,
		cfg.MySQLPassword,
		cfg.MySQLHost,
		cfg.MySQLPort,
		cfg.MySQLDatabase,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Error opening database connection: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		log.Fatalf("Error pinging database: %v", err)
	}

	log.Println("Successfully connected to MySQL database")
	return db
}
