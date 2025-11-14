package bash

import (
	"os"
	"testing"
	"time"
)

func TestNewRegistry(t *testing.T) {
	tempDB := "test_registry.db"
	defer os.Remove(tempDB)

	registry, err := NewRegistry(tempDB)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.db.Close()

	// Verify table was created
	var count int
	err = registry.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='commands_registry'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query table: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 table, got %d", count)
	}
}

func TestGetOrCreateCommand(t *testing.T) {
	tempDB := "test_get_or_create.db"
	defer os.Remove(tempDB)

	registry, err := NewRegistry(tempDB)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.db.Close()

	cmd := "ls -la /workspace"

	// First call should create
	hash1, err := registry.GetOrCreateCommand(cmd)
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}
	if hash1 == "" {
		t.Error("Expected non-empty hash")
	}

	// Second call should return same hash
	hash2, err := registry.GetOrCreateCommand(cmd)
	if err != nil {
		t.Fatalf("Failed to get command: %v", err)
	}
	if hash1 != hash2 {
		t.Errorf("Expected same hash, got %s vs %s", hash1, hash2)
	}

	// Verify only one record exists
	var count int
	registry.db.QueryRow("SELECT COUNT(*) FROM commands_registry").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 record, got %d", count)
	}
}

func TestUpdateExecution(t *testing.T) {
	tempDB := "test_update_exec.db"
	defer os.Remove(tempDB)

	registry, err := NewRegistry(tempDB)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.db.Close()

	cmd := "echo hello"
	hash, _ := registry.GetOrCreateCommand(cmd)

	// Record successful execution
	err = registry.UpdateExecution(hash, 0, 100)
	if err != nil {
		t.Fatalf("Failed to update execution: %v", err)
	}

	// Verify counts
	var execCount, successCount, failureCount, avgDuration int
	err = registry.db.QueryRow(`
		SELECT execution_count, success_count, failure_count, avg_duration_ms
		FROM commands_registry WHERE command_hash = ?
	`, hash).Scan(&execCount, &successCount, &failureCount, &avgDuration)

	if err != nil {
		t.Fatalf("Failed to query stats: %v", err)
	}

	if execCount != 1 {
		t.Errorf("Expected execution_count=1, got %d", execCount)
	}
	if successCount != 1 {
		t.Errorf("Expected success_count=1, got %d", successCount)
	}
	if failureCount != 0 {
		t.Errorf("Expected failure_count=0, got %d", failureCount)
	}
	if avgDuration != 100 {
		t.Errorf("Expected avg_duration=100, got %d", avgDuration)
	}

	// Record failed execution
	err = registry.UpdateExecution(hash, 1, 200)
	if err != nil {
		t.Fatalf("Failed to update execution: %v", err)
	}

	registry.db.QueryRow(`
		SELECT execution_count, success_count, failure_count, avg_duration_ms
		FROM commands_registry WHERE command_hash = ?
	`, hash).Scan(&execCount, &successCount, &failureCount, &avgDuration)

	if execCount != 2 {
		t.Errorf("Expected execution_count=2, got %d", execCount)
	}
	if successCount != 1 {
		t.Errorf("Expected success_count=1, got %d", successCount)
	}
	if failureCount != 1 {
		t.Errorf("Expected failure_count=1, got %d", failureCount)
	}
	// Avg should be (100 + 200) / 2 = 150
	if avgDuration != 150 {
		t.Errorf("Expected avg_duration=150, got %d", avgDuration)
	}
}

func TestGetPolicy(t *testing.T) {
	tempDB := "test_get_policy.db"
	defer os.Remove(tempDB)

	registry, err := NewRegistry(tempDB)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.db.Close()

	cmd := "cat file.txt"
	hash, _ := registry.GetOrCreateCommand(cmd)

	// Default policy should be "unknown"
	policy, err := registry.GetPolicy(hash)
	if err != nil {
		t.Fatalf("Failed to get policy: %v", err)
	}
	if policy != "unknown" {
		t.Errorf("Expected default policy 'unknown', got %s", policy)
	}

	// Set current_policy
	registry.db.Exec("UPDATE commands_registry SET current_policy = 'ask' WHERE command_hash = ?", hash)

	policy, _ = registry.GetPolicy(hash)
	if policy != "ask" {
		t.Errorf("Expected policy 'ask', got %s", policy)
	}

	// User override should take priority
	registry.db.Exec("UPDATE commands_registry SET user_override = 'auto_approve' WHERE command_hash = ?", hash)

	policy, _ = registry.GetPolicy(hash)
	if policy != "auto_approve" {
		t.Errorf("Expected policy 'auto_approve' (user override), got %s", policy)
	}
}

func TestSetPolicy(t *testing.T) {
	tempDB := "test_set_policy.db"
	defer os.Remove(tempDB)

	registry, err := NewRegistry(tempDB)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.db.Close()

	cmd := "grep pattern file.txt"
	hash, _ := registry.GetOrCreateCommand(cmd)

	// Set policy
	err = registry.SetPolicy(hash, "auto_approve", "trusted command", false)
	if err != nil {
		t.Fatalf("Failed to set policy: %v", err)
	}

	policy, _ := registry.GetPolicy(hash)
	if policy != "auto_approve" {
		t.Errorf("Expected policy 'auto_approve', got %s", policy)
	}

	// Set user override
	err = registry.SetPolicy(hash, "ask_warning", "needs review", true)
	if err != nil {
		t.Fatalf("Failed to set user override: %v", err)
	}

	policy, _ = registry.GetPolicy(hash)
	if policy != "ask_warning" {
		t.Errorf("Expected policy 'ask_warning' (override), got %s", policy)
	}
}

