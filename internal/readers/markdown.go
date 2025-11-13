package readers

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ReadMarkdown reads and analyzes a markdown file
func (h *Hub) ReadMarkdown(params map[string]interface{}) (string, error) {
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

	// Parse markdown
	analysis := h.parseMarkdown(string(content))

	// Format analysis as JSON string
	analysisJSON, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal analysis: %w", err)
	}

	// Generate digest using Cerebras
	digest, err := h.generateDigest("markdown", string(analysisJSON))
	if err != nil {
		return "", err
	}

	// Save to cache
	if err := h.saveCache(hash, "markdown", filePath, digest); err != nil {
		fmt.Printf("Warning: failed to save cache: %v\n", err)
	}

	// Publish to output
	if err := h.publishDigest(hash, "markdown", filePath, digest); err != nil {
		fmt.Printf("Warning: failed to publish digest: %v\n", err)
	}

	return digest, nil
}

// parseMarkdown parses markdown content and extracts structure
func (h *Hub) parseMarkdown(content string) map[string]interface{} {
	analysis := make(map[string]interface{})

	lines := strings.Split(content, "\n")

	// Extract sections (headers)
	sections := h.extractMarkdownSections(lines)
	analysis["sections"] = sections

	// Extract code blocks
	codeBlocks := h.extractMarkdownCodeBlocks(lines)
	analysis["code_blocks"] = codeBlocks
	analysis["code_block_count"] = len(codeBlocks)

	// Extract links
	links := h.extractMarkdownLinks(content)
	analysis["links"] = links
	analysis["link_count"] = len(links)

	// Extract images
	images := h.extractMarkdownImages(content)
	analysis["images"] = images

	// Extract lists
	lists := h.extractMarkdownLists(lines)
	analysis["lists"] = lists

	// Basic statistics
	analysis["line_count"] = len(lines)
	analysis["character_count"] = len(content)
	analysis["word_count"] = len(strings.Fields(content))

	return analysis
}

// extractMarkdownSections extracts section headers
func (h *Hub) extractMarkdownSections(lines []string) []map[string]interface{} {
	var sections []map[string]interface{}
	headerRegex := regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

	for lineNum, line := range lines {
		if matches := headerRegex.FindStringSubmatch(line); matches != nil {
			level := len(matches[1])
			title := matches[2]

			sections = append(sections, map[string]interface{}{
				"level":     level,
				"title":     title,
				"line_number": lineNum + 1,
			})
		}
	}

	return sections
}

// extractMarkdownCodeBlocks extracts code blocks with language
func (h *Hub) extractMarkdownCodeBlocks(lines []string) []map[string]interface{} {
	var codeBlocks []map[string]interface{}
	var currentBlock *map[string]interface{}
	var currentCode []string

	fenceRegex := regexp.MustCompile(`^```(\w*)`)

	for lineNum, line := range lines {
		if matches := fenceRegex.FindStringSubmatch(line); matches != nil {
			if currentBlock == nil {
				// Start new code block
				lang := matches[1]
				if lang == "" {
					lang = "text"
				}
				currentBlock = &map[string]interface{}{
					"language":   lang,
					"start_line": lineNum + 1,
				}
				currentCode = []string{}
			} else {
				// End current code block
				(*currentBlock)["code"] = strings.Join(currentCode, "\n")
				(*currentBlock)["end_line"] = lineNum + 1
				(*currentBlock)["line_count"] = len(currentCode)
				codeBlocks = append(codeBlocks, *currentBlock)
				currentBlock = nil
				currentCode = nil
			}
		} else if currentBlock != nil {
			currentCode = append(currentCode, line)
		}
	}

	return codeBlocks
}

// extractMarkdownLinks extracts all links
func (h *Hub) extractMarkdownLinks(content string) []map[string]interface{} {
	var links []map[string]interface{}
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

	matches := linkRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		links = append(links, map[string]interface{}{
			"text": match[1],
			"url":  match[2],
		})
	}

	return links
}

// extractMarkdownImages extracts all images
func (h *Hub) extractMarkdownImages(content string) []map[string]interface{} {
	var images []map[string]interface{}
	imageRegex := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)

	matches := imageRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		images = append(images, map[string]interface{}{
			"alt": match[1],
			"url": match[2],
		})
	}

	return images
}

// extractMarkdownLists extracts list items
func (h *Hub) extractMarkdownLists(lines []string) map[string]interface{} {
	unorderedCount := 0
	orderedCount := 0

	unorderedRegex := regexp.MustCompile(`^[\s]*[-*+]\s+`)
	orderedRegex := regexp.MustCompile(`^[\s]*\d+\.\s+`)

	for _, line := range lines {
		if unorderedRegex.MatchString(line) {
			unorderedCount++
		} else if orderedRegex.MatchString(line) {
			orderedCount++
		}
	}

	return map[string]interface{}{
		"unordered_items": unorderedCount,
		"ordered_items":   orderedCount,
		"total_items":     unorderedCount + orderedCount,
	}
}
