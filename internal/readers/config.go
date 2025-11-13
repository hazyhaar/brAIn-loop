package readers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadConfig reads and analyzes configuration files (JSON/YAML/TOML)
func (h *Hub) ReadConfig(params map[string]interface{}) (string, error) {
	// Extract parameters
	filePath, ok := params["file_path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid file_path parameter")
	}

	// Compute hash for caching
	hash, err := h.computeHash(filePath)
	if err != nil {
		return "", err
	}

	// Check cache
	if digest, found := h.checkCache(hash); found {
		h.outputDB.RecordMetric("reader_cache_hit", 1.0)
		return digest, nil
	}

	h.outputDB.RecordMetric("reader_cache_miss", 1.0)

	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Detect config type
	configType := h.detectConfigType(filePath)

	// Parse config
	var analysis map[string]interface{}
	switch configType {
	case "json":
		analysis, err = h.parseJSONConfig(string(content))
	case "yaml":
		analysis, err = h.parseYAMLConfig(string(content))
	case "toml":
		analysis, err = h.parseTOMLConfig(string(content))
	default:
		analysis = h.parseGenericConfig(string(content))
	}

	if err != nil {
		return "", fmt.Errorf("failed to parse config: %w", err)
	}

	analysis["config_type"] = configType
	analysis["file_path"] = filePath

	// Format analysis as JSON string
	analysisJSON, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal analysis: %w", err)
	}

	// Generate digest using Cerebras
	digest, err := h.generateDigest("config", string(analysisJSON))
	if err != nil {
		return "", err
	}

	// Save to cache
	if err := h.saveCache(hash, "config", filePath, digest); err != nil {
		fmt.Printf("Warning: failed to save cache: %v\n", err)
	}

	// Publish to output
	if err := h.publishDigest(hash, "config", filePath, digest); err != nil {
		fmt.Printf("Warning: failed to publish digest: %v\n", err)
	}

	return digest, nil
}

// detectConfigType detects configuration file type from extension
func (h *Hub) detectConfigType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	base := strings.ToLower(filepath.Base(filePath))

	// Check extensions
	types := map[string]string{
		".json": "json",
		".yaml": "yaml",
		".yml":  "yaml",
		".toml": "toml",
	}

	if configType, ok := types[ext]; ok {
		return configType
	}

	// Check common config filenames
	if strings.HasSuffix(base, "config.json") || base == "package.json" {
		return "json"
	}

	return "unknown"
}

// parseJSONConfig parses JSON configuration
func (h *Hub) parseJSONConfig(content string) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	analysis := make(map[string]interface{})

	// Extract top-level keys
	topLevelKeys := make([]string, 0, len(data))
	for key := range data {
		topLevelKeys = append(topLevelKeys, key)
	}
	analysis["top_level_keys"] = topLevelKeys
	analysis["key_count"] = len(topLevelKeys)

	// Detect critical settings
	criticalSettings := h.detectCriticalSettings(data)
	analysis["critical_settings"] = criticalSettings

	// Detect environment variables
	envVars := h.detectEnvironmentVars(data)
	if len(envVars) > 0 {
		analysis["environment_vars"] = envVars
	}

	// Detect secrets
	secrets := h.detectSecrets(data)
	if len(secrets) > 0 {
		analysis["potential_secrets"] = secrets
	}

	// Structure summary
	analysis["structure"] = h.summarizeStructure(data)

	return analysis, nil
}

// parseYAMLConfig parses YAML configuration (basic)
func (h *Hub) parseYAMLConfig(content string) (map[string]interface{}, error) {
	// For now, provide basic analysis without full YAML parsing
	// (Would need gopkg.in/yaml.v3 for full support)

	analysis := make(map[string]interface{})

	lines := strings.Split(content, "\n")
	analysis["line_count"] = len(lines)

	// Extract top-level keys (simplified)
	topLevelKeys := h.extractYAMLKeys(lines)
	analysis["top_level_keys"] = topLevelKeys

	// Detect environment variable references
	envVars := h.detectYAMLEnvVars(content)
	if len(envVars) > 0 {
		analysis["environment_vars"] = envVars
	}

	return analysis, nil
}

