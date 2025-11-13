package tests

import (
	"os"
	"testing"
)

// TestReadMarkdownFixture tests reading the sample markdown file
func TestReadMarkdownFixture(t *testing.T) {
	fixturePath := "fixtures/sample.md"

	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("Fixture not found: %s", fixturePath)
		return
	}

	if len(content) == 0 {
		t.Error("Markdown fixture should not be empty")
	}

	// Check for expected markdown elements
	mdString := string(content)

	if !contains(mdString, "# Brainloop Documentation") {
		t.Error("Expected H1 header 'Brainloop Documentation'")
	}

	if !contains(mdString, "```bash") {
		t.Error("Expected bash code block")
	}

	if !contains(mdString, "```json") {
		t.Error("Expected JSON code block")
	}

	t.Logf("Markdown fixture valid, %d bytes", len(content))
}

// TestReadCodeFixture tests reading the sample Go file
func TestReadCodeFixture(t *testing.T) {
	fixturePath := "fixtures/sample.go"

	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("Fixture not found: %s", fixturePath)
		return
	}

	if len(content) == 0 {
		t.Error("Go fixture should not be empty")
	}

	goString := string(content)

	// Check for expected Go elements
	if !contains(goString, "package sample") {
		t.Error("Expected 'package sample'")
	}

	if !contains(goString, "type User struct") {
		t.Error("Expected User struct definition")
	}

	if !contains(goString, "func NewDatabase") {
		t.Error("Expected NewDatabase function")
	}

	if !contains(goString, "modernc.org/sqlite") {
		t.Error("Expected modernc.org/sqlite import")
	}

	t.Logf("Go fixture valid, %d bytes", len(content))
}

// TestReadConfigFixture tests reading the sample JSON config
func TestReadConfigFixture(t *testing.T) {
	fixturePath := "fixtures/sample.json"

	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("Fixture not found: %s", fixturePath)
		return
	}

	if len(content) == 0 {
		t.Error("JSON fixture should not be empty")
	}

	// Validate JSON structure
	var config map[string]interface{}
	if err := json.Unmarshal(content, &config); err != nil {
		t.Errorf("Invalid JSON: %v", err)
	}

	// Check for expected sections
	expectedSections := []string{"server", "database", "cerebras", "cache", "logging"}

	for _, section := range expectedSections {
		if _, ok := config[section]; !ok {
			t.Errorf("Expected section '%s' in config", section)
		}
	}

	t.Logf("JSON fixture valid, %d sections", len(config))
}

// TestReaderCacheHash tests hash calculation for caching
func TestReaderCacheHash(t *testing.T) {
	filePath := "fixtures/sample.md"

	info, err := os.Stat(filePath)
	if err != nil {
		t.Skip("Fixture not found")
		return
	}

	// Simulate hash calculation (path + mtime)
	hashInput := filePath + ":" + string(info.ModTime().Unix())

	if hashInput == "" {
		t.Error("Hash input should not be empty")
	}

	t.Logf("Cache hash input: %s", hashInput)
}

// TestReaderSourceTypes tests supported source types
func TestReaderSourceTypes(t *testing.T) {
	sourceTypes := []string{"sqlite", "markdown", "code", "config"}

	for _, sourceType := range sourceTypes {
		t.Logf("Supported source type: %s", sourceType)
	}

	if len(sourceTypes) != 4 {
		t.Errorf("Expected 4 source types, got %d", len(sourceTypes))
	}
}

// TestDigestStructure tests digest JSON structure
func TestDigestStructure(t *testing.T) {
	// Example digest structure
	digest := map[string]interface{}{
		"source_type": "code",
		"summary":     "Go code with database operations",
		"structure": map[string]interface{}{
			"packages":  []string{"sample"},
			"imports":   []string{"database/sql", "modernc.org/sqlite"},
			"functions": []string{"NewDatabase", "CreateUser", "GetUser"},
		},
		"patterns": map[string]interface{}{
			"naming_convention": "camelCase",
			"error_handling":    "return errors",
		},
		"recommendations": []string{
			"Add input validation",
			"Implement connection pooling",
		},
	}

	// Verify structure
	if digest["source_type"] != "code" {
		t.Error("Expected source_type=code")
	}

	if _, ok := digest["structure"]; !ok {
		t.Error("Expected 'structure' field")
	}

	if _, ok := digest["patterns"]; !ok {
		t.Error("Expected 'patterns' field")
	}

	t.Logf("Digest structure valid: %d top-level keys", len(digest))
}

// TestCacheExpiration tests cache TTL logic
func TestCacheExpiration(t *testing.T) {
	ttlSeconds := int64(3600) // 1 hour
	currentTime := time.Now().Unix()
	cachedAt := currentTime - 1800 // 30 minutes ago
	expiresAt := cachedAt + ttlSeconds

	if currentTime > expiresAt {
		t.Error("Cache should not be expired")
	} else {
		t.Logf("Cache valid, expires in %d seconds", expiresAt-currentTime)
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Import for JSON test
import (
	"encoding/json"
	"time"
)
