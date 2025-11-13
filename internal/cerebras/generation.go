package cerebras

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GenerateCode generates code using Cerebras with pattern injection
func (c *Client) GenerateCode(prompt string, codeType string, patterns interface{}) (string, error) {
	// Build enhanced system prompt with patterns
	systemPrompt := buildSystemPrompt(codeType, patterns)

	// Generate with low temperature for deterministic output
	result, err := c.Generate(systemPrompt, prompt, 0.1)
	if err != nil {
		return "", err
	}

	// Clean code (remove markdown fences)
	code := cleanCode(result.Content, codeType)

	return code, nil
}

// GenerateCodeWithTemperature generates code with custom temperature
func (c *Client) GenerateCodeWithTemperature(prompt string, codeType string, patterns interface{}, temperature float64) (*GenerationResult, error) {
	// Build enhanced system prompt with patterns
	systemPrompt := buildSystemPrompt(codeType, patterns)

	// Generate with specified temperature
	return c.Generate(systemPrompt, prompt, temperature)
}

// buildSystemPrompt creates an enhanced system prompt with pattern injection
func buildSystemPrompt(codeType string, patterns interface{}) string {
	basePrompts := map[string]string{
		"go": `You are an expert Go programmer. Generate clean, idiomatic Go code following best practices.

IMPORTANT RULES:
- Use modernc.org/sqlite (NOT github.com/mattn/go-sqlite3)
- Follow HOROS patterns: 4-BDD architecture, idempotence via processed_log
- Use proper error handling (return errors, don't panic)
- Add comments for exported functions
- Use standard library when possible`,

		"sql": `You are an expert SQL database designer. Generate SQLite schemas following best practices.

IMPORTANT RULES:
- Always use CREATE TABLE IF NOT EXISTS
- Add PRIMARY KEY and FOREIGN KEY constraints
- Use appropriate indexes for performance
- Include comments explaining table purposes
- Use PRAGMA statements: journal_mode=WAL, foreign_keys=ON`,

		"python": `You are an expert Python programmer. Generate clean, Pythonic code following PEP 8.

IMPORTANT RULES:
- Use type hints
- Add docstrings to functions and classes
- Follow naming conventions (snake_case for functions, PascalCase for classes)
- Use standard library when possible`,

		"code": `You are an expert programmer. Generate clean, well-structured code following best practices for the target language.`,
	}

	systemPrompt := basePrompts[codeType]
	if systemPrompt == "" {
		systemPrompt = basePrompts["code"]
	}

	// Inject patterns if provided
	if patterns != nil {
		patternsJSON, err := json.MarshalIndent(patterns, "", "  ")
		if err == nil {
			systemPrompt += fmt.Sprintf("\n\nDETECTED PROJECT PATTERNS (follow these conventions):\n%s", string(patternsJSON))
		}
	}

	return systemPrompt
}

// cleanCode removes markdown code fences and trims whitespace
func cleanCode(content, codeType string) string {
	// Remove markdown code fences
	lines := strings.Split(content, "\n")
	var cleaned []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect code fence start
		if strings.HasPrefix(trimmed, "```") {
			if !inCodeBlock {
				// Entering code block
				inCodeBlock = true
				continue
			} else {
				// Exiting code block
				inCodeBlock = false
				continue
			}
		}

		// Only include lines inside code block or if no fences detected
		if inCodeBlock || !hasCodeFences(content) {
			cleaned = append(cleaned, line)
		}
	}

	result := strings.Join(cleaned, "\n")
	return strings.TrimSpace(result)
}

// hasCodeFences checks if content contains markdown code fences
func hasCodeFences(content string) bool {
	return strings.Contains(content, "```")
}

// ExtractCodeBlocks extracts multiple code blocks from markdown response
func ExtractCodeBlocks(content string) []CodeBlock {
	var blocks []CodeBlock
	lines := strings.Split(content, "\n")

	var currentBlock *CodeBlock
	var currentLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			if currentBlock == nil {
				// Start new block
				lang := strings.TrimPrefix(trimmed, "```")
				currentBlock = &CodeBlock{Language: lang}
				currentLines = []string{}
			} else {
				// End current block
				currentBlock.Content = strings.Join(currentLines, "\n")
				blocks = append(blocks, *currentBlock)
				currentBlock = nil
				currentLines = nil
			}
		} else if currentBlock != nil {
			// Inside code block
			currentLines = append(currentLines, line)
		}
	}

	return blocks
}

// CodeBlock represents a code block with language
type CodeBlock struct {
	Language string
	Content  string
}

// ValidateCode performs basic validation on generated code
func ValidateCode(code string, codeType string) error {
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("generated code is empty")
	}

	// Type-specific validation
	switch codeType {
	case "go":
		if !strings.Contains(code, "package") {
			return fmt.Errorf("Go code missing package declaration")
		}
	case "sql":
		if !strings.Contains(strings.ToUpper(code), "CREATE TABLE") {
			return fmt.Errorf("SQL code missing CREATE TABLE statement")
		}
	}

	return nil
}
