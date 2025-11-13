package database

import (
	"database/sql"
	"fmt"
	"time"
)

// LifecycleDB provides helper methods for lifecycle database operations
type LifecycleDB struct {
	db *sql.DB
}

// NewLifecycleDB creates a new lifecycle database helper
func NewLifecycleDB(db *sql.DB) *LifecycleDB {
	return &LifecycleDB{db: db}
}

// CreateSession creates a new session
func (l *LifecycleDB) CreateSession(sessionID, status string) error {
	_, err := l.db.Exec(`
		INSERT INTO sessions (session_id, status, created_at)
		VALUES (?, ?, ?)
	`, sessionID, status, time.Now().Unix())
	return err
}

// GetSession retrieves a session by ID
func (l *LifecycleDB) GetSession(sessionID string) (map[string]interface{}, error) {
	var status string
	var createdAt, completedAt sql.NullInt64

	err := l.db.QueryRow(`
		SELECT status, created_at, completed_at
		FROM sessions
		WHERE session_id = ?
	`, sessionID).Scan(&status, &createdAt, &completedAt)

	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"session_id": sessionID,
		"status":     status,
		"created_at": createdAt.Int64,
	}

	if completedAt.Valid {
		result["completed_at"] = completedAt.Int64
	}

	return result, nil
}

// UpdateSessionStatus updates session status
func (l *LifecycleDB) UpdateSessionStatus(sessionID, status string) error {
	_, err := l.db.Exec(`
		UPDATE sessions
		SET status = ?, completed_at = ?
		WHERE session_id = ?
	`, status, time.Now().Unix(), sessionID)
	return err
}

// CreateBlock creates a new block in a session
func (l *LifecycleDB) CreateBlock(blockID, sessionID, description, blockType, target string) error {
	_, err := l.db.Exec(`
		INSERT INTO session_blocks
		(block_id, session_id, description, type, target, generated_at, status)
		VALUES (?, ?, ?, ?, ?, ?, 'pending')
	`, blockID, sessionID, description, blockType, target, time.Now().Unix())
	return err
}

// GetBlock retrieves a block by ID
func (l *LifecycleDB) GetBlock(blockID string) (map[string]interface{}, error) {
	var sessionID, description, blockType, target, status string
	var code sql.NullString
	var iterations int
	var generatedAt int64
	var lastRefinedAt, committedAt sql.NullInt64

	err := l.db.QueryRow(`
		SELECT session_id, description, type, target, code, iterations, status,
		       generated_at, last_refined_at, committed_at
		FROM session_blocks
		WHERE block_id = ?
	`, blockID).Scan(&sessionID, &description, &blockType, &target, &code, &iterations,
		&status, &generatedAt, &lastRefinedAt, &committedAt)

	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"block_id":     blockID,
		"session_id":   sessionID,
		"description":  description,
		"type":         blockType,
		"target":       target,
		"iterations":   iterations,
		"status":       status,
		"generated_at": generatedAt,
	}

	if code.Valid {
		result["code"] = code.String
	}
	if lastRefinedAt.Valid {
		result["last_refined_at"] = lastRefinedAt.Int64
	}
	if committedAt.Valid {
		result["committed_at"] = committedAt.Int64
	}

	return result, nil
}

// UpdateBlockCode updates the code for a block
func (l *LifecycleDB) UpdateBlockCode(blockID, code string) error {
	_, err := l.db.Exec(`
		UPDATE session_blocks
		SET code = ?, iterations = iterations + 1, last_refined_at = ?
		WHERE block_id = ?
	`, code, time.Now().Unix(), blockID)
	return err
}

// CommitBlock marks a block as committed
func (l *LifecycleDB) CommitBlock(blockID string) error {
	_, err := l.db.Exec(`
		UPDATE session_blocks
		SET status = 'committed', committed_at = ?
		WHERE block_id = ?
	`, time.Now().Unix(), blockID)
	return err
}

// AddRefinement records a refinement for a block
func (l *LifecycleDB) AddRefinement(refinementID, blockID, feedback, refinedCode string, temperature float64) error {
	_, err := l.db.Exec(`
		INSERT INTO block_refinements
		(refinement_id, block_id, feedback, temperature, refined_code, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, refinementID, blockID, feedback, temperature, refinedCode, time.Now().Unix())
	return err
}

// GetCachedDigest retrieves a cached digest
func (l *LifecycleDB) GetCachedDigest(hash string) (string, error) {
	var digestJSON string
	var expiresAt int64

	err := l.db.QueryRow(`
		SELECT digest_json, expires_at
		FROM reader_cache
		WHERE hash = ?
	`, hash).Scan(&digestJSON, &expiresAt)

	if err != nil {
		return "", err
	}

	// Check if expired
	if time.Now().Unix() > expiresAt {
		return "", fmt.Errorf("cache expired")
	}

	return digestJSON, nil
}

// SetCachedDigest stores a digest in cache
func (l *LifecycleDB) SetCachedDigest(hash, sourceType, sourcePath, digestJSON string, ttlSeconds int64) error {
	now := time.Now().Unix()
	expiresAt := now + ttlSeconds

	_, err := l.db.Exec(`
		INSERT OR REPLACE INTO reader_cache
		(hash, source_type, source_path, digest_json, cached_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, hash, sourceType, sourcePath, digestJSON, now, expiresAt)
	return err
}

// IsProcessed checks if an operation was already processed
func (l *LifecycleDB) IsProcessed(hash string) (bool, error) {
	var count int
	err := l.db.QueryRow(`
		SELECT COUNT(*) FROM processed_log WHERE hash = ?
	`, hash).Scan(&count)
	return count > 0, err
}

// MarkProcessed marks an operation as processed
func (l *LifecycleDB) MarkProcessed(hash, operation string, resultJSON string) error {
	_, err := l.db.Exec(`
		INSERT INTO processed_log (hash, operation, timestamp, result_json)
		VALUES (?, ?, ?, ?)
	`, hash, operation, time.Now().Unix(), resultJSON)
	return err
}

// RecordCerebrasUsage records API usage metrics
func (l *LifecycleDB) RecordCerebrasUsage(requestID, operation, model string, temperature float64, tokensPrompt, tokensCompletion, latencyMs int) error {
	_, err := l.db.Exec(`
		INSERT INTO cerebras_usage
		(request_id, operation, model, temperature, tokens_prompt, tokens_completion, latency_ms, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, requestID, operation, model, temperature, tokensPrompt, tokensCompletion, latencyMs, time.Now().Unix())
	return err
}
