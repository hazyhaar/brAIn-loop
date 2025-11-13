package loop

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"brainloop/internal/cerebras"
	"brainloop/internal/database"

	"github.com/google/uuid"
)

// Manager manages cerebras_loop sessions
type Manager struct {
	lifecycleDB *database.LifecycleDB
	outputDB    *database.OutputDB
	cerebras    *cerebras.Client
	mu          sync.Mutex
}

// NewManager creates a new loop manager
func NewManager(lifecycleDBConn *sql.DB, outputDBConn *sql.DB, cerebrasClient *cerebras.Client) *Manager {
	return &Manager{
		lifecycleDB: database.NewLifecycleDB(lifecycleDBConn),
		outputDB:    database.NewOutputDB(outputDBConn),
		cerebras:    cerebrasClient,
	}
}

// Propose creates a new session and generates initial code for all blocks
func (m *Manager) Propose(req ProposeRequest) (*ProposeResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create session
	sessionID := uuid.New().String()
	if err := m.lifecycleDB.CreateSession(sessionID, "pending_audit"); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Generate code for each block in parallel
	var wg sync.WaitGroup
	blocks := make([]Block, len(req.Blocks))
	errors := make([]error, len(req.Blocks))

	for i, blockInput := range req.Blocks {
		wg.Add(1)
		go func(idx int, input BlockInput) {
			defer wg.Done()

			blockID := input.ID
			if blockID == "" {
				blockID = uuid.New().String()
			}

			// Create block record
			if err := m.lifecycleDB.CreateBlock(blockID, sessionID, input.Description, input.Type, input.Target); err != nil {
				errors[idx] = fmt.Errorf("failed to create block %s: %w", blockID, err)
				return
			}

			// Generate initial code
			code, err := m.generateCode(input.Description, input.Type, 0.6, nil)
			if err != nil {
				errors[idx] = fmt.Errorf("failed to generate code for block %s: %w", blockID, err)
				return
			}

			// Update block with code
			if err := m.lifecycleDB.UpdateBlockCode(blockID, code); err != nil {
				errors[idx] = fmt.Errorf("failed to update block code %s: %w", blockID, err)
				return
			}

			// Retrieve complete block
			blockData, err := m.lifecycleDB.GetBlock(blockID)
			if err != nil {
				errors[idx] = fmt.Errorf("failed to retrieve block %s: %w", blockID, err)
				return
			}

			blocks[idx] = mapToBlock(blockData)
		}(i, blockInput)
	}

	wg.Wait()

	// Check for errors
	for _, err := range errors {
		if err != nil {
			return nil, err
		}
	}

	return &ProposeResponse{
		SessionID: sessionID,
		Blocks:    blocks,
	}, nil
}

// Audit retrieves a block for audit
func (m *Manager) Audit(req AuditRequest) (*AuditResponse, error) {
	blockData, err := m.lifecycleDB.GetBlock(req.BlockID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve block: %w", err)
	}

	// Verify block belongs to session
	if blockData["session_id"].(string) != req.SessionID {
		return nil, fmt.Errorf("block %s does not belong to session %s", req.BlockID, req.SessionID)
	}

	block := mapToBlock(blockData)

	return &AuditResponse{
		Block: block,
	}, nil
}

// Refine regenerates code for a block based on audit feedback
func (m *Manager) Refine(req RefineRequest) (*RefineResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get current block
	blockData, err := m.lifecycleDB.GetBlock(req.BlockID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve block: %w", err)
	}

	block := mapToBlock(blockData)

	// Verify block belongs to session
	if block.SessionID != req.SessionID {
		return nil, fmt.Errorf("block %s does not belong to session %s", req.BlockID, req.SessionID)
	}

	// Build refined prompt with feedback
	refinedPrompt := fmt.Sprintf("Original requirement: %s\n\nCurrent code:\n%s\n\nFeedback: %s\n\nGenerate improved code addressing the feedback.",
		block.Description, block.Code, req.AuditFeedback)

	// Generate refined code with lower temperature
	refinedCode, err := m.generateCode(refinedPrompt, block.Type, 0.3, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refined code: %w", err)
	}

	// Record refinement
	refinementID := uuid.New().String()
	if err := m.lifecycleDB.AddRefinement(refinementID, req.BlockID, req.AuditFeedback, refinedCode, 0.3); err != nil {
		return nil, fmt.Errorf("failed to record refinement: %w", err)
	}

	// Update block code
	if err := m.lifecycleDB.UpdateBlockCode(req.BlockID, refinedCode); err != nil {
		return nil, fmt.Errorf("failed to update block code: %w", err)
	}

	// Get updated block
	blockData, err = m.lifecycleDB.GetBlock(req.BlockID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve updated block: %w", err)
	}

	updatedBlock := mapToBlock(blockData)

	return &RefineResponse{
		Block:       updatedBlock,
		RefinedCode: refinedCode,
		Iterations:  updatedBlock.Iterations,
	}, nil
}

