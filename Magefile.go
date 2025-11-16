//go:build mage
// +build mage

package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	_ "modernc.org/sqlite"
)

// Build builds the worker binary
func Build() error {
	mg.Deps(Lint, Test)

	workerName := getWorkerName()
	fmt.Printf("Building %s...\n", workerName)

	return sh.RunV("go", "build",
		"-o", "bin/"+workerName,
		"-ldflags", "-s -w",
		".")
}

// Test runs all tests (Go + SQL)
func Test() error {
	mg.Deps(TestGo, TestSQL)
	fmt.Println("✅ All tests passed")
	return nil
}

// TestGo runs Go unit tests
func TestGo() error {
	fmt.Println("Running Go tests...")
	return sh.RunV("go", "test", "-v", "-race", "-coverprofile=coverage.out", "./...")
}

// TestSQL runs SQL functional tests
func TestSQL() error {
	fmt.Println("Running SQL tests...")
	return sh.RunV("bash", "tests/sql/run_sql_tests.sh")
}

// Lint runs golangci-lint with HOROS custom linters
func Lint() error {
	fmt.Println("Running linters...")
	return sh.RunV("golangci-lint", "run", "--config", ".golangci.yml")
}

// LintFix runs linters with auto-fix
func LintFix() error {
	fmt.Println("Running linters with auto-fix...")
	return sh.RunV("golangci-lint", "run", "--fix", "--config", ".golangci.yml")
}

// Validate checks HOROS DSL compliance (structure + schema + contracts)
func Validate() error {
	mg.Deps(ValidateStructure, ValidateSchemas, ValidateContracts, ValidateDimensions)
	fmt.Println("✅ HOROS DSL validation passed")
	return nil
}

// ValidateStructure checks 4-BDD file presence or workflow structure
func ValidateStructure() error {
	fmt.Println("🔍 Checking structure...")

	projectName := getWorkerName()
	projectType := detectProjectType()

	switch projectType {
	case "workflow":
		return validateWorkflowStructure(projectName)
	case "worker":
		return validateWorkerStructure(projectName)
	default:
		return fmt.Errorf("❌ Unknown project type")
	}
}

func validateWorkflowStructure(projectName string) error {
	// Workflows must have: flow.sql, workflow.toml, workers/
	requiredFiles := []string{"flow.sql", "workflow.toml"}

	for _, file := range requiredFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return fmt.Errorf("❌ WORKFLOW VIOLATION: Missing %s", file)
		}
	}

	// Check workers/ directory exists
	if _, err := os.Stat("workers"); os.IsNotExist(err) {
		return fmt.Errorf("❌ WORKFLOW VIOLATION: Missing workers/ directory")
	}

	fmt.Println("  ✓ Workflow structure valid (flow.sql + workflow.toml + workers/)")
	return nil
}

func validateWorkerStructure(projectName string) error {
	// Workers must have 4-BDD
	requiredDBs := []string{
		projectName + ".input.db",
		projectName + ".lifecycle.db",
		projectName + ".output.db",
		projectName + ".metadata.db",
	}

	for _, dbPath := range requiredDBs {
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("❌ HOROS VIOLATION: Missing database %s", dbPath)
		}
	}

	fmt.Println("  ✓ All 4 databases present")
	return nil
}

