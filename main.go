package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "modernc.org/sqlite"

	"brainloop/internal/database"
	"brainloop/internal/mcp"
)

type Worker struct {
	workerID    string
	inputDB     *sql.DB
	lifecycleDB *sql.DB
	outputDB    *sql.DB
	metadataDB  *sql.DB
	mcpServer   *mcp.Server
	ctx         context.Context
	cancel      context.CancelFunc
}

func main() {
	// Validate working directory - HOROS pattern compliance
	if err := validateWorkingDirectory(); err != nil {
		log.Fatalf("Working directory validation failed: %v", err)
	}

	// Check for single instance
	lockFile := "brainloop.lock"
	if err := checkSingleInstance(lockFile); err != nil {
		log.Fatalf("Single instance check failed: %v", err)
	}
	defer os.Remove(lockFile)

	// Initialize worker
	w := &Worker{
		workerID: fmt.Sprintf("brainloop-%d", time.Now().Unix()),
	}

	// Context with cancellation
	w.ctx, w.cancel = context.WithCancel(context.Background())

	// Initialize 4 databases
	if err := w.initDatabases(); err != nil {
		log.Fatalf("Failed to initialize databases: %v", err)
	}
	defer w.closeDatabases()

	// Record startup event
	recordEvent(w.metadataDB, "startup", fmt.Sprintf("Worker %s starting", w.workerID))

	// Initialize MCP server
	mcpServer, err := mcp.NewServer(w.lifecycleDB, w.outputDB, w.metadataDB)
	if err != nil {
		log.Fatalf("Failed to initialize MCP server: %v", err)
	}
	w.mcpServer = mcpServer

	// Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Start MCP server (stdio)
	go func() {
		if err := w.mcpServer.Serve(os.Stdin, os.Stdout); err != nil {
			log.Printf("MCP server error: %v", err)
		}
	}()

	// Heartbeat loop
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	log.Printf("Worker %s started successfully", w.workerID)

	for {
		select {
		case <-ticker.C:
			w.sendHeartbeat("running")
		case sig := <-sigChan:
			log.Printf("Received signal %v, shutting down gracefully...", sig)
			w.shutdown()
			return
		case <-w.ctx.Done():
			return
		}
	}
}

func (w *Worker) initDatabases() error {
	var err error

	dbHelper := database.New()

	w.inputDB, err = dbHelper.InitInputDB("brainloop.input.db")
	if err != nil {
		return fmt.Errorf("input DB: %w", err)
	}

	w.lifecycleDB, err = dbHelper.InitLifecycleDB("brainloop.lifecycle.db")
	if err != nil {
		return fmt.Errorf("lifecycle DB: %w", err)
	}

	w.outputDB, err = dbHelper.InitOutputDB("brainloop.output.db")
	if err != nil {
		return fmt.Errorf("output DB: %w", err)
	}

	w.metadataDB, err = dbHelper.InitMetadataDB("brainloop.metadata.db")
	if err != nil {
		return fmt.Errorf("metadata DB: %w", err)
	}

	log.Println("All 4 databases initialized successfully")
	return nil
}

func (w *Worker) sendHeartbeat(status string) {
	var sessionsActive, sessionsCompleted int
	var cacheHitRate float64

	// Query metrics from lifecycle DB
	w.lifecycleDB.QueryRow("SELECT COUNT(*) FROM sessions WHERE status = 'pending_audit'").Scan(&sessionsActive)
	w.lifecycleDB.QueryRow("SELECT COUNT(*) FROM sessions WHERE status = 'committed'").Scan(&sessionsCompleted)

	// Calculate cache hit rate
	var cacheHits, cacheTotal int
	w.lifecycleDB.QueryRow("SELECT COUNT(*) FROM reader_cache WHERE expires_at > ?", time.Now().Unix()).Scan(&cacheHits)
	w.lifecycleDB.QueryRow("SELECT COUNT(*) FROM processed_log WHERE operation = 'read'").Scan(&cacheTotal)
	if cacheTotal > 0 {
		cacheHitRate = float64(cacheHits) / float64(cacheTotal)
	}

	// Cleanup old workers (zombies) - remove workers inactive for > 2 minutes
	cutoffTime := time.Now().Unix() - 120
	_, err := w.outputDB.Exec("DELETE FROM heartbeat WHERE timestamp < ? AND worker_id != ?", cutoffTime, w.workerID)
	if err != nil {
		log.Printf("Failed to cleanup old heartbeats: %v", err)
	}

	// Insert heartbeat
	_, err = w.outputDB.Exec(`
		INSERT OR REPLACE INTO heartbeat
		(worker_id, timestamp, status, sessions_active, sessions_completed, cache_hit_rate)
		VALUES (?, ?, ?, ?, ?, ?)
	`, w.workerID, time.Now().Unix(), status, sessionsActive, sessionsCompleted, cacheHitRate)

	if err != nil {
		log.Printf("Failed to send heartbeat: %v", err)
	}
}

