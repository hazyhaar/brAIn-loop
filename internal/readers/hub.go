package readers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"

	"brainloop/internal/cerebras"
	"brainloop/internal/database"
)

// Hub coordinates all readers
type Hub struct {
	lifecycleDB *database.LifecycleDB
	outputDB    *database.OutputDB
	cerebras    *cerebras.Client
}

// NewHub creates a new reader hub
func NewHub(lifecycleDBConn *sql.DB, outputDBConn *sql.DB, cerebrasClient *cerebras.Client) *Hub {
	return &Hub{
		lifecycleDB: database.NewLifecycleDB(lifecycleDBConn),
		outputDB:    database.NewOutputDB(outputDBConn),
		cerebras:    cerebrasClient,
	}
}

// Read dispatches to the appropriate reader based on source type
func (h *Hub) Read(sourceType string, params map[string]interface{}) (string, error) {
	switch sourceType {
	case "sqlite":
		return h.ReadSQLite(params)
	case "markdown":
		return h.ReadMarkdown(params)
	case "code":
		return h.ReadCode(params)
	case "config":
		return h.ReadConfig(params)
	default:
		return "", fmt.Errorf("unsupported source type: %s", sourceType)
	}
}

// computeHash computes SHA256 hash of file path + mtime
func (h *Hub) computeHash(filePath string) (string, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	data := fmt.Sprintf("%s:%d", filePath, fileInfo.ModTime().Unix())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:]), nil
}

// checkCache checks if a digest is cached
func (h *Hub) checkCache(hash string) (string, bool) {
	digest, err := h.lifecycleDB.GetCachedDigest(hash)
	if err != nil {
		return "", false
	}
	return digest, true
}

// saveCache saves a digest to cache
func (h *Hub) saveCache(hash, sourceType, sourcePath, digest string) error {
	// Cache for 1 hour (3600 seconds)
	return h.lifecycleDB.SetCachedDigest(hash, sourceType, sourcePath, digest, 3600)
}

// publishDigest publishes a digest to output database
func (h *Hub) publishDigest(hash, sourceType, sourcePath, digest string) error {
	return h.outputDB.PublishDigest(hash, sourceType, sourcePath, digest)
}

// generateDigest generates a digest using Cerebras
func (h *Hub) generateDigest(sourceType, sourceData string) (string, error) {
	digest, err := h.cerebras.GenerateDigest(sourceType, sourceData)
	if err != nil {
		return "", fmt.Errorf("failed to generate digest: %w", err)
	}

	// Record metric
	h.outputDB.RecordMetric("reader_digest_generated", 1.0)

	return digest, nil
}
