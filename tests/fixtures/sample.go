package sample

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

// User represents a user in the system
type User struct {
	ID       string
	Username string
	Email    string
}

// Database provides database operations
type Database struct {
	db *sql.DB
}

// NewDatabase creates a new database instance
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return &Database{db: db}, nil
}

// CreateUser creates a new user
func (d *Database) CreateUser(user *User) error {
	_, err := d.db.Exec(`
		INSERT INTO users (id, username, email)
		VALUES (?, ?, ?)
	`, user.ID, user.Username, user.Email)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	log.Printf("User created: %s", user.Username)
	return nil
}

// GetUser retrieves a user by ID
func (d *Database) GetUser(id string) (*User, error) {
	var user User
	err := d.db.QueryRow(`
		SELECT id, username, email FROM users WHERE id = ?
	`, id).Scan(&user.ID, &user.Username, &user.Email)

	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}
