package database

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

// Helper provides database initialization utilities
type Helper struct{}

// New creates a new database helper
func New() *Helper {
	return &Helper{}
}

// InitInputDB initializes the input database with schema
func (h *Helper) InitInputDB(path string) (*sql.DB, error) {
	db, err := h.openDB(path)
	if err != nil {
		return nil, err
	}

	schema, err := os.ReadFile("brainloop.input_schema.sql")
	if err != nil {
		return nil, fmt.Errorf("failed to read input schema: %w", err)
	}

	if _, err := db.Exec(string(schema)); err != nil {
		return nil, fmt.Errorf("failed to execute input schema: %w", err)
	}

	return db, nil
}

// InitLifecycleDB initializes the lifecycle database with schema
func (h *Helper) InitLifecycleDB(path string) (*sql.DB, error) {
	db, err := h.openDB(path)
	if err != nil {
		return nil, err
	}

	schema, err := os.ReadFile("brainloop.lifecycle_schema.sql")
	if err != nil {
		return nil, fmt.Errorf("failed to read lifecycle schema: %w", err)
	}

	if _, err := db.Exec(string(schema)); err != nil {
		return nil, fmt.Errorf("failed to execute lifecycle schema: %w", err)
	}

	return db, nil
}

// InitOutputDB initializes the output database with schema
func (h *Helper) InitOutputDB(path string) (*sql.DB, error) {
	db, err := h.openDB(path)
	if err != nil {
		return nil, err
	}

	schema, err := os.ReadFile("brainloop.output_schema.sql")
	if err != nil {
		return nil, fmt.Errorf("failed to read output schema: %w", err)
	}

	if _, err := db.Exec(string(schema)); err != nil {
		return nil, fmt.Errorf("failed to execute output schema: %w", err)
	}

	return db, nil
}

// InitMetadataDB initializes the metadata database with schema
func (h *Helper) InitMetadataDB(path string) (*sql.DB, error) {
	db, err := h.openDB(path)
	if err != nil {
		return nil, err
	}

	schema, err := os.ReadFile("brainloop.metadata_schema.sql")
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata schema: %w", err)
	}

	if _, err := db.Exec(string(schema)); err != nil {
		return nil, fmt.Errorf("failed to execute metadata schema: %w", err)
	}

	return db, nil
}

// openDB opens a SQLite database with standard HOROS pragmas
func (h *Helper) openDB(path string) (*sql.DB, error) {
	connString := fmt.Sprintf("%s?_journal_mode=WAL&_synchronous=NORMAL&_foreign_keys=ON&_busy_timeout=5000&_cache_size=-64000", path)

	db, err := sql.Open("sqlite", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database %s: %w", path, err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database %s: %w", path, err)
	}

	// Set additional pragmas explicitly
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA cache_size = -64000",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("failed to set pragma: %w", err)
		}
	}

	return db, nil
}
