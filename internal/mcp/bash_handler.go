package mcp

import (
	"brainloop/internal/bash"
	"fmt"
	"log"
	"time"
)

type BashHandler struct {
	registry     *bash.Registry
	executor     *bash.Executor
	validator    *bash.Validator
	policyManager *bash.PolicyManager
}

type ExecutionResponse struct {
	Success       bool    `json:"success"`
	ExitCode      int     `json:"exit_code,omitempty"`
	Stdout        string  `json:"stdout,omitempty"`
	Stderr        string  `json:"stderr,omitempty"`
	DurationMs    int64   `json:"duration_ms,omitempty"`
	PolicyUsed    string  `json:"policy_used,omitempty"`
	Policy        string  `json:"policy,omitempty"`
	Status        string  `json:"status,omitempty"`
	Command       string  `json:"command,omitempty"`
	RiskScore     float64 `json:"risk_score,omitempty"`
	SecondsSinceLast int64 `json:"seconds_since_last,omitempty"`
}

func NewBashHandler(dbPath string) (*BashHandler, error) {
	registry, err := bash.NewRegistry(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize registry: %w", err)
	}

	executor := bash.NewExecutor()
	validator := bash.NewValidator()
	policyManager := bash.NewPolicyManager(registry)

	return &BashHandler{
		registry:     registry,
		executor:     executor,
		validator:    validator,
		policyManager: policyManager,
	}, nil
}

func (h *BashHandler) HandleExecuteBash(params map[string]interface{}) (interface{}, error) {
	// Étape 2: Extraire command string des params
	command, ok := params["command"].(string)
	if !ok {
		return nil, fmt.Errorf("command parameter is required and must be a string")
	}
	if command == "" {
		return nil, fmt.Errorf("command cannot be empty")
	}

	// Étape 3: Extraire force_execute bool optionnel
	forceExecute := false
	if fe, exists := params["force_execute"].(bool); exists {
		forceExecute = fe
	}

	// Étape 4: Créer validator, valider commande
	if err := h.validator.Validate(command); err != nil {
		log.Printf("[SECURITY] Invalid command rejected: %s - Error: %v", command, err)
		return nil, fmt.Errorf("command validation failed: %w", err)
	}

	// Étape 5: Calculer risk_score
	riskScore := h.validator.CalculateRiskScore(command)
	if riskScore >= 0.8 {
		log.Printf("[SECURITY] High risk command detected (score: %.2f): %s", riskScore, command)
	}

	// Étape 6: Ouvrir command_security.db registry (déjà ouvert dans NewBashHandler)

	// Étape 7: GetOrCreateCommand (retourne hash string)
	cmdHash, err := h.registry.GetOrCreateCommand(command)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create command entry: %w", err)
	}

	// Étape 8: GetPolicy (méthode de Registry, pas PolicyManager)
	policy, err := h.registry.GetPolicy(cmdHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get policy: %w", err)
	}

	// Étape 9: Si policy = 'auto_approve' OU force_execute = true → aller étape 13
	if policy == "auto_approve" || forceExecute {
		return h.executeCommand(command, cmdHash, policy)
	}

	// Étape 10: Si policy = 'ask' ou 'ask_warning'
	if policy == "ask" || policy == "ask_warning" {
		// GetDuplicationCheck retourne (lastTimestamp, thresholdMs, enabled, error)
		lastTimestamp, thresholdMs, enabled, err := h.registry.GetDuplicationCheck(cmdHash)
		if err != nil {
			log.Printf("[WARNING] Failed to check duplication for hash %s: %v", cmdHash, err)
		}

		// Si duplicate détecté (intervalle < seuil et check activé)
		if enabled && lastTimestamp > 0 {
			now := time.Now().Unix()
			secondsSinceLast := now - lastTimestamp
			if secondsSinceLast*1000 < int64(thresholdMs) {
				log.Printf("[SECURITY] Duplicate command detected: %s (seconds since last: %d)",
					command, secondsSinceLast)
				return &ExecutionResponse{
					Success:          false,
					Status:          "duplicate_warning",
					Command:         command,
					SecondsSinceLast: secondsSinceLast,
				}, nil
			}
		}

		// Sinon retourner pending_validation
		return &ExecutionResponse{
			Success:   false,
			Status:   "pending_validation",
			Command:  command,
			Policy:   policy,
			RiskScore: riskScore,
		}, nil
	}

	// Étape 11 & 12: L'utilisateur MCP reçoit pending et demande confirmation
	// (géré par le client MCP, retour à handleExecuteBash avec force_execute=true)

	// Étape 13: Créer executor, Execute(command) (déjà créé dans NewBashHandler)
	return h.executeCommand(command, cmdHash, policy)
}

func (h *BashHandler) executeCommand(command, hash, policy string) (interface{}, error) {
	startTime := time.Now()

	// Étape 13: Exécuter la commande (Execute retourne *ExecutionResult)
	result := h.executor.Execute(command)

	// Vérifier si erreur dans le résultat
	if result.Error != "" {
		// Même en cas d'erreur d'exécution, on met à jour le registry
		durationMs := int(time.Since(startTime).Milliseconds())
		updateErr := h.registry.UpdateExecution(hash, result.ExitCode, durationMs)
		if updateErr != nil {
			log.Printf("[ERROR] Failed to update execution after error: %v", updateErr)
		}
		return nil, fmt.Errorf("command execution failed: %s", result.Error)
	}

	durationMs := int(time.Since(startTime).Milliseconds())

	// Étape 14: UpdateExecution dans registry
	if err := h.registry.UpdateExecution(hash, result.ExitCode, durationMs); err != nil {
		log.Printf("[ERROR] Failed to update execution in registry: %v", err)
	}

	// Étape 15: CheckAutoEvolution
	if err := h.policyManager.CheckAutoEvolution(hash); err != nil {
		log.Printf("[WARNING] Failed to check auto evolution: %v", err)
	}

	// Log des commandes exécutées avec risque élevé
	if h.validator.CalculateRiskScore(command) >= 0.6 {
		log.Printf("[AUDIT] Executed command (risk: %.2f): %s",
			h.validator.CalculateRiskScore(command), command)
	}

	// Étape 16: Retourner le résultat
	return &ExecutionResponse{
		Success:    result.ExitCode == 0,
		ExitCode:   result.ExitCode,
		Stdout:     result.Stdout,
		Stderr:     result.Stderr,
		DurationMs: int64(durationMs),
		PolicyUsed: policy,
	}, nil
}

func (h *BashHandler) Close() error {
	if h.registry != nil {
		return h.registry.Close()
	}
	return nil
}