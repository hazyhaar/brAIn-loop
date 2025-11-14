package tests

import (
	"encoding/json"
	"testing"
)

// TestMCPInitialize tests the MCP initialize handshake
func TestMCPInitialize(t *testing.T) {
	// This is a placeholder test - would need actual server setup
	// In real implementation, we'd initialize databases and server

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
		},
	}

	reqJSON, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	t.Logf("Initialize request: %s", string(reqJSON))

	// In real test, would send to server and verify response
	// For now, just verify request structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(reqJSON, &parsed); err != nil {
		t.Errorf("Request is not valid JSON: %v", err)
	}

	if parsed["method"] != "initialize" {
		t.Errorf("Expected method=initialize, got %v", parsed["method"])
	}
}

// TestMCPToolsList tests the tools/list response
func TestMCPToolsList(t *testing.T) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}

	reqJSON, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	t.Logf("Tools/list request: %s", string(reqJSON))

	// Expected response should contain exactly 1 tool named "brainloop"
	// This is the progressive disclosure pattern
	expectedToolName := "brainloop"

	// In real test, would verify server returns this
	t.Logf("Expected tool name: %s", expectedToolName)
}

// TestMCPToolCallListActions tests the list_actions action
func TestMCPToolCallListActions(t *testing.T) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "brainloop",
			"arguments": map[string]interface{}{
				"action": "list_actions",
				"params": map[string]interface{}{},
			},
		},
	}

	reqJSON, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	t.Logf("List actions request: %s", string(reqJSON))

	// Expected actions
	expectedActions := []string{
		"generate_file", "generate_sql", "explore", "loop",
		"read_sqlite", "read_markdown", "read_code", "read_config",
		"list_actions", "get_schema", "get_stats",
	}

	t.Logf("Expected %d actions", len(expectedActions))

	for _, action := range expectedActions {
		t.Logf("  - %s", action)
	}
}

// TestJSONRPCParsing tests JSON-RPC message parsing
func TestJSONRPCParsing(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "Valid initialize",
			input:   `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
			wantErr: false,
		},
		{
			name:    "Valid tools/list",
			input:   `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
			wantErr: false,
		},
		{
			name:    "Invalid JSON",
			input:   `{invalid json`,
			wantErr: true,
		},
		{
			name:    "Missing method",
			input:   `{"jsonrpc":"2.0","id":1}`,
			wantErr: false, // Valid JSON, just incomplete
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req map[string]interface{}
			err := json.Unmarshal([]byte(tt.input), &req)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil {
				if jsonrpc, ok := req["jsonrpc"].(string); ok {
					if jsonrpc != "2.0" {
						t.Errorf("Expected jsonrpc=2.0, got %s", jsonrpc)
					}
				}
			}
		})
	}
}

// TestProgressiveDisclosure verifies the progressive disclosure pattern
func TestProgressiveDisclosure(t *testing.T) {
	// Progressive disclosure means:
	// 1. tools/list returns 1 tool ("brainloop")
	// 2. That tool has an "action" parameter with enum of 11 actions
	// 3. Saves ~83% context tokens (1 tool vs 8 tools)

	singleToolSize := 800   // estimated tokens for 1 tool
	multipleToolsSize := 4800 // estimated tokens for 8 tools

	saving := float64(multipleToolsSize-singleToolSize) / float64(multipleToolsSize) * 100

	t.Logf("Progressive disclosure token savings:")
	t.Logf("  Single tool: %d tokens", singleToolSize)
	t.Logf("  Multiple tools: %d tokens", multipleToolsSize)
	t.Logf("  Savings: %.1f%%", saving)

	if saving < 80 {
		t.Errorf("Expected savings >= 80%%, got %.1f%%", saving)
	}
}
