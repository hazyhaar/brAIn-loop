package readers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ReadCode reads and analyzes source code files
func (h *Hub) ReadCode(params map[string]interface{}) (string, error) {
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

	// Detect language
	language := h.detectLanguage(filePath)

	// Parse code
	var analysis map[string]interface{}
	switch language {
	case "go":
		analysis = h.parseGoCode(string(content))
	case "python":
		analysis = h.parsePythonCode(string(content))
	case "sql":
		analysis = h.parseSQLCode(string(content))
	default:
		analysis = h.parseGenericCode(string(content))
	}

	analysis["language"] = language
	analysis["file_path"] = filePath

	// Format analysis as JSON string
	analysisJSON, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal analysis: %w", err)
	}

	// Generate digest using Cerebras
	digest, err := h.generateDigest("code", string(analysisJSON))
	if err != nil {
		return "", err
	}

	// Save to cache
	if err := h.saveCache(hash, "code", filePath, digest); err != nil {
		fmt.Printf("Warning: failed to save cache: %v\n", err)
	}

	// Publish to output
	if err := h.publishDigest(hash, "code", filePath, digest); err != nil {
		fmt.Printf("Warning: failed to publish digest: %v\n", err)
	}

	return digest, nil
}

// detectLanguage detects programming language from file extension
func (h *Hub) detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	languages := map[string]string{
		".go":   "go",
		".py":   "python",
		".sql":  "sql",
		".js":   "javascript",
		".ts":   "typescript",
		".java": "java",
		".c":    "c",
		".cpp":  "cpp",
		".rs":   "rust",
	}

	if lang, ok := languages[ext]; ok {
		return lang
	}
	return "unknown"
}

// parseGoCode parses Go source code
func (h *Hub) parseGoCode(content string) map[string]interface{} {
	analysis := make(map[string]interface{})

	lines := strings.Split(content, "\n")

	// Extract package
	packageRegex := regexp.MustCompile(`^package\s+(\w+)`)
	for _, line := range lines {
		if matches := packageRegex.FindStringSubmatch(line); matches != nil {
			analysis["package"] = matches[1]
			break
		}
	}

	// Extract imports
	imports := h.extractGoImports(content)
	analysis["imports"] = imports
	analysis["import_count"] = len(imports)

	// Extract functions
	functions := h.extractGoFunctions(content)
	analysis["functions"] = functions
	analysis["function_count"] = len(functions)

	// Extract types (structs, interfaces)
	types := h.extractGoTypes(content)
	analysis["types"] = types
	analysis["type_count"] = len(types)

	// Extract constants and variables
	constants := h.extractGoConstants(content)
	analysis["constants"] = constants

	// Basic statistics
	analysis["line_count"] = len(lines)
	analysis["comment_lines"] = h.countCommentLines(lines, "//")

	return analysis
}

// extractGoImports extracts import statements
func (h *Hub) extractGoImports(content string) []string {
	var imports []string

	// Single import
	singleRegex := regexp.MustCompile(`import\s+"([^"]+)"`)
	matches := singleRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		imports = append(imports, match[1])
	}

	// Multi-line import block
	blockRegex := regexp.MustCompile(`import\s*\(\s*([^)]+)\)`)
	blockMatches := blockRegex.FindAllStringSubmatch(content, -1)
	for _, blockMatch := range blockMatches {
		importBlock := blockMatch[1]
		importRegex := regexp.MustCompile(`"([^"]+)"`)
		for _, imp := range importRegex.FindAllStringSubmatch(importBlock, -1) {
			imports = append(imports, imp[1])
		}
	}

	return imports
}

// extractGoFunctions extracts function definitions
func (h *Hub) extractGoFunctions(content string) []map[string]interface{} {
	var functions []map[string]interface{}
	funcRegex := regexp.MustCompile(`func\s+(?:\([^)]+\)\s+)?(\w+)\s*\(([^)]*)\)`)

	matches := funcRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		functions = append(functions, map[string]interface{}{
			"name":   match[1],
			"params": match[2],
		})
	}

	return functions
}

