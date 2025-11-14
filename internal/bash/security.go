package bash

import (
	"encoding/json"
	"log"
	"regexp"
	"strings"
	"time"
)

var DangerousPatterns = []string{
	`(?i)rm\s+-rf\s+/`,
	`(?i)chmod\s+777`,
	`(?i)mkfs\.[a-z0-9]+`,
	`(?i)dd\s+if=/dev/`,
	`:\(\)\{.*\|.*&\s*\};:`,
	`(?i)wget.*\|.*sh`,
	`(?i)curl.*\|.*bash`,
	`(?i)eval\s+\$`,
	`(?i)sudo\s+(su|-i)`,
	`(?i)>\s+/dev/`,
	`(?i)rm\s+-rf\s+.*\*`,
	`(?i)chmod\s+-R\s+777`,
	`(?i)chown\s+-R\s+root`,
	`(?i)shred\s+.*\*`,
	`(?i)dd\s+of=/dev/`,
	`(?i)exec\s+.*sh`,
	`(?i)system\s*\(`,
	`(?i)export\s+PATH=.*\.\.`,
	`(?i)\$\(\s*.*\|\s*sh\s*\)`,
	"(?i)`\\s*.*\\|\\s*sh\\s*`",
}

type SecurityEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	CommandHash string    `json:"command_hash"`
	EventType   string    `json:"event_type"`
	Details     string    `json:"details"`
}

func MatchesDangerousPattern(command string) (bool, string) {
	normalizedCmd := strings.ToLower(strings.TrimSpace(command))
	
	for _, pattern := range DangerousPatterns {
		matched, err := regexp.MatchString(pattern, normalizedCmd)
		if err != nil {
			continue
		}
		if matched {
			return true, pattern
		}
	}
	
	return false, ""
}

func ValidatePromotionSecurity(command string) error {
	matched, pattern := MatchesDangerousPattern(command)
	if matched {
		event := SecurityEvent{
			Timestamp:   time.Now(),
			CommandHash: hashCommand(command),
			EventType:   "DANGEROUS_PATTERN_BLOCKED",
			Details:     pattern,
		}
		LogSecurityEvent(event)
		return &SecurityError{
			Message: "Command blocked due to dangerous pattern",
			Pattern: pattern,
		}
	}
	return nil
}

func LogSecurityEvent(event SecurityEvent) {
	jsonData, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal security event: %v", err)
		return
	}
	log.Printf("SECURITY_EVENT: %s", string(jsonData))
}

func hashCommand(command string) string {
	return strings.ReplaceAll(command, "\n", "\\n")
}

type SecurityError struct {
	Message string
	Pattern string
}

func (e *SecurityError) Error() string {
	return e.Message + ": " + e.Pattern
}