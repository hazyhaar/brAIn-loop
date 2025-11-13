package mcp

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"brainloop/internal/database"
	"brainloop/internal/loop"
)

// dispatchAction routes actions to appropriate handlers
func (s *Server) dispatchAction(action string, params map[string]interface{}) (interface{}, error) {
	switch action {
	case "generate_file":
		return s.handleGenerateFile(params)
	case "generate_sql":
		return s.handleGenerateSQL(params)
	case "explore":
		return s.handleExplore(params)
	case "loop":
		return s.handleLoop(params)
	case "read_sqlite":
		return s.handleReadSQLite(params)
	case "read_markdown":
		return s.handleReadMarkdown(params)
	case "read_code":
		return s.handleReadCode(params)
	case "read_config":
		return s.handleReadConfig(params)
	case "list_actions":
		return s.handleListActions(params)
	case "get_schema":
		return s.handleGetSchema(params)
	case "get_stats":
		return s.handleGetStats(params)
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

// handleGenerateFile generates a code file
func (s *Server) handleGenerateFile(params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	verifiedPrompt, ok := params["verified_prompt"].(string)
	if !ok {
		return nil, fmt.Errorf("missing verified_prompt")
	}

	outputPath, ok := params["output_path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing output_path")
	}

	codeType, ok := params["code_type"].(string)
	if !ok {
		codeType = "code"
	}

	// Extract patterns if provided
	var patterns interface{}
	if p, ok := params["patterns"]; ok {
		patterns = p
	}

	// Generate code
	code, err := s.cerebrasClient.GenerateCode(verifiedPrompt, codeType, patterns)
	if err != nil {
		return nil, fmt.Errorf("failed to generate code: %w", err)
	}

	// Write file
	if err := os.WriteFile(outputPath, []byte(code), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Calculate hash and mark processed
	hash := hashString(verifiedPrompt + outputPath + code)
	lifecycleDB := s.lifecycleDB
	db := database.NewLifecycleDB(lifecycleDB)

	resultJSON, _ := json.Marshal(map[string]interface{}{
		"output_path": outputPath,
		"code_type":   codeType,
		"line_count":  len(strings.Split(code, "\n")),
	})
	db.MarkProcessed(hash, "generate_file", string(resultJSON))

	return map[string]interface{}{
		"success":     true,
		"output_path": outputPath,
		"code_type":   codeType,
		"line_count":  len(strings.Split(code, "\n")),
		"message":     fmt.Sprintf("File generated successfully: %s", outputPath),
	}, nil
}

// handleGenerateSQL generates and executes SQL
func (s *Server) handleGenerateSQL(params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	verifiedPrompt, ok := params["verified_prompt"].(string)
	if !ok {
		return nil, fmt.Errorf("missing verified_prompt")
	}

	dbPath, ok := params["db_path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing db_path")
	}

	// Generate SQL
	sqlCode, err := s.cerebrasClient.GenerateCode(verifiedPrompt, "sql", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate SQL: %w", err)
	}

	// Execute SQL in transaction
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	if _, err := tx.Exec(sqlCode); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to execute SQL: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Mark processed
	hash := hashString(verifiedPrompt + dbPath + sqlCode)
	lifecycleDB := database.NewLifecycleDB(s.lifecycleDB)

	resultJSON, _ := json.Marshal(map[string]interface{}{
		"db_path": dbPath,
		"success": true,
	})
	lifecycleDB.MarkProcessed(hash, "generate_sql", string(resultJSON))

	return map[string]interface{}{
		"success": true,
		"db_path": dbPath,
		"message": "SQL executed successfully",
	}, nil
}

// handleExplore generates exploratory code without execution
func (s *Server) handleExplore(params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	description, ok := params["description"].(string)
	if !ok {
		return nil, fmt.Errorf("missing description")
	}

	codeType, ok := params["type"].(string)
	if !ok {
		codeType = "code"
	}

	// Generate with creative temperature
	result, err := s.cerebrasClient.GenerateCodeWithTemperature(description, codeType, nil, 0.6)
	if err != nil {
		return nil, fmt.Errorf("failed to generate code: %w", err)
	}

	return map[string]interface{}{
		"success": true,
		"code":    result.Content,
		"tokens":  result.PromptTokens + result.CompletionTokens,
		"message": "Exploratory code generated (not executed)",
	}, nil
}

// handleLoop handles loop workflow actions
func (s *Server) handleLoop(params map[string]interface{}) (interface{}, error) {
	// Extract mode
	mode, ok := params["mode"].(string)
	if !ok {
		return nil, fmt.Errorf("missing mode parameter")
	}

	switch mode {
	case "propose":
		return s.handleLoopPropose(params)
	case "audit":
		return s.handleLoopAudit(params)
	case "refine":
		return s.handleLoopRefine(params)
	case "commit":
		return s.handleLoopCommit(params)
	default:
		return nil, fmt.Errorf("unknown loop mode: %s", mode)
	}
}

// handleLoopPropose handles loop propose action
func (s *Server) handleLoopPropose(params map[string]interface{}) (interface{}, error) {
	// Extract blocks
	blocksRaw, ok := params["blocks"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing blocks parameter")
	}

	// Convert to BlockInput
	var blocks []loop.BlockInput
	for _, b := range blocksRaw {
		blockMap, ok := b.(map[string]interface{})
		if !ok {
			continue
		}

		block := loop.BlockInput{
			ID:          getString(blockMap, "id"),
			Description: getString(blockMap, "description"),
			Type:        getString(blockMap, "type"),
			Target:      getString(blockMap, "target"),
		}
		blocks = append(blocks, block)
	}

	// Call loop manager
	response, err := s.loopManager.Propose(loop.ProposeRequest{Blocks: blocks})
	if err != nil {
		return nil, err
	}

	return response, nil
}

// handleLoopAudit handles loop audit action
func (s *Server) handleLoopAudit(params map[string]interface{}) (interface{}, error) {
	sessionID, ok := params["session_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing session_id")
	}

	blockID, ok := params["block_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing block_id")
	}

	response, err := s.loopManager.Audit(loop.AuditRequest{
		SessionID: sessionID,
		BlockID:   blockID,
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

// handleLoopRefine handles loop refine action
func (s *Server) handleLoopRefine(params map[string]interface{}) (interface{}, error) {
	sessionID, ok := params["session_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing session_id")
	}

	blockID, ok := params["block_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing block_id")
	}

	auditFeedback, ok := params["audit_feedback"].(string)
	if !ok {
		return nil, fmt.Errorf("missing audit_feedback")
	}

	response, err := s.loopManager.Refine(loop.RefineRequest{
		SessionID:     sessionID,
		BlockID:       blockID,
		AuditFeedback: auditFeedback,
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

// handleLoopCommit handles loop commit action
func (s *Server) handleLoopCommit(params map[string]interface{}) (interface{}, error) {
	sessionID, ok := params["session_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing session_id")
	}

	blockID, ok := params["block_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing block_id")
	}

	response, err := s.loopManager.Commit(loop.CommitRequest{
		SessionID: sessionID,
		BlockID:   blockID,
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

// handleReadSQLite handles SQLite database reading
func (s *Server) handleReadSQLite(params map[string]interface{}) (interface{}, error) {
	digest, err := s.readersHub.ReadSQLite(params)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"digest":  digest,
		"format":  "json",
	}, nil
}

// handleReadMarkdown handles markdown file reading
func (s *Server) handleReadMarkdown(params map[string]interface{}) (interface{}, error) {
	digest, err := s.readersHub.ReadMarkdown(params)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"digest":  digest,
		"format":  "json",
	}, nil
}

// handleReadCode handles code file reading
func (s *Server) handleReadCode(params map[string]interface{}) (interface{}, error) {
	digest, err := s.readersHub.ReadCode(params)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"digest":  digest,
		"format":  "json",
	}, nil
}

// handleReadConfig handles config file reading
func (s *Server) handleReadConfig(params map[string]interface{}) (interface{}, error) {
	digest, err := s.readersHub.ReadConfig(params)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"digest":  digest,
		"format":  "json",
	}, nil
}