func TestPromoteToAutoApprove(t *testing.T) {
	tempDB := "test_promote.db"
	defer os.Remove(tempDB)

	registry, err := NewRegistry(tempDB)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.db.Close()

	cmd := "ls -la"
	hash, _ := registry.GetOrCreateCommand(cmd)

	// Set initial policy
	registry.SetPolicy(hash, "ask", "testing", false)

	// Simulate 20 successful executions (95% success rate)
	for i := 0; i < 19; i++ {
		registry.UpdateExecution(hash, 0, 100)
	}
	registry.UpdateExecution(hash, 1, 100) // 1 failure

	// Promote
	err = registry.PromoteToAutoApprove(hash)
	if err != nil {
		t.Fatalf("Failed to promote: %v", err)
	}

	// Verify policy changed
	policy, _ := registry.GetPolicy(hash)
	if policy != "auto_approve" {
		t.Errorf("Expected policy 'auto_approve' after promotion, got %s", policy)
	}

	// Verify promoted_at timestamp
	var promotedAt int64
	registry.db.QueryRow("SELECT promoted_at FROM commands_registry WHERE command_hash = ?", hash).Scan(&promotedAt)
	if promotedAt == 0 {
		t.Error("Expected promoted_at to be set")
	}
}

func TestCheckAutoEvolution(t *testing.T) {
	tempDB := "test_auto_evolution.db"
	defer os.Remove(tempDB)

	registry, err := NewRegistry(tempDB)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.db.Close()

	cmd := "echo test"
	hash, _ := registry.GetOrCreateCommand(cmd)

	// Set policy to ask
	registry.SetPolicy(hash, "ask", "testing auto-evolution", false)

	// Simulate 20 successful executions
	for i := 0; i < 20; i++ {
		registry.UpdateExecution(hash, 0, 50)
	}

	// Check auto-evolution (should promote)
	promoted, err := registry.CheckAutoEvolution(hash)
	if err != nil {
		t.Fatalf("Failed to check auto-evolution: %v", err)
	}

	if !promoted {
		t.Error("Expected command to be promoted")
	}

	policy, _ := registry.GetPolicy(hash)
	if policy != "auto_approve" {
		t.Errorf("Expected policy 'auto_approve', got %s", policy)
	}
}

func TestTimestamps100(t *testing.T) {
	tempDB := "test_timestamps.db"
	defer os.Remove(tempDB)

	registry, err := NewRegistry(tempDB)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.db.Close()

	cmd := "pwd"
	hash, _ := registry.GetOrCreateCommand(cmd)

	// Execute 150 times (should keep only last 100)
	for i := 0; i < 150; i++ {
		registry.UpdateExecution(hash, 0, 10)
		time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	}

	// Verify last_100_timestamps
	var timestamps string
	registry.db.QueryRow("SELECT last_100_timestamps FROM commands_registry WHERE command_hash = ?", hash).Scan(&timestamps)

	// Count semicolons (should be 99 for 100 timestamps)
	count := 0
	for _, c := range timestamps {
		if c == ';' {
			count++
		}
	}

	if count != 99 {
		t.Errorf("Expected 99 semicolons (100 timestamps), got %d", count)
	}
}

func TestCalculateHash(t *testing.T) {
	tests := []struct {
		cmd1     string
		cmd2     string
		samehash bool
	}{
		{"ls -la", "ls -la", true},
		{"ls -la", "ls -l", false},
		{"echo hello", "echo  hello", false}, // Different whitespace
		{"", "", true},
	}

	for _, tt := range tests {
		hash1 := calculateHash(tt.cmd1)
		hash2 := calculateHash(tt.cmd2)

		if tt.samehash && hash1 != hash2 {
			t.Errorf("Expected same hash for '%s' and '%s'", tt.cmd1, tt.cmd2)
		}
		if !tt.samehash && hash1 == hash2 {
			t.Errorf("Expected different hash for '%s' and '%s'", tt.cmd1, tt.cmd2)
		}

		// Verify hash is 64 chars (SHA256)
		if len(hash1) != 64 {
			t.Errorf("Expected hash length 64, got %d", len(hash1))
		}
	}
}

func TestGetCommandStats(t *testing.T) {
	tempDB := "test_stats.db"
	defer os.Remove(tempDB)

	registry, err := NewRegistry(tempDB)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	defer registry.db.Close()

	cmd := "make test"
	hash, _ := registry.GetOrCreateCommand(cmd)

	// Execute multiple times
	registry.UpdateExecution(hash, 0, 100)
	registry.UpdateExecution(hash, 0, 200)
	registry.UpdateExecution(hash, 1, 150)

	stats, err := registry.GetCommandStats(hash)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.ExecutionCount != 3 {
		t.Errorf("Expected execution_count=3, got %d", stats.ExecutionCount)
	}
	if stats.SuccessCount != 2 {
		t.Errorf("Expected success_count=2, got %d", stats.SuccessCount)
	}
	if stats.FailureCount != 1 {
		t.Errorf("Expected failure_count=1, got %d", stats.FailureCount)
	}
	if stats.AvgDurationMs != 150 {
		t.Errorf("Expected avg_duration=150, got %d", stats.AvgDurationMs)
	}
}
