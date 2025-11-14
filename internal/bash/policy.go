package bash

import (
	"fmt"
	"time"
)

type CommandStats struct {
	// Identité
	Hash        string
	CommandText string

	// Timestamps principaux
	FirstSeen    int64
	LastExecuted int64
	CreatedAt    int64
	UpdatedAt    int64

	// Statistiques exécution
	ExecutionCount int
	SuccessCount   int
	FailureCount   int

	// Métriques performance
	AvgDurationMs int

	// Policy dynamique
	CurrentPolicy     string
	PolicyReason      string
	PolicyLastUpdated int64
	PromotedAt        int64

	// Override utilisateur
	UserOverride string

	// Détection duplication
	DuplicateThresholdMs int
	DuplicateEnabled     bool

	// Classification et risque
	RiskScore float64

	// Historique temporel (format texte DB)
	Last100Timestamps string

	// Champs calculés (pour logique policy)
	AvgIntervalSeconds  float64
	LastExecutionTime   time.Time
	ExecutionTimestamps []time.Time
}

type PolicyManager struct {
	registry *Registry
}

func NewPolicyManager(registry *Registry) *PolicyManager {
	return &PolicyManager{
		registry: registry,
	}
}

func (pm *PolicyManager) CheckAutoEvolution(hash string) error {
	stats, err := pm.registry.GetCommandStats(hash)
	if err != nil {
		return fmt.Errorf("failed to get command stats: %w", err)
	}

	// Never promote if risk score is too high
	if stats.RiskScore >= 0.7 {
		return nil
	}

	// Rule 1: Promote to auto_approve based on execution metrics
	if pm.ShouldPromoteToAutoApprove(stats) {
		if err := pm.registry.PromotePolicy(hash, "auto_approve", "Auto: 20+ exec, 95%+ success"); err != nil {
			return fmt.Errorf("failed to promote policy to auto_approve: %w", err)
		}
	}

	// Rule 2: Detect monitoring pattern and disable duplicate check
	if pm.DetectMonitoringPattern(stats.ExecutionTimestamps) && stats.ExecutionCount >= 50 {
		if err := pm.registry.UpdatePolicy(hash, map[string]interface{}{
			"duplicate_check": false,
			"policy_type":     "monitoring",
		}); err != nil {
			return fmt.Errorf("failed to update monitoring policy: %w", err)
		}
	}

	// Rule 3: Detect rare command and increase duplicate threshold
	if pm.DetectRareCommandPattern(stats.ExecutionTimestamps) {
		if err := pm.registry.UpdatePolicy(hash, map[string]interface{}{
			"duplicate_threshold": 30000,
		}); err != nil {
			return fmt.Errorf("failed to update rare command policy: %w", err)
		}
	}

	return nil
}

func (pm *PolicyManager) DetectMonitoringPattern(timestamps []time.Time) bool {
	if len(timestamps) < 10 {
		return false
	}

	var totalInterval time.Duration
	count := 0

	for i := 1; i < len(timestamps); i++ {
		interval := timestamps[i].Sub(timestamps[i-1])
		totalInterval += interval
		count++
	}

	if count == 0 {
		return false
	}

	avgInterval := totalInterval.Seconds() / float64(count)
	return avgInterval < 5.0
}

func (pm *PolicyManager) DetectRareCommandPattern(timestamps []time.Time) bool {
	if len(timestamps) < 2 {
		return false
	}

	var totalInterval time.Duration
	count := 0

	for i := 1; i < len(timestamps); i++ {
		interval := timestamps[i].Sub(timestamps[i-1])
		totalInterval += interval
		count++
	}

	if count == 0 {
		return false
	}

	avgInterval := totalInterval.Seconds() / float64(count)
	return avgInterval > 3600.0
}

func (pm *PolicyManager) ShouldPromoteToAutoApprove(stats *CommandStats) bool {
	if stats.CurrentPolicy != "ask" {
		return false
	}

	if stats.ExecutionCount < 20 {
		return false
	}

	successRate := float64(stats.SuccessCount) / float64(stats.ExecutionCount)
	if successRate < 0.95 {
		return false
	}

	// Additional conservative checks
	if stats.RiskScore >= 0.5 {
		return false
	}

	// Ensure command has been executed recently (within last 30 days)
	if time.Since(stats.LastExecutionTime) > 30*24*time.Hour {
		return false
	}

	return true
}