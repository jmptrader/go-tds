package gotds

import (
	"database/sql/driver"
	"strings"
)

// Stmt is a shim at the moment until I implement proper parameter handling
type Stmt struct {
	c         *Conn
	statement string
}

func (c *Conn) Prepare(query string) (driver.Stmt, error) {
	return Stmt{statement: query}, nil
}

func (s Stmt) Close() error {
	// Nothing to do
	return nil
}

func (s Stmt) NumInput() int {
	return strings.Count(s.statement, "?")
}

func (s Stmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.c.Exec(s.statement, args)
}

func (s Stmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.c.Query(s.statement, args)
}