func (w *Worker) shutdown() {
	log.Println("Starting graceful shutdown...")

	// Phase 1: Stop accepting new work
	w.sendHeartbeat("shutting_down")
	w.cancel()

	// Phase 2: Wait for ongoing operations (max 55s)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer shutdownCancel()

	// Stop MCP server
	if w.mcpServer != nil {
		if err := w.mcpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("MCP server shutdown error: %v", err)
		}
	}

	// Wait a bit for operations to complete
	time.Sleep(2 * time.Second)

	// Phase 3: WAL checkpoint
	log.Println("Checkpointing WAL files...")
	for name, db := range map[string]*sql.DB{
		"input":     w.inputDB,
		"lifecycle": w.lifecycleDB,
		"output":    w.outputDB,
		"metadata":  w.metadataDB,
	} {
		if db != nil {
			if _, err := db.Exec("PRAGMA wal_checkpoint(RESTART)"); err != nil {
				log.Printf("WAL checkpoint error (%s): %v", name, err)
			}
		}
	}

	// Final logs
	recordEvent(w.metadataDB, "shutdown", fmt.Sprintf("Worker %s shutdown gracefully", w.workerID))
	log.Println("Graceful shutdown completed")
	log.Printf("Worker %s shutdown with status: graceful", w.workerID)
}

func (w *Worker) closeDatabases() {
	for name, db := range map[string]*sql.DB{
		"input":     w.inputDB,
		"lifecycle": w.lifecycleDB,
		"output":    w.outputDB,
		"metadata":  w.metadataDB,
	} {
		if db != nil {
			if err := db.Close(); err != nil {
				log.Printf("Error closing %s database: %v", name, err)
			}
		}
	}
}

// recordEvent is a simple telemetry helper
func recordEvent(db *sql.DB, eventType, description string) {
	if db == nil {
		return
	}

	_, err := db.Exec(`
		INSERT INTO telemetry_events (timestamp, event_type, description)
		VALUES (?, ?, ?)
	`, time.Now().Unix(), eventType, description)

	if err != nil {
		log.Printf("Failed to record event: %v", err)
	}
}

// recordMetric is a simple metrics helper
func recordMetric(db *sql.DB, metricName string, metricValue float64) {
	if db == nil {
		return
	}

	_, err := db.Exec(`
		INSERT INTO metrics (timestamp, metric_name, metric_value)
		VALUES (?, ?, ?)
	`, time.Now().Unix(), metricName, metricValue)

	if err != nil {
		log.Printf("Failed to record metric: %v", err)
	}
}

// checkSingleInstance ensures only one brainloop instance runs
func checkSingleInstance(lockFile string) error {
	// Try to create lock file
	file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			// Lock file exists, check if process is still running
			if content, readErr := os.ReadFile(lockFile); readErr == nil {
				var pid int
				if _, scanErr := fmt.Sscanf(string(content), "%d", &pid); scanErr == nil {
					// Check if PID exists
					if processExists(pid) {
						return fmt.Errorf("brainloop is already running with PID %d", pid)
					}
					// Stale lock file, remove it
					os.Remove(lockFile)
				}
			}
			// Retry after removing stale lock
			file, err = os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	defer file.Close()

	// Write current PID to lock file
	pid := os.Getpid()
	_, err = file.WriteString(fmt.Sprintf("%d\n", pid))
	return err
}

// processExists checks if a process with given PID exists
func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// validateWorkingDirectory ensures we're running from the correct project directory
func validateWorkingDirectory() error {
	// Check for required database files in current directory
	requiredFiles := []string{
		"brainloop.input_schema.sql",
		"brainloop.lifecycle_schema.sql",
		"brainloop.output_schema.sql",
		"brainloop.metadata_schema.sql",
	}

	for _, file := range requiredFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return fmt.Errorf("required schema file %s not found in current directory. Brainloop must run from its project directory", file)
		}
	}

	// Verify we're in a brainloop project directory
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	if !strings.Contains(pwd, "brainloop") {
		return fmt.Errorf("brainloop must run from its project directory, not from: %s", pwd)
	}

	log.Printf("Running from validated directory: %s", pwd)
	return nil
}
