// Package sqlscript es un MySQL simulado para pruebas: devuelve las filas
// preparadas según la consulta y guarda cada escritura. Sirve para comprobar qué
// columnas toca un flujo; no valida el SQL.
package sqlscript

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// Write es una sentencia de escritura ejecutada. Query viene en una línea.
type Write struct {
	Query string
	Args  []driver.Value
}

// DB se prepara con Rows: fragmento de la consulta → filas que devuelve.
type DB struct {
	Rows map[string][][]driver.Value

	mu     sync.Mutex
	writes []Write
}

// Open devuelve un *sql.DB sobre el simulador; se cierra al terminar la prueba.
func (d *DB) Open(t *testing.T) *sql.DB {
	t.Helper()
	db := sql.OpenDB(d)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// Writes devuelve las escrituras cuya sentencia contiene fragment.
func (d *DB) Writes(fragment string) []Write {
	d.mu.Lock()
	defer d.mu.Unlock()
	var found []Write
	for _, write := range d.writes {
		if strings.Contains(write.Query, fragment) {
			found = append(found, write)
		}
	}
	return found
}

// Column devuelve el valor que un UPDATE asigna a una columna con "col = ?" o
// "col = COALESCE(?, col)".
func (w Write) Column(t *testing.T, name string) driver.Value {
	t.Helper()
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + ` = (?:COALESCE\()?\?`)
	location := pattern.FindStringIndex(w.Query)
	if location == nil {
		t.Fatalf("the write does not set %s: %s", name, w.Query)
	}
	return w.Args[strings.Count(w.Query[:location[0]], "?")]
}

// Sets indica si la sentencia asigna la columna.
func (w Write) Sets(name string) bool {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + ` = `).MatchString(w.Query)
}

var whitespace = regexp.MustCompile(`\s+`)

// Connect y Driver implementan driver.Connector.
func (d *DB) Connect(context.Context) (driver.Conn, error) { return conn{d}, nil }
func (d *DB) Driver() driver.Driver                        { return nil }

type conn struct{ db *DB }

func (c conn) Prepare(query string) (driver.Stmt, error) { return stmt{c.db, query}, nil }
func (c conn) Close() error                              { return nil }
func (c conn) Begin() (driver.Tx, error)                 { return nil, errors.New("transactions not supported") }

type stmt struct {
	db    *DB
	query string
}

func (s stmt) Close() error  { return nil }
func (s stmt) NumInput() int { return -1 }

func (s stmt) Exec(args []driver.Value) (driver.Result, error) {
	s.db.mu.Lock()
	defer s.db.mu.Unlock()
	s.db.writes = append(s.db.writes, Write{
		Query: strings.TrimSpace(whitespace.ReplaceAllString(s.query, " ")),
		Args:  args,
	})
	return result{}, nil
}

type result struct{}

func (result) LastInsertId() (int64, error) { return 1, nil }
func (result) RowsAffected() (int64, error) { return 1, nil }

func (s stmt) Query([]driver.Value) (driver.Rows, error) {
	for fragment, found := range s.db.Rows {
		if strings.Contains(s.query, fragment) {
			return &rows{values: found}, nil
		}
	}
	return &rows{}, nil
}

type rows struct {
	values [][]driver.Value
	next   int
}

func (r *rows) Columns() []string {
	if len(r.values) == 0 {
		return []string{"empty"}
	}
	return make([]string, len(r.values[0]))
}

func (r *rows) Close() error { return nil }

func (r *rows) Next(dest []driver.Value) error {
	if r.next >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.next])
	r.next++
	return nil
}