// extractGoTypes extracts type definitions
func (h *Hub) extractGoTypes(content string) []map[string]interface{} {
	var types []map[string]interface{}

	// Struct types
	structRegex := regexp.MustCompile(`type\s+(\w+)\s+struct`)
	matches := structRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		types = append(types, map[string]interface{}{
			"name": match[1],
			"kind": "struct",
		})
	}

	// Interface types
	interfaceRegex := regexp.MustCompile(`type\s+(\w+)\s+interface`)
	matches = interfaceRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		types = append(types, map[string]interface{}{
			"name": match[1],
			"kind": "interface",
		})
	}

	return types
}

// extractGoConstants extracts constant definitions
func (h *Hub) extractGoConstants(content string) []string {
	var constants []string
	constRegex := regexp.MustCompile(`const\s+(\w+)`)

	matches := constRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		constants = append(constants, match[1])
	}

	return constants
}

// parsePythonCode parses Python source code
func (h *Hub) parsePythonCode(content string) map[string]interface{} {
	analysis := make(map[string]interface{})

	lines := strings.Split(content, "\n")

	// Extract imports
	imports := h.extractPythonImports(content)
	analysis["imports"] = imports

	// Extract functions
	functions := h.extractPythonFunctions(content)
	analysis["functions"] = functions

	// Extract classes
	classes := h.extractPythonClasses(content)
	analysis["classes"] = classes

	// Basic statistics
	analysis["line_count"] = len(lines)
	analysis["comment_lines"] = h.countCommentLines(lines, "#")

	return analysis
}

// extractPythonImports extracts import statements
func (h *Hub) extractPythonImports(content string) []string {
	var imports []string
	importRegex := regexp.MustCompile(`(?:import|from)\s+([\w.]+)`)

	matches := importRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		imports = append(imports, match[1])
	}

	return imports
}

// extractPythonFunctions extracts function definitions
func (h *Hub) extractPythonFunctions(content string) []map[string]interface{} {
	var functions []map[string]interface{}
	funcRegex := regexp.MustCompile(`def\s+(\w+)\s*\(([^)]*)\)`)

	matches := funcRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		functions = append(functions, map[string]interface{}{
			"name":   match[1],
			"params": match[2],
		})
	}

	return functions
}

// extractPythonClasses extracts class definitions
func (h *Hub) extractPythonClasses(content string) []string {
	var classes []string
	classRegex := regexp.MustCompile(`class\s+(\w+)`)

	matches := classRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		classes = append(classes, match[1])
	}

	return classes
}

// parseSQLCode parses SQL code
func (h *Hub) parseSQLCode(content string) map[string]interface{} {
	analysis := make(map[string]interface{})

	// Extract CREATE TABLE statements
	tables := h.extractSQLTables(content)
	analysis["tables"] = tables

	// Extract PRAGMA statements
	pragmas := h.extractSQLPragmas(content)
	analysis["pragmas"] = pragmas

	// Extract CREATE INDEX statements
	indexes := h.extractSQLIndexes(content)
	analysis["indexes"] = indexes

	return analysis
}

// extractSQLTables extracts table names from CREATE TABLE
func (h *Hub) extractSQLTables(content string) []string {
	var tables []string
	tableRegex := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)

	matches := tableRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		tables = append(tables, match[1])
	}

	return tables
}

// extractSQLPragmas extracts PRAGMA statements
func (h *Hub) extractSQLPragmas(content string) []string {
	var pragmas []string
	pragmaRegex := regexp.MustCompile(`(?i)PRAGMA\s+([^;]+)`)

	matches := pragmaRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		pragmas = append(pragmas, strings.TrimSpace(match[1]))
	}

	return pragmas
}

// extractSQLIndexes extracts index names from CREATE INDEX
func (h *Hub) extractSQLIndexes(content string) []string {
	var indexes []string
	indexRegex := regexp.MustCompile(`(?i)CREATE\s+INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)

	matches := indexRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		indexes = append(indexes, match[1])
	}

	return indexes
}

// parseGenericCode provides basic analysis for unknown languages
func (h *Hub) parseGenericCode(content string) map[string]interface{} {
	lines := strings.Split(content, "\n")

	return map[string]interface{}{
		"line_count":      len(lines),
		"character_count": len(content),
		"blank_lines":     h.countBlankLines(lines),
	}
}

// countCommentLines counts lines starting with comment prefix
func (h *Hub) countCommentLines(lines []string, prefix string) int {
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			count++
		}
	}
	return count
}

// countBlankLines counts blank lines
func (h *Hub) countBlankLines(lines []string) int {
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			count++
		}
	}
	return count
}
