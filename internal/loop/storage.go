package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"brainloop/internal/database"
)

// Storage provides persistence operations for sessions
type Storage struct {
	lifecycleDB *database.LifecycleDB
	outputDB    *database.OutputDB
}

// NewStorage creates a new storage helper
func NewStorage(lifecycleDB *database.LifecycleDB, outputDB *database.OutputDB) *Storage {
	return &Storage{
		lifecycleDB: lifecycleDB,
		outputDB:    outputDB,
	}
}

// SaveSession persists a session to the database
func (s *Storage) SaveSession(session *Session) error {
	// Session is already saved via CreateSession in manager
	// This is for updating session status
	return s.lifecycleDB.UpdateSessionStatus(session.SessionID, session.Status)
}

// LoadSession retrieves a session from the database
func (s *Storage) LoadSession(sessionID string) (*Session, error) {
	sessionData, err := s.lifecycleDB.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	session := &Session{
		SessionID: sessionID,
		Status:    sessionData["status"].(string),
		CreatedAt: sessionData["created_at"].(int64),
	}

	if completedAt, ok := sessionData["completed_at"].(int64); ok {
		session.CompletedAt = completedAt
	}

	return session, nil
}

// PublishSessionResult publishes a completed session to output database
func (s *Storage) PublishSessionResult(session *Session) error {
	// Calculate session hash
	hash := hashSession(session.SessionID)

	// Count committed blocks
	blocksCommitted := 0
	for _, block := range session.Blocks {
		if block.Status == "committed" {
			blocksCommitted++
		}
	}

	// Serialize session data
	dataJSON, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Publish to output database
	return s.outputDB.PublishResult(hash, session.SessionID, blocksCommitted, string(dataJSON))
}

// GetSessionBlocks retrieves all blocks for a session
func (s *Storage) GetSessionBlocks(sessionID string) ([]Block, error) {
	// This would require a query to get all blocks by session_id
	// For now, return empty slice (blocks are loaded individually)
	return []Block{}, nil
}

// DeleteSession marks a session as abandoned
func (s *Storage) DeleteSession(sessionID string) error {
	return s.lifecycleDB.UpdateSessionStatus(sessionID, "abandoned")
}

// hashSession creates a hash for a session
func hashSession(sessionID string) string {
	hash := sha256.Sum256([]byte(sessionID))
	return hex.EncodeToString(hash[:])
}

// SessionStats represents statistics for sessions
type SessionStats struct {
	TotalSessions      int     `json:"total_sessions"`
	PendingAudit       int     `json:"pending_audit"`
	Committed          int     `json:"committed"`
	Abandoned          int     `json:"abandoned"`
	AvgBlocksPerSession float64 `json:"avg_blocks_per_session"`
}

// GetSessionStats retrieves statistics about sessions
func (s *Storage) GetSessionStats() (*SessionStats, error) {
	// This would require custom queries
	// Placeholder implementation
	return &SessionStats{
		TotalSessions:       0,
		PendingAudit:        0,
		Committed:           0,
		Abandoned:           0,
		AvgBlocksPerSession: 0.0,
	}, nil
}

// CleanupExpiredCache removes expired cache entries
func (s *Storage) CleanupExpiredCache() error {
	// This would delete expired entries from reader_cache
	// Placeholder for now
	return nil
}