// Commit finalizes a block (executes SQL or writes file)
func (m *Manager) Commit(req CommitRequest) (*CommitResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get block
	blockData, err := m.lifecycleDB.GetBlock(req.BlockID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve block: %w", err)
	}

	block := mapToBlock(blockData)

	// Verify block belongs to session
	if block.SessionID != req.SessionID {
		return nil, fmt.Errorf("block %s does not belong to session %s", req.BlockID, req.SessionID)
	}

	// Final generation with very low temperature (deterministic)
	finalCode, err := m.generateCode(block.Description, block.Type, 0.1, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate final code: %w", err)
	}

	// Execute based on type
	var outputPath string
	switch block.Type {
	case "sql":
		// Execute SQL (assuming target is a database path)
		if err := m.executeSQL(block.Target, finalCode); err != nil {
			return nil, fmt.Errorf("failed to execute SQL: %w", err)
		}
		outputPath = block.Target

	case "go", "python", "code":
		// Write file
		if err := os.WriteFile(block.Target, []byte(finalCode), 0644); err != nil {
			return nil, fmt.Errorf("failed to write file %s: %w", block.Target, err)
		}
		outputPath = block.Target

	default:
		return nil, fmt.Errorf("unsupported block type: %s", block.Type)
	}

	// Calculate hash for idempotence
	hash := calculateHash(req.SessionID, req.BlockID, finalCode)

	// Mark as processed
	resultJSON, _ := json.Marshal(map[string]interface{}{
		"block_id":    req.BlockID,
		"output_path": outputPath,
		"type":        block.Type,
	})
	if err := m.lifecycleDB.MarkProcessed(hash, "commit", string(resultJSON)); err != nil {
		return nil, fmt.Errorf("failed to mark processed: %w", err)
	}

	// Update block status
	if err := m.lifecycleDB.CommitBlock(req.BlockID); err != nil {
		return nil, fmt.Errorf("failed to commit block: %w", err)
	}

	// Update block code one last time
	if err := m.lifecycleDB.UpdateBlockCode(req.BlockID, finalCode); err != nil {
		return nil, fmt.Errorf("failed to update final code: %w", err)
	}

	// Get final block
	blockData, err = m.lifecycleDB.GetBlock(req.BlockID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve committed block: %w", err)
	}

	committedBlock := mapToBlock(blockData)

	return &CommitResponse{
		Block:      committedBlock,
		Success:    true,
		Message:    fmt.Sprintf("Block committed successfully to %s", outputPath),
		OutputPath: outputPath,
	}, nil
}

// generateCode generates code using Cerebras
func (m *Manager) generateCode(prompt, codeType string, temperature float64, patterns interface{}) (string, error) {
	result, err := m.cerebras.GenerateCodeWithTemperature(prompt, codeType, patterns, temperature)
	if err != nil {
		return "", err
	}

	// Record usage
	requestID := uuid.New().String()
	m.lifecycleDB.RecordCerebrasUsage(
		requestID,
		"generate_code",
		result.Model,
		result.Temperature,
		result.PromptTokens,
		result.CompletionTokens,
		result.LatencyMs,
	)

	// Record metric
	m.outputDB.RecordMetric("cerebras_tokens_prompt", float64(result.PromptTokens))
	m.outputDB.RecordMetric("cerebras_tokens_completion", float64(result.CompletionTokens))
	m.outputDB.RecordMetric("cerebras_latency_ms", float64(result.LatencyMs))

	return result.Content, nil
}

// executeSQL executes SQL in a transaction
func (m *Manager) executeSQL(dbPath, sqlCode string) error {
	// Open target database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Execute in transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if _, err := tx.Exec(sqlCode); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// calculateHash calculates SHA256 hash for idempotence
func calculateHash(sessionID, blockID, code string) string {
	data := sessionID + blockID + code
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// mapToBlock converts map to Block struct
func mapToBlock(data map[string]interface{}) Block {
	block := Block{
		BlockID:     data["block_id"].(string),
		SessionID:   data["session_id"].(string),
		Description: data["description"].(string),
		Type:        data["type"].(string),
		Target:      data["target"].(string),
		Status:      data["status"].(string),
		Iterations:  data["iterations"].(int),
		GeneratedAt: data["generated_at"].(int64),
	}

	if code, ok := data["code"].(string); ok {
		block.Code = code
	}
	if lastRefined, ok := data["last_refined_at"].(int64); ok {
		block.LastRefinedAt = lastRefined
	}
	if committed, ok := data["committed_at"].(int64); ok {
		block.CommittedAt = committed
	}

	return block
}