// ValidateSchemas checks table counts and required tables
func ValidateSchemas() error {
	fmt.Println("🔍 Checking database schemas...")

	projectName := getWorkerName()
	projectType := detectProjectType()

	// Skip schema validation for workflows (they don't have databases)
	if projectType == "workflow" {
		fmt.Println("  ℹ️  Workflow detected, skipping database schema validation")
		return nil
	}

	// Check if HOROS-FLOW (38 tables) or standard (34 tables)
	isFlow := checkHOROSFlow(projectName)

	// Expected table counts
	expectedCounts := map[string]int{
		projectName + ".input.db":    5,
		projectName + ".lifecycle.db": 19,
		projectName + ".output.db":   4,
		projectName + ".metadata.db": 6,
	}

	if isFlow {
		expectedCounts[projectName+".input.db"] = 6
		expectedCounts[projectName+".lifecycle.db"] = 21
		// Note: output.db garde 4 tables (colonnes workflow ajoutées dans results)
		fmt.Println("  ℹ️  HOROS-FLOW worker detected (37 tables)")
	} else {
		fmt.Println("  ℹ️  Standard HOROS worker detected (34 tables)")
	}

	// Required tables by database
	requiredTables := map[string][]string{
		projectName + ".lifecycle.db": {"processed_log", "config", "ego_index"},
		projectName + ".output.db":    {"results", "heartbeat", "metrics"},
		projectName + ".metadata.db":  {"poisonpill"},
	}

	for dbPath, expectedCount := range expectedCounts {
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			return fmt.Errorf("❌ Failed to open %s: %w", dbPath, err)
		}
		defer db.Close()

		// Count tables
		var tableCount int
		err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&tableCount)
		if err != nil {
			return fmt.Errorf("❌ Failed to count tables in %s: %w", dbPath, err)
		}

		if tableCount != expectedCount {
			fmt.Printf("  ⚠️  %s has %d tables, expected %d\n",
				filepath.Base(dbPath), tableCount, expectedCount)
		} else {
			fmt.Printf("  ✓ %s: %d tables\n", filepath.Base(dbPath), tableCount)
		}

		// Check required tables
		if tables, ok := requiredTables[dbPath]; ok {
			for _, tableName := range tables {
				var exists bool
				err = db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`,
					tableName).Scan(&exists)

				if err != nil {
					return fmt.Errorf("❌ Failed to check table %s in %s: %w", tableName, dbPath, err)
				}

				if !exists {
					return fmt.Errorf("❌ HOROS VIOLATION: Missing required table '%s' in %s",
						tableName, dbPath)
				}
			}
		}
	}

	return nil
}

// ValidateContracts checks protobuf schemas and upstream declarations
func ValidateContracts() error {
	fmt.Println("🔍 Checking contracts...")

	projectName := getWorkerName()
	projectType := detectProjectType()

	// Skip for workflows
	if projectType == "workflow" {
		fmt.Println("  ℹ️  Workflow detected, checking flow.sql topology instead")
		return validateWorkflowTopology()
	}

	lifecycleDB, err := sql.Open("sqlite", projectName+".lifecycle.db")
	if err != nil {
		return err
	}
	defer lifecycleDB.Close()

	// 1. Check if workflow enabled
	var workflowEnabled string
	err = lifecycleDB.QueryRow(`SELECT value FROM config WHERE key='workflow_enabled'`).Scan(&workflowEnabled)

	if err == nil && workflowEnabled == "true" {
		// Must have workflow_name and step_name
		var workflowName, stepName string
		err := lifecycleDB.QueryRow(`SELECT value FROM config WHERE key='workflow_name'`).Scan(&workflowName)
		if err != nil {
			return fmt.Errorf("❌ HOROS VIOLATION: workflow_enabled=true but no workflow_name in config")
		}

		err = lifecycleDB.QueryRow(`SELECT value FROM config WHERE key='step_name'`).Scan(&stepName)
		if err != nil {
			return fmt.Errorf("❌ HOROS VIOLATION: workflow_enabled=true but no step_name in config")
		}

		fmt.Printf("  ✓ Workflow config: %s / %s\n", workflowName, stepName)
	}

	// 2. Check upstream dependencies declared
	inputDB, err := sql.Open("sqlite", projectName+".input.db")
	if err != nil {
		return err
	}
	defer inputDB.Close()

	var depCount int
	inputDB.QueryRow(`SELECT COUNT(*) FROM input_dependencies`).Scan(&depCount)
	fmt.Printf("  ✓ %d upstream dependencies declared\n", depCount)

	// 3. Check proto files exist if output.db has results
	outputDB, err := sql.Open("sqlite", projectName+".output.db")
	if err != nil {
		return err
	}
	defer outputDB.Close()

	var resultCount int
	outputDB.QueryRow(`SELECT COUNT(*) FROM results`).Scan(&resultCount)

	if resultCount > 0 {
		if _, err := os.Stat("proto/output.proto"); os.IsNotExist(err) {
			fmt.Println("  ⚠️  WARNING: output.db has results but no proto/output.proto found")
		} else {
			fmt.Println("  ✓ proto/output.proto exists")
		}
	}

	return nil
}

// ValidateDimensions checks 15 universal dimensions are documented
func ValidateDimensions() error {
	fmt.Println("🔍 Checking 15 universal dimensions...")

	projectName := getWorkerName()
	projectType := detectProjectType()

	// Skip for workflows (no lifecycle.db)
	if projectType == "workflow" {
		fmt.Println("  ℹ️  Workflow detected, dimensions validated per worker")
		return nil
	}

	lifecycleDB, err := sql.Open("sqlite", projectName+".lifecycle.db")
	if err != nil {
		return err
	}
	defer lifecycleDB.Close()

	dimensions := []string{
		"dim_origines", "dim_composition", "dim_finalites", "dim_interactions",
		"dim_dependances", "dim_temporalite", "dim_cardinalite", "dim_observabilite",
		"dim_reversibilite", "dim_congruence", "dim_anticipation", "dim_granularite",
		"dim_conditionnalite", "dim_autorite", "dim_mutabilite",
	}

	missingDims := []string{}
	for _, dim := range dimensions {
		var exists bool
		err = lifecycleDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM ego_index WHERE key=?)`, dim).Scan(&exists)
		if err != nil || !exists {
			missingDims = append(missingDims, dim)
		}
	}

	if len(missingDims) > 0 {
		return fmt.Errorf("❌ HOROS VIOLATION: Missing dimensions in ego_index: %v", missingDims)
	}

	fmt.Println("  ✓ All 15 dimensions documented in ego_index")
	return nil
}

