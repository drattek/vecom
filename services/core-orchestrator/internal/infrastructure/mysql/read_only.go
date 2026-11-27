package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"core-orchestrator/internal/config"
)

// readOnlyMaxConns basta para una herramienta de consulta.
const readOnlyMaxConns = 2

// OpenReadOnlyConnection abre un pool en el que MySQL rechaza toda escritura:
// el driver aplica transaction_read_only a cada conexión.
func OpenReadOnlyConnection(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	db, err := sql.Open("mysql", readOnlyDSN(cfg))
	if err != nil {
		return nil, fmt.Errorf("opening read-only database connection: %w", err)
	}
	db.SetMaxOpenConns(readOnlyMaxConns)
	db.SetConnMaxLifetime(cfg.MySQLConnMaxLifetime)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// Se comprueba la protección en vez de darla por hecha.
	var readOnly bool
	if err := db.QueryRowContext(pingCtx, "SELECT @@session.transaction_read_only").Scan(&readOnly); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connecting read-only to the database: %w", err)
	}
	if !readOnly {
		_ = db.Close()
		return nil, fmt.Errorf("the database session is not read-only")
	}
	return db, nil
}

func readOnlyDSN(cfg config.Config) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Local&tls=preferred&transaction_read_only=1",
		cfg.MySQLUser, cfg.MySQLPassword, cfg.MySQLHost, cfg.MySQLPort, cfg.MySQLDatabase)
}
