package cerebras

import (
	"encoding/json"
	"fmt"
)

// GenerateDigest generates a structured digest of source data
func (c *Client) GenerateDigest(sourceType, sourceData string) (string, error) {
	systemPrompt := buildDigestSystemPrompt(sourceType)
	userPrompt := buildDigestUserPrompt(sourceType, sourceData)

	// Generate with moderate temperature for balanced output
	result, err := c.Generate(systemPrompt, userPrompt, 0.3)
	if err != nil {
		return "", err
	}

	// Parse and validate JSON response
	var digest map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content), &digest); err != nil {
		// If not valid JSON, wrap in structure
		digest = map[string]interface{}{
			"summary":     result.Content,
			"source_type": sourceType,
		}
	}

	// Re-marshal to ensure valid JSON
	digestJSON, err := json.MarshalIndent(digest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal digest: %w", err)
	}

	return string(digestJSON), nil
}

// buildDigestSystemPrompt creates system prompt for digest generation
func buildDigestSystemPrompt(sourceType string) string {
	basePrompt := `You are an expert code analyst. Your task is to analyze source code/data and produce a structured, concise digest in JSON format.

IMPORTANT:
- Output ONLY valid JSON (no markdown, no explanations)
- Be concise but comprehensive
- Focus on actionable insights
- Extract key patterns and structures`

	specificPrompts := map[string]string{
		"sqlite": basePrompt + `

For SQLite databases, include:
{
  "database_summary": "Brief description",
  "tables": [
    {
      "name": "table_name",
      "columns": ["col1", "col2"],
      "row_count": 123,
      "sample_data": [...],
      "purpose": "What this table stores"
    }
  ],
  "schemas": ["DDL statements"],
  "pragmas": ["journal_mode=WAL", ...],
  "relationships": ["Foreign key relationships"],
  "recommendations": ["Performance tips", "Schema improvements"]
}`,

		"markdown": basePrompt + `

For Markdown documents, include:
{
  "document_summary": "Brief description",
  "structure": {
    "sections": ["Section 1", "Section 2"],
    "code_blocks": [{"language": "go", "lines": 50}]
  },
  "key_concepts": ["Concept 1", "Concept 2"],
  "code_examples": [{"language": "go", "purpose": "Example purpose"}],
  "recommendations": ["What to extract", "How to use"]
}`,

		"code": basePrompt + `

For source code, include:
{
  "language": "go",
  "summary": "Brief description",
  "structure": {
    "packages": ["package1"],
    "imports": ["import1", "import2"],
    "functions": [{"name": "funcName", "purpose": "..."}],
    "types": [{"name": "TypeName", "kind": "struct"}]
  },
  "patterns": {
    "naming_convention": "camelCase",
    "error_handling": "return errors",
    "testing_framework": "testing"
  },
  "dependencies": ["dep1", "dep2"],
  "recommendations": ["Best practices", "Improvements"]
}`,

		"config": basePrompt + `

For configuration files, include:
{
  "config_type": "json|yaml|toml",
  "summary": "Brief description",
  "structure": {
    "sections": ["database", "server"],
    "critical_settings": [{"key": "port", "value": 8080}]
  },
  "environment_vars": ["VAR1", "VAR2"],
  "secrets": ["Detected sensitive keys"],
  "recommendations": ["Security tips", "Best practices"]
}`,
	}

	if prompt, ok := specificPrompts[sourceType]; ok {
		return prompt
	}
	return basePrompt
}

// buildDigestUserPrompt creates user prompt for digest generation
func buildDigestUserPrompt(sourceType, sourceData string) string {
	// Truncate if too long (max 10000 chars for prompt)
	maxLen := 10000
	if len(sourceData) > maxLen {
		sourceData = sourceData[:maxLen] + "\n\n... (truncated for brevity)"
	}

	return fmt.Sprintf("Analyze this %s and provide a structured digest:\n\n%s", sourceType, sourceData)
}

// DigestResult represents a parsed digest
type DigestResult struct {
	SourceType      string                 `json:"source_type"`
	Summary         string                 `json:"summary"`
	Structure       map[string]interface{} `json:"structure,omitempty"`
	Patterns        map[string]interface{} `json:"patterns,omitempty"`
	Recommendations []string               `json:"recommendations,omitempty"`
	RawData         map[string]interface{} `json:"-"`
}

// ParseDigest parses a digest JSON string into structured format
func ParseDigest(digestJSON string) (*DigestResult, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(digestJSON), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse digest JSON: %w", err)
	}

	result := &DigestResult{
		RawData: raw,
	}

	// Extract common fields
	if sourceType, ok := raw["source_type"].(string); ok {
		result.SourceType = sourceType
	}
	if summary, ok := raw["summary"].(string); ok {
		result.Summary = summary
	}
	if structure, ok := raw["structure"].(map[string]interface{}); ok {
		result.Structure = structure
	}
	if patterns, ok := raw["patterns"].(map[string]interface{}); ok {
		result.Patterns = patterns
	}
	if recs, ok := raw["recommendations"].([]interface{}); ok {
		for _, rec := range recs {
			if recStr, ok := rec.(string); ok {
				result.Recommendations = append(result.Recommendations, recStr)
			}
		}
	}

	return result, nil
}

// GenerateMultiSourceDigest generates a combined digest from multiple sources
func (c *Client) GenerateMultiSourceDigest(sources map[string]string) (string, error) {
	// Combine all source data
	combinedPrompt := "Analyze these multiple sources and provide a unified digest:\n\n"

	for sourceType, sourceData := range sources {
		combinedPrompt += fmt.Sprintf("=== %s ===\n%s\n\n", sourceType, sourceData)
	}

	systemPrompt := `You are an expert code analyst. Analyze multiple sources and produce a unified digest in JSON format.

Include:
- Overall summary
- Cross-source patterns
- Dependencies between sources
- Unified recommendations`

	result, err := c.Generate(systemPrompt, combinedPrompt, 0.3)
	if err != nil {
		return "", err
	}

	return result.Content, nil
}
