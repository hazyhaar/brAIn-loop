//go:build mage

package main

import (
	"fmt"
	"os"

	"github.com/magefile/mage/sh"
)

// Build compiles the brainloop binary
func Build() error {
	fmt.Println("Building brainloop...")
	return sh.Run("go", "build", "-o", "brainloop", "main.go")
}

// Test runs all tests
func Test() error {
	fmt.Println("Running tests...")
	return sh.Run("go", "test", "-v", "./...")
}

// Lint runs golangci-lint
func Lint() error {
	fmt.Println("Running linter...")
	return sh.Run("golangci-lint", "run")
}

// Clean removes binaries and databases
func Clean() error {
	fmt.Println("Cleaning artifacts...")

	files := []string{
		"brainloop",
		"brainloop.input.db",
		"brainloop.input.db-shm",
		"brainloop.input.db-wal",
		"brainloop.lifecycle.db",
		"brainloop.lifecycle.db-shm",
		"brainloop.lifecycle.db-wal",
		"brainloop.output.db",
		"brainloop.output.db-shm",
		"brainloop.output.db-wal",
		"brainloop.metadata.db",
		"brainloop.metadata.db-shm",
		"brainloop.metadata.db-wal",
	}

	for _, file := range files {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Warning: failed to remove %s: %v\n", file, err)
		}
	}

	fmt.Println("Clean completed")
	return nil
}

// Init initializes the databases with schemas
func Init() error {
	fmt.Println("Initializing databases...")
	return sh.Run("go", "run", "main.go", "--init-only")
}

// Dev runs the server in development mode
func Dev() error {
	fmt.Println("Starting brainloop in dev mode...")
	return sh.Run("go", "run", "main.go")
}
