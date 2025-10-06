package database

import (
	"database/sql"
	"log"
)

// Executor provides database execution capabilities
type Executor struct {
	db *sql.DB
}

// NewExecutor creates a new database executor
func NewExecutor(db *sql.DB) *Executor {
	return &Executor{db: db}
}

// Query executes a SELECT query and returns rows
func (e *Executor) Query(query string, args ...interface{}) (*sql.Rows, error) {
	log.Printf("Executing query: %s with args: %v", query, args)
	return e.db.Query(query, args...)
}

// Exec executes a non-SELECT query and returns the result
func (e *Executor) Exec(query string, args ...interface{}) (sql.Result, error) {
	log.Printf("Executing query: %s with args: %v", query, args)
	return e.db.Exec(query, args...)
}

// Close closes the database connection
func (e *Executor) Close() error {
	return e.db.Close()
}