// Check runs full HOROS validation + build + test
func Check() error {
	mg.Deps(Validate, Lint, Test, Build)
	fmt.Println("✅ Full HOROS compliance check passed")
	return nil
}

// InitDB initializes the 4-BDD databases with schemas
func InitDB() error {
	fmt.Println("Initializing 4-BDD databases...")

	workerName := getWorkerName()
	schemas := map[string]string{
		workerName + ".input.db":    "schemas/input_schema.sql",
		workerName + ".lifecycle.db": "schemas/lifecycle_schema.sql",
		workerName + ".output.db":   "schemas/output_schema.sql",
		workerName + ".metadata.db": "schemas/metadata_schema.sql",
	}

	for dbPath, schemaPath := range schemas {
		if err := initDatabase(dbPath, schemaPath); err != nil {
			return fmt.Errorf("failed to init %s: %w", dbPath, err)
		}
		fmt.Printf("  ✓ Initialized %s\n", dbPath)
	}

	return nil
}

// Clean removes build artifacts
func Clean() error {
	fmt.Println("Cleaning...")
	os.RemoveAll("bin")
	os.RemoveAll("coverage.out")
	return nil
}

// Run builds and runs the worker
func Run() error {
	mg.Deps(Build)

	workerName := getWorkerName()
	return sh.RunV("./bin/" + workerName)
}

// ============================================================================
// Helpers
// ============================================================================

func getWorkerName() string {
	dir, _ := os.Getwd()
	return filepath.Base(dir)
}

func initDatabase(dbPath, schemaPath string) error {
	// Read schema
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}

	// Open database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Apply pragmas
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		"PRAGMA cache_size=-64000",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("failed to apply pragma: %w", err)
		}
	}

	// Execute schema
	if _, err := db.Exec(string(schema)); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return nil
}

// detectProjectType returns "workflow" or "worker"
func detectProjectType() string {
	// Workflows have flow.sql + workflow.toml + workers/
	hasFlowSQL := fileExists("flow.sql")
	hasWorkflowToml := fileExists("workflow.toml")
	hasWorkersDir := dirExists("workers")

	if hasFlowSQL && hasWorkflowToml && hasWorkersDir {
		return "workflow"
	}

	return "worker"
}

func checkHOROSFlow(workerName string) bool {
	// Method 1: Check if workflow_enabled in config
	lifecycleDB, err := sql.Open("sqlite", workerName+".lifecycle.db")
	if err != nil {
		return false
	}
	defer lifecycleDB.Close()

	var workflowEnabled string
	err = lifecycleDB.QueryRow(`SELECT value FROM config WHERE key='workflow_enabled'`).Scan(&workflowEnabled)

	if err == nil && strings.ToLower(workflowEnabled) == "true" {
		return true
	}

	// Method 2: Check if input_correlations table exists (HOROS-FLOW specific)
	inputDB, err := sql.Open("sqlite", workerName+".input.db")
	if err != nil {
		return false
	}
	defer inputDB.Close()

	var hasCorrelations bool
	err = inputDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='input_correlations')`).Scan(&hasCorrelations)

	return err == nil && hasCorrelations
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func validateWorkflowTopology() error {
	// Read flow.sql and check it contains workflow_topology INSERTs
	content, err := os.ReadFile("flow.sql")
	if err != nil {
		return fmt.Errorf("❌ Failed to read flow.sql: %w", err)
	}

	flowSQL := string(content)

	// Check for workflow_topology table references
	if !strings.Contains(flowSQL, "workflow_topology") {
		return fmt.Errorf("❌ WORKFLOW VIOLATION: flow.sql must contain workflow_topology INSERTs")
	}

	// Check for required columns
	requiredColumns := []string{"edge_id", "workflow_name", "from_worker", "to_worker"}
	for _, col := range requiredColumns {
		if !strings.Contains(flowSQL, col) {
			return fmt.Errorf("❌ WORKFLOW VIOLATION: flow.sql missing column '%s' in workflow_topology", col)
		}
	}

	fmt.Println("  ✓ flow.sql topology valid")
	return nil
}
