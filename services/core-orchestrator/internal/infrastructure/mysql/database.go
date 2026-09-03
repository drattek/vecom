package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"core-orchestrator/internal/config"
	_ "github.com/go-sql-driver/mysql"
)

// Querier is satisfied by both *sql.DB and *sql.Tx, so repos built against it can run
// against the plain pool or be re-scoped to a transaction by constructing a new repo
// instance with a *sql.Tx instead — no repo code duplication. Every NewXxxRepository
// constructor takes a Querier, so an application use case can build its whole repo set
// against a *sql.Tx (see WithinTx) and get all-or-nothing writes without any repo change.
//
// Todo método de repo toma ctx y lo propaga al driver por estos métodos ...Context
// (ver ADR infrastructure/decisions/0001). El ctx se enhebra desde el handler
// (r.Context()) o desde el consumer RabbitMQ (su ctx de vida).
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// WithinTx corre fn dentro de una transacción: rollback si fn devuelve error o
// entra en panic, commit si no. fn recibe el *sql.Tx para pasárselo a los
// constructores NewXxxRepository y escopear ahí los repos que la operación
// necesita. Uso:
//
//	err := mysql.WithinTx(ctx, db, func(tx *sql.Tx) error {
//	    brands := mysql.NewBrandsRepository(tx)
//	    products := mysql.NewProductRepository(tx)
//	    // ... varias escrituras; si una falla, ninguna queda aplicada
//	    return nil
//	})
func WithinTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
				err = fmt.Errorf("%w (además falló el rollback: %v)", err, rbErr)
			}
			return
		}
		if cErr := tx.Commit(); cErr != nil {
			err = fmt.Errorf("committing transaction: %w", cErr)
		}
	}()

	return fn(tx)
}

func NewConnection(cfg config.Config) *sql.DB {
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

	// Límites del pool. Sin esto database/sql abre conexiones sin tope (un pico
	// de carga o el endpoint de migración agota max_connections) y las recicla
	// para siempre ("invalid connection" tras un reinicio de MySQL o detrás de
	// un LB). Valores configurables vía MYSQL_* — ver config.Load.
	db.SetMaxOpenConns(cfg.MySQLMaxOpenConns)
	db.SetMaxIdleConns(cfg.MySQLMaxIdleConns)
	db.SetConnMaxLifetime(cfg.MySQLConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.MySQLConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		log.Fatalf("Error pinging database: %v", err)
	}

	log.Println("Successfully connected to MySQL database")
	return db
}