// parseTOMLConfig parses TOML configuration (basic)
func (h *Hub) parseTOMLConfig(content string) (map[string]interface{}, error) {
	// For now, provide basic analysis without full TOML parsing
	// (Would need github.com/BurntSushi/toml for full support)

	analysis := make(map[string]interface{})

	lines := strings.Split(content, "\n")
	analysis["line_count"] = len(lines)

	// Extract sections
	sections := h.extractTOMLSections(lines)
	analysis["sections"] = sections

	return analysis, nil
}

// parseGenericConfig provides basic analysis for unknown config formats
func (h *Hub) parseGenericConfig(content string) map[string]interface{} {
	lines := strings.Split(content, "\n")

	return map[string]interface{}{
		"line_count":      len(lines),
		"character_count": len(content),
	}
}

// detectCriticalSettings detects common critical configuration settings
func (h *Hub) detectCriticalSettings(data map[string]interface{}) []map[string]interface{} {
	var critical []map[string]interface{}

	criticalKeys := []string{
		"port", "host", "database", "db", "api_key", "secret",
		"password", "token", "url", "endpoint", "timeout",
	}

	for _, key := range criticalKeys {
		if value, exists := data[key]; exists {
			critical = append(critical, map[string]interface{}{
				"key":   key,
				"value": value,
			})
		}
	}

	return critical
}

// detectEnvironmentVars detects environment variable references
func (h *Hub) detectEnvironmentVars(data map[string]interface{}) []string {
	var envVars []string
	h.findEnvVarsRecursive(data, &envVars)
	return uniqueStrings(envVars)
}

// findEnvVarsRecursive recursively finds environment variable references
func (h *Hub) findEnvVarsRecursive(data interface{}, envVars *[]string) {
	switch v := data.(type) {
	case map[string]interface{}:
		for _, value := range v {
			h.findEnvVarsRecursive(value, envVars)
		}
	case []interface{}:
		for _, item := range v {
			h.findEnvVarsRecursive(item, envVars)
		}
	case string:
		// Look for ${VAR} or $VAR patterns
		if strings.Contains(v, "${") || strings.HasPrefix(v, "$") {
			*envVars = append(*envVars, v)
		}
	}
}

// detectSecrets detects potential secret keys
func (h *Hub) detectSecrets(data map[string]interface{}) []string {
	var secrets []string

	secretKeywords := []string{"secret", "password", "token", "api_key", "private_key", "credential"}

	for key := range data {
		lowerKey := strings.ToLower(key)
		for _, keyword := range secretKeywords {
			if strings.Contains(lowerKey, keyword) {
				secrets = append(secrets, key)
				break
			}
		}
	}

	return secrets
}

// summarizeStructure provides a summary of the data structure
func (h *Hub) summarizeStructure(data map[string]interface{}) map[string]interface{} {
	summary := make(map[string]interface{})

	objectCount := 0
	arrayCount := 0
	primitiveCount := 0

	for _, value := range data {
		switch value.(type) {
		case map[string]interface{}:
			objectCount++
		case []interface{}:
			arrayCount++
		default:
			primitiveCount++
		}
	}

	summary["objects"] = objectCount
	summary["arrays"] = arrayCount
	summary["primitives"] = primitiveCount

	return summary
}

// extractYAMLKeys extracts top-level keys from YAML (simplified)
func (h *Hub) extractYAMLKeys(lines []string) []string {
	var keys []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Top-level keys don't have leading spaces
		if !strings.HasPrefix(line, " ") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			key := strings.TrimSpace(parts[0])
			if key != "" {
				keys = append(keys, key)
			}
		}
	}

	return keys
}

// detectYAMLEnvVars detects environment variable references in YAML
func (h *Hub) detectYAMLEnvVars(content string) []string {
	var envVars []string

	// Look for ${VAR} or $VAR patterns
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.Contains(line, "${") || strings.Contains(line, "$") {
			envVars = append(envVars, strings.TrimSpace(line))
		}
	}

	return envVars
}

// extractTOMLSections extracts sections from TOML
func (h *Hub) extractTOMLSections(lines []string) []string {
	var sections []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section := strings.Trim(trimmed, "[]")
			sections = append(sections, section)
		}
	}

	return sections
}

// uniqueStrings returns unique strings from a slice
func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}