// handleListActions lists all available actions with descriptions
func (s *Server) handleListActions(params map[string]interface{}) (interface{}, error) {
	actions := []map[string]interface{}{
		{
			"name":        "generate_file",
			"description": "Generate a code file from prompt with pattern injection",
			"parameters":  []string{"verified_prompt", "output_path", "code_type", "patterns (optional)"},
		},
		{
			"name":        "generate_sql",
			"description": "Generate and execute SQL in a database",
			"parameters":  []string{"verified_prompt", "db_path"},
		},
		{
			"name":        "explore",
			"description": "Generate exploratory code without execution (creative mode)",
			"parameters":  []string{"description", "type"},
		},
		{
			"name":        "loop",
			"description": "Iterative code generation workflow (propose/audit/refine/commit)",
			"parameters":  []string{"mode", "session_id (audit/refine/commit)", "block_id (audit/refine/commit)", "blocks (propose)", "audit_feedback (refine)"},
		},
		{
			"name":        "read_sqlite",
			"description": "Read and analyze SQLite database with intelligent digest",
			"parameters":  []string{"db_path", "max_sample_rows (optional)"},
		},
		{
			"name":        "read_markdown",
			"description": "Read and analyze markdown file",
			"parameters":  []string{"file_path"},
		},
		{
			"name":        "read_code",
			"description": "Read and analyze source code file",
			"parameters":  []string{"file_path"},
		},
		{
			"name":        "read_config",
			"description": "Read and analyze configuration file (JSON/YAML/TOML)",
			"parameters":  []string{"file_path"},
		},
		{
			"name":        "list_actions",
			"description": "List all available actions (this action)",
			"parameters":  []string{},
		},
		{
			"name":        "get_schema",
			"description": "Get detailed schema for a specific action",
			"parameters":  []string{"action_name"},
		},
		{
			"name":        "get_stats",
			"description": "Get usage statistics (Cerebras tokens, cache hit rate, etc.)",
			"parameters":  []string{},
		},
	}

	return map[string]interface{}{
		"actions": actions,
		"count":   len(actions),
	}, nil
}

