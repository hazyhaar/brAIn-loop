package bash

import (
	"database/sql"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"strconv"
	"time"
	_ "modernc.org/sqlite"
)

type Registry struct {
	db *sql.DB
}

func NewRegistry(dbPath string) (*Registry, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	registry := &Registry{db: db}
	if err := registry.initTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	return registry, nil
}

func (r *Registry) initTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS commands_registry (
		command_hash TEXT PRIMARY KEY,
		command_text TEXT NOT NULL,
		execution_count INTEGER DEFAULT 0,
		success_count INTEGER DEFAULT 0,
		failure_count INTEGER DEFAULT 0,
		avg_duration_ms INTEGER DEFAULT 0,
		last_executed INTEGER,
		first_seen INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		current_policy TEXT DEFAULT 'unknown',
		user_override TEXT,
		policy_reason TEXT,
		policy_last_updated INTEGER,
		promoted_at INTEGER,
		duplicate_check_enabled BOOLEAN DEFAULT 1,
		duplicate_threshold_ms INTEGER DEFAULT 1000,
		last_100_timestamps TEXT
	);`

	_, err := r.db.Exec(query)
	return err
}

func (r *Registry) GetOrCreateCommand(commandText string) (hash string, err error) {
	hash = calculateHash(commandText)

	var existingHash string
	err = r.db.QueryRow("SELECT command_hash FROM commands_registry WHERE command_hash = ?", hash).Scan(&existingHash)
	if err == nil {
		return hash, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("failed to query command: %w", err)
	}

	now := time.Now().Unix()
	_, err = r.db.Exec(`
		INSERT INTO commands_registry 
		(command_hash, command_text, first_seen, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?)`,
		hash, commandText, now, now, now)
	if err != nil {
		return "", fmt.Errorf("failed to insert command: %w", err)
	}

	return hash, nil
}

func (r *Registry) UpdateExecution(hash string, exitCode int, durationMs int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var last100Timestamps string
	var executionCount, successCount, failureCount, avgDurationMs int
	err = tx.QueryRow(`
		SELECT last_100_timestamps, execution_count, success_count, failure_count, avg_duration_ms 
		FROM commands_registry WHERE command_hash = ?`, hash).Scan(
		&last100Timestamps, &executionCount, &successCount, &failureCount, &avgDurationMs)
	if err != nil {
		return fmt.Errorf("failed to query command stats: %w", err)
	}

	timestamps := parseTimestamps(last100Timestamps)
	now := time.Now().Unix()
	timestamps = append(timestamps, now)
	if len(timestamps) > 100 {
		timestamps = timestamps[len(timestamps)-100:]
	}

	if exitCode == 0 {
		successCount++
	} else {
		failureCount++
	}
	executionCount++

	newAvgDurationMs := (avgDurationMs*(executionCount-1) + durationMs) / executionCount

	_, err = tx.Exec(`
		UPDATE commands_registry 
		SET execution_count = ?, success_count = ?, failure_count = ?, 
		    avg_duration_ms = ?, last_executed = ?, last_100_timestamps = ?, updated_at = ?
		WHERE command_hash = ?`,
		executionCount, successCount, failureCount, newAvgDurationMs, now, 
		formatTimestamps(timestamps), now, hash)
	if err != nil {
		return fmt.Errorf("failed to update execution stats: %w", err)
	}

	return tx.Commit()
}

func (r *Registry) GetPolicy(hash string) (string, error) {
	var currentPolicy, userOverride sql.NullString
	err := r.db.QueryRow(`
		SELECT current_policy, user_override 
		FROM commands_registry WHERE command_hash = ?`, hash).Scan(
		&currentPolicy, &userOverride)
	if err != nil {
		return "", fmt.Errorf("failed to query policy: %w", err)
	}

	if userOverride.Valid && userOverride.String != "" {
		return userOverride.String, nil
	}
	if currentPolicy.Valid {
		return currentPolicy.String, nil
	}
	return "unknown", nil
}

func (r *Registry) PromotePolicy(hash, newPolicy, reason string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var commandText string
	err = tx.QueryRow(`
		SELECT command_text FROM commands_registry WHERE command_hash = ? FOR UPDATE`, hash).Scan(&commandText)
	if err != nil {
		return fmt.Errorf("failed to lock command for update: %w", err)
	}

	if err := ValidatePromotionSecurity(commandText); err != nil {
		return fmt.Errorf("security validation failed: %w", err)
	}

	now := time.Now().Unix()
	_, err = tx.Exec(`
		UPDATE commands_registry 
		SET current_policy = ?, policy_reason = ?, policy_last_updated = ?, 
		    promoted_at = ?, updated_at = ?
		WHERE command_hash = ?`,
		newPolicy, reason, now, now, now, hash)
	if err != nil {
		return fmt.Errorf("failed to promote policy: %w", err)
	}

	return tx.Commit()
}

func (r *Registry) GetDuplicationCheck(hash string) (lastTimestamp int64, thresholdMs int, enabled bool, err error) {
	var last100Timestamps sql.NullString
	var duplicateThresholdMs sql.NullInt64
	var duplicateCheckEnabled sql.NullBool

	err = r.db.QueryRow(`
		SELECT last_100_timestamps, duplicate_threshold_ms, duplicate_check_enabled 
		FROM commands_registry WHERE command_hash = ?`, hash).Scan(
		&last100Timestamps, &duplicateThresholdMs, &duplicateCheckEnabled)
	if err != nil {
		return 0, 0, false, fmt.Errorf("failed to query duplication check: %w", err)
	}

	if duplicateCheckEnabled.Valid {
		enabled = duplicateCheckEnabled.Bool
	} else {
		enabled = true
	}

	if duplicateThresholdMs.Valid {
		thresholdMs = int(duplicateThresholdMs.Int64)
	} else {
		thresholdMs = 1000
	}

	if last100Timestamps.Valid && last100Timestamps.String != "" {
		timestamps := parseTimestamps(last100Timestamps.String)
		if len(timestamps) > 0 {
			lastTimestamp = timestamps[len(timestamps)-1]
		}
	}

	return lastTimestamp, thresholdMs, enabled, nil
}

func (r *Registry) GetCommandStats(hash string) (*CommandStats, error) {
	stats := &CommandStats{Hash: hash}
	
	var (
		commandText, currentPolicy, userOverride, policyReason, last100Timestamps sql.NullString
		executionCount, successCount, failureCount, avgDurationMs, duplicateThresholdMs sql.NullInt64
		lastExecuted, firstSeen, createdAt, updatedAt, policyLastUpdated, promotedAt sql.NullInt64
		duplicateCheckEnabled sql.NullBool
	)

	err := r.db.QueryRow(`
		SELECT command_text, execution_count, success_count, failure_count, 
		       avg_duration_ms, last_executed, first_seen, created_at, updated_at,
		       current_policy, user_override, policy_reason, policy_last_updated,
		       promoted_at, duplicate_check_enabled, duplicate_threshold_ms,
		       last_100_timestamps
		FROM commands_registry WHERE command_hash = ?`, hash).Scan(
		&commandText, &executionCount, &successCount, &failureCount,
		&avgDurationMs, &lastExecuted, &firstSeen, &createdAt, &updatedAt,
		&currentPolicy, &userOverride, &policyReason, &policyLastUpdated,
		&promotedAt, &duplicateCheckEnabled, &duplicateThresholdMs,
		&last100Timestamps)

	if err != nil {
		return nil, fmt.Errorf("failed to query command stats: %w", err)
	}

	if commandText.Valid {
		stats.CommandText = commandText.String
	}
	if executionCount.Valid {
		stats.ExecutionCount = int(executionCount.Int64)
	}
	if successCount.Valid {
		stats.SuccessCount = int(successCount.Int64)
	}
	if failureCount.Valid {
		stats.FailureCount = int(failureCount.Int64)
	}
	if avgDurationMs.Valid {
		stats.AvgDurationMs = int(avgDurationMs.Int64)
	}
	if lastExecuted.Valid {
		stats.LastExecuted = lastExecuted.Int64
	}
	if firstSeen.Valid {
		stats.FirstSeen = firstSeen.Int64
	}
	if currentPolicy.Valid {
		stats.CurrentPolicy = currentPolicy.String
	}
	if userOverride.Valid {
		stats.UserOverride = userOverride.String
	}
	if policyReason.Valid {
		stats.PolicyReason = policyReason.String
	}
	if policyLastUpdated.Valid {
		stats.PolicyLastUpdated = policyLastUpdated.Int64
	}
	if promotedAt.Valid {
		stats.PromotedAt = promotedAt.Int64
	}
	if duplicateCheckEnabled.Valid {
		stats.DuplicateEnabled = duplicateCheckEnabled.Bool
	}
	if duplicateThresholdMs.Valid {
		stats.DuplicateThresholdMs = int(duplicateThresholdMs.Int64)
	}
	if last100Timestamps.Valid {
		stats.Last100Timestamps = last100Timestamps.String

		// Convertir timestamps string vers []time.Time pour logique policy
		timestampsInt := parseTimestamps(stats.Last100Timestamps)
		stats.ExecutionTimestamps = make([]time.Time, len(timestampsInt))
		for i, ts := range timestampsInt {
			stats.ExecutionTimestamps[i] = time.Unix(ts, 0)
		}

		// Calculer LastExecutionTime
		if len(stats.ExecutionTimestamps) > 0 {
			stats.LastExecutionTime = stats.ExecutionTimestamps[len(stats.ExecutionTimestamps)-1]
		}

		// Calculer intervalle moyen pour détection patterns
		if len(stats.ExecutionTimestamps) > 1 {
			var totalInterval time.Duration
			for i := 1; i < len(stats.ExecutionTimestamps); i++ {
				interval := stats.ExecutionTimestamps[i].Sub(stats.ExecutionTimestamps[i-1])
				totalInterval += interval
			}
			stats.AvgIntervalSeconds = totalInterval.Seconds() / float64(len(stats.ExecutionTimestamps)-1)
		}
	}

	// Calculer RiskScore si non présent
	if stats.CommandText != "" {
		validator := &Validator{}
		stats.RiskScore = validator.CalculateRiskScore(stats.CommandText)
	}

	return stats, nil
}

func (r *Registry) UpdatePolicy(hash string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return fmt.Errorf("no updates provided")
	}

	setParts := make([]string, 0, len(updates))
	args := make([]interface{}, 0, len(updates)+1)

	for column, value := range updates {
		setParts = append(setParts, fmt.Sprintf("%s = ?", column))
		args = append(args, value)
	}

	args = append(args, hash)
	args = append(args, time.Now().Unix())

	query := fmt.Sprintf(`
		UPDATE commands_registry 
		SET %s, updated_at = ? 
		WHERE command_hash = ?`, strings.Join(setParts, ", "))

	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update policy: %w", err)
	}

	return nil
}

// SetPolicy sets the policy for a command
func (r *Registry) SetPolicy(hash, policy, reason string, isOverride bool) error {
	now := time.Now().Unix()

	if isOverride {
		_, err := r.db.Exec(`
			UPDATE commands_registry
			SET user_override = ?, policy_reason = ?, policy_last_updated = ?, updated_at = ?
			WHERE command_hash = ?
		`, policy, reason, now, now, hash)
		return err
	}

	_, err := r.db.Exec(`
		UPDATE commands_registry
		SET current_policy = ?, policy_reason = ?, policy_last_updated = ?, updated_at = ?
		WHERE command_hash = ?
	`, policy, reason, now, now, hash)
	return err
}

// PromoteToAutoApprove promotes a command to auto_approve policy
func (r *Registry) PromoteToAutoApprove(hash string) error {
	return r.PromotePolicy(hash, "auto_approve", "auto-promoted after successful executions")
}

// CheckAutoEvolution checks if a command qualifies for auto-promotion
// Returns true if promoted, false otherwise
func (r *Registry) CheckAutoEvolution(hash string) (bool, error) {
	stats, err := r.GetCommandStats(hash)
	if err != nil {
		return false, err
	}

	// Criteria for auto-promotion:
	// - At least 20 executions
	// - Success rate >= 95%
	// - Not already auto_approve
	if stats.ExecutionCount < 20 {
		return false, nil
	}

	successRate := float64(stats.SuccessCount) / float64(stats.ExecutionCount)
	if successRate < 0.95 {
		return false, nil
	}

	currentPolicy, err := r.GetPolicy(hash)
	if err != nil {
		return false, err
	}

	if currentPolicy == "auto_approve" {
		return false, nil // Already promoted
	}

	// Promote
	err = r.PromoteToAutoApprove(hash)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *Registry) Close() error {
	return r.db.Close()
}

func calculateHash(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}

func parseTimestamps(str string) []int64 {
	if str == "" {
		return []int64{}
	}

	parts := strings.Split(str, ";")
	timestamps := make([]int64, 0, len(parts))

	for _, part := range parts {
		if ts, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil {
			timestamps = append(timestamps, ts)
		}
	}

	return timestamps
}

func formatTimestamps(timestamps []int64) string {
	if len(timestamps) == 0 {
		return ""
	}

	strParts := make([]string, len(timestamps))
	for i, ts := range timestamps {
		strParts[i] = strconv.FormatInt(ts, 10)
	}

	return strings.Join(strParts, ";")
}