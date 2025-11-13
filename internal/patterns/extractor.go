package patterns

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"brainloop/internal/database"
	"database/sql"

	"github.com/google/uuid"
)

// Extractor extracts patterns from project files
type Extractor struct {
	lifecycleDB *database.LifecycleDB
}

// NewExtractor creates a new pattern extractor
func NewExtractor(lifecycleDBConn *sql.DB) *Extractor {
	return &Extractor{
		lifecycleDB: database.NewLifecycleDB(lifecycleDBConn),
	}
}

// ExtractForProject extracts patterns from a project directory
func (e *Extractor) ExtractForProject(projectPath string) (map[string]interface{}, error) {
	// Collect all Go and SQL files
	goFiles, err := e.findFiles(projectPath, ".go")
	if err != nil {
		return nil, fmt.Errorf("failed to find Go files: %w", err)
	}

	sqlFiles, err := e.findFiles(projectPath, ".sql")
	if err != nil {
		return nil, fmt.Errorf("failed to find SQL files: %w", err)
	}

	// Extract Go patterns
	var goPatterns map[string]interface{}
	if len(goFiles) > 0 {
		goPatterns = DetectGoPatterns(goFiles)
	}

	// Extract SQL patterns
	var sqlPatterns map[string]interface{}
	if len(sqlFiles) > 0 {
		sqlPatterns = DetectSQLPatterns(sqlFiles)
	}

	// Merge patterns
	patterns := make(map[string]interface{})
	if goPatterns != nil {
		patterns["go"] = goPatterns
	}
	if sqlPatterns != nil {
		patterns["sql"] = sqlPatterns
	}

	// Add metadata
	patterns["project_path"] = projectPath
	patterns["extracted_at"] = time.Now().Unix()
	patterns["go_file_count"] = len(goFiles)
	patterns["sql_file_count"] = len(sqlFiles)

	// Save patterns to database
	if err := e.savePatterns(projectPath, patterns); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to save patterns: %v\n", err)
	}

	return patterns, nil
}

// ExtractFromFiles extracts patterns from specific files
func (e *Extractor) ExtractFromFiles(filePaths []string) (map[string]interface{}, error) {
	var goFiles, sqlFiles []string

	for _, path := range filePaths {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".go":
			goFiles = append(goFiles, path)
		case ".sql":
			sqlFiles = append(sqlFiles, path)
		}
	}

	patterns := make(map[string]interface{})

	if len(goFiles) > 0 {
		patterns["go"] = DetectGoPatterns(goFiles)
	}
	if len(sqlFiles) > 0 {
		patterns["sql"] = DetectSQLPatterns(sqlFiles)
	}

	return patterns, nil
}

// findFiles recursively finds files with given extension
func (e *Extractor) findFiles(rootPath, extension string) ([]string, error) {
	var files []string

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and vendor
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check extension
		if strings.HasSuffix(path, extension) {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// savePatterns saves detected patterns to database
func (e *Extractor) savePatterns(sourcePath string, patterns map[string]interface{}) error {
	// Serialize patterns
	patternsJSON, err := json.Marshal(patterns)
	if err != nil {
		return fmt.Errorf("failed to marshal patterns: %w", err)
	}

	// Determine pattern type
	patternType := "project"
	if _, hasGo := patterns["go"]; hasGo {
		if _, hasSQL := patterns["sql"]; hasSQL {
			patternType = "mixed"
		} else {
			patternType = "go"
		}
	} else if _, hasSQL := patterns["sql"]; hasSQL {
		patternType = "sql"
	}

	// Calculate confidence score (placeholder)
	confidenceScore := 0.8

	// Insert into database
	patternID := uuid.New().String()

	// Use direct SQL since this is a specialized operation
	_, err = e.lifecycleDB.GetCachedDigest("dummy") // Access underlying DB
	// This is a workaround - in production, add a SavePattern method to LifecycleDB

	// For now, just return nil
	return nil
}

// GetPatterns retrieves patterns for a project from cache
func (e *Extractor) GetPatterns(projectPath string) (map[string]interface{}, error) {
	// Query detected_patterns table
	// For now, return empty map
	return make(map[string]interface{}), nil
}

// Pattern represents a detected pattern
type Pattern struct {
	PatternID       string                 `json:"pattern_id"`
	SourcePath      string                 `json:"source_path"`
	PatternType     string                 `json:"pattern_type"`
	PatternData     map[string]interface{} `json:"pattern_data"`
	ConfidenceScore float64                `json:"confidence_score"`
	DetectedAt      int64                  `json:"detected_at"`
}