// handleGetSchema returns detailed schema for an action
func (s *Server) handleGetSchema(params map[string]interface{}) (interface{}, error) {
	actionName, ok := params["action_name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing action_name parameter")
	}

	schemas := map[string]interface{}{
		"generate_file": map[string]interface{}{
			"verified_prompt": map[string]string{
				"type":        "string",
				"required":    "true",
				"description": "The prompt describing what code to generate",
			},
			"output_path": map[string]string{
				"type":        "string",
				"required":    "true",
				"description": "File path where generated code will be written",
			},
			"code_type": map[string]string{
				"type":        "string",
				"required":    "false",
				"description": "Type of code: go, python, sql, code (default)",
			},
			"patterns": map[string]string{
				"type":        "object",
				"required":    "false",
				"description": "Project patterns for context injection",
			},
		},
		// Add other schemas as needed
	}

	schema, ok := schemas[actionName]
	if !ok {
		return map[string]interface{}{
			"error": fmt.Sprintf("No schema found for action: %s", actionName),
		}, nil
	}

	return map[string]interface{}{
		"action": actionName,
		"schema": schema,
	}, nil
}

// handleGetStats returns usage statistics
func (s *Server) handleGetStats(params map[string]interface{}) (interface{}, error) {
	outputDB := database.NewOutputDB(s.outputDB)

	// Get aggregated metrics for last hour
	since := time.Now().Add(-1 * time.Hour).Unix()
	metrics, err := outputDB.GetAggregatedMetrics(since)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	return map[string]interface{}{
		"period_hours": 1,
		"metrics":      metrics,
		"timestamp":    time.Now().Unix(),
	}, nil
}

// Helper functions

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}
