package bash

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

type Validator struct {
	maxLength int
}

func NewValidator() *Validator {
	return &Validator{
		maxLength: 4096,
	}
}

func (v *Validator) Validate(command string) error {
	if len(command) > v.maxLength {
		return fmt.Errorf("command exceeds maximum length of %d characters", v.maxLength)
	}

	if strings.Contains(command, "\x00") {
		return fmt.Errorf("command contains null bytes")
	}

	injectionPatterns := []string{
		`\$\(\s*wget`,
		`\$\(\s*curl`,
		`\$\(\s*nc`,
		`\$\(\s*netcat`,
		"`[^`]*`",
		`\$\(\s*sh`,
		`\$\(\s*bash`,
		`\$\(\s*zsh`,
		`\$\(\s*python`,
		`\$\(\s*perl`,
		`\$\(\s*ruby`,
		`\$\(\s*node`,
		`\$\(\s*php`,
	}

	for _, pattern := range injectionPatterns {
		if matched, _ := regexp.MatchString(pattern, command); matched {
			return fmt.Errorf("potential injection detected: %s", pattern)
		}
	}

	if strings.Contains(command, "/dev/tcp") || strings.Contains(command, "/dev/udp") {
		return fmt.Errorf("network redirection not allowed")
	}

	if strings.Contains(command, "sudo") || strings.Contains(command, "su ") {
		return fmt.Errorf("privilege escalation commands not allowed")
	}

	base64Pattern := regexp.MustCompile(`(base64\s+-d|echo\s+[^|]*\|\s*base64\s+-d)`)
	if base64Pattern.MatchString(command) {
		return fmt.Errorf("base64 decoding detected")
	}

	hexPattern := regexp.MustCompile(`(xxd\s+-r|echo\s+[^|]*\|\s*xxd\s+-r)`)
	if hexPattern.MatchString(command) {
		return fmt.Errorf("hex decoding detected")
	}

	return nil
}

func (v *Validator) SanitizeCommand(command string) (string, error) {
	trimmed := strings.TrimSpace(command)

	if trimmed == "" {
		return "", fmt.Errorf("empty command after sanitization")
	}

	for _, r := range trimmed {
		if !unicode.IsPrint(r) && r != ' ' && r != '\t' {
			return "", fmt.Errorf("invalid character detected in command")
		}
	}

	if err := v.Validate(trimmed); err != nil {
		return "", err
	}

	return trimmed, nil
}

func (v *Validator) CalculateRiskScore(command string) float64 {
	score := 0.3

	dangerousCommands := []string{"rm ", "dd ", "mkfs", "format", "fdisk"}
	for _, cmd := range dangerousCommands {
		if strings.Contains(command, cmd) {
			score += 0.3
			break
		}
	}

	modifyCommands := []string{"chmod", "chown", "chgrp"}
	for _, cmd := range modifyCommands {
		if strings.Contains(command, cmd) {
			score += 0.2
			break
		}
	}

	pipeCount := strings.Count(command, "|")
	if pipeCount > 2 {
		score += 0.1
	}

	redirectionPatterns := []string{">>", ">", "<", "2>", "2>>"}
	for _, pattern := range redirectionPatterns {
		if strings.Contains(command, pattern) {
			score += 0.05
			break
		}
	}

	if score > 1.0 {
		score = 1.0
	}

	return score
}