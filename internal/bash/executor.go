package bash

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	defaultTimeout      = 120 * time.Second
	defaultMaxOutput    = 10 * 1024 // 10KB
	defaultWorkingDir   = "/workspace"
	maxCommandLength    = 4096
	forbiddenCommands   = "sudo|su|passwd|chroot|mount|umount|fdisk|mkfs|format"
)

// Executor configure et exécute des commandes bash de manière sécurisée
type Executor struct {
	timeout       time.Duration
	maxOutputBytes int
	workingDir    string
	allowedEnv    []string
}

// ExecutionResult contient le résultat d'une exécution de commande
type ExecutionResult struct {
	ExitCode    int    `json:"exit_code"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	DurationMs  int64  `json:"duration_ms"`
	Error       string `json:"error,omitempty"`
	WasTimeout  bool   `json:"was_timeout"`
	WasTruncated bool  `json:"was_truncated"`
}

// NewExecutor crée une nouvelle instance d'Executor avec les valeurs par défaut
func NewExecutor() *Executor {
	return &Executor{
		timeout:       defaultTimeout,
		maxOutputBytes: defaultMaxOutput,
		workingDir:    defaultWorkingDir,
		allowedEnv:    []string{"PATH", "HOME", "USER", "LANG", "LC_ALL", "TERM"},
	}
}

// WithTimeout définit le timeout d'exécution
func (e *Executor) WithTimeout(timeout time.Duration) *Executor {
	e.timeout = timeout
	return e
}

// WithMaxOutputBytes définit la taille maximale de sortie
func (e *Executor) WithMaxOutputBytes(maxBytes int) *Executor {
	e.maxOutputBytes = maxBytes
	return e
}

// WithWorkingDir définit le répertoire de travail
func (e *Executor) WithWorkingDir(dir string) *Executor {
	e.workingDir = dir
	return e
}

// Execute exécute une commande bash de manière sécurisée
func (e *Executor) Execute(command string) *ExecutionResult {
	result := &ExecutionResult{}
	startTime := time.Now()

	// Validation de la commande
	if err := e.validateCommand(command); err != nil {
		result.Error = err.Error()
		result.DurationMs = time.Since(startTime).Milliseconds()
		return result
	}

	// Création du contexte avec timeout
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	// Préparation de la commande
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", command)
	
	// Configuration du répertoire de travail
	if e.workingDir != "" {
		// Vérifier que le chemin est absolu et sécurisé
		absPath, err := filepath.Abs(e.workingDir)
		if err != nil {
			result.Error = fmt.Sprintf("invalid working directory: %v", err)
			result.DurationMs = time.Since(startTime).Milliseconds()
			return result
		}
		cmd.Dir = absPath
	}

	// Configuration des variables d'environnement filtrées
	cmd.Env = e.filterEnvironment()

	// Capture de stdout et stderr avec limite de taille
	var stdoutBuf, stderrBuf limitedBuffer
	stdoutBuf.limit = e.maxOutputBytes
	stderrBuf.limit = e.maxOutputBytes

	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Exécution de la commande
	err := cmd.Run()
	result.DurationMs = time.Since(startTime).Milliseconds()

	// Traitement du résultat
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.WasTimeout = true
			result.Error = "command timed out"
			// Forcer l'arrêt du processus
			if cmd.Process != nil {
				cmd.Process.Signal(syscall.SIGKILL)
			}
		} else if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				result.ExitCode = status.ExitStatus()
			}
		} else {
			result.Error = err.Error()
		}
	}

	// Récupération de la sortie
	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()
	result.WasTruncated = stdoutBuf.truncated || stderrBuf.truncated

	// Si pas d'erreur et exit code non défini, c'est un succès
	if result.ExitCode == 0 && result.Error == "" {
		result.ExitCode = 0
	}

	return result
}

// validateCommand vérifie que la commande est sécurisée
func (e *Executor) validateCommand(command string) error {
	// Vérifier la longueur de la commande
	if len(command) > maxCommandLength {
		return fmt.Errorf("command too long (max %d characters)", maxCommandLength)
	}

	// Vérifier les commandes interdites
	if strings.Contains(strings.ToLower(command), "sudo") ||
		strings.Contains(strings.ToLower(command), "su ") ||
		strings.Contains(strings.ToLower(command), "passwd") ||
		strings.Contains(strings.ToLower(command), "chroot") ||
		strings.Contains(strings.ToLower(command), "mount ") ||
		strings.Contains(strings.ToLower(command), "umount ") ||
		strings.Contains(strings.ToLower(command), "fdisk") ||
		strings.Contains(strings.ToLower(command), "mkfs") ||
		strings.Contains(strings.ToLower(command), "format") {
		return fmt.Errorf("forbidden command detected")
	}

	// Vérifier les caractères dangereux
	dangerousChars := []string{"\x00", "\r", "\n"}
	for _, char := range dangerousChars {
		if strings.Contains(command, char) {
			return fmt.Errorf("dangerous character detected in command")
		}
	}

	return nil
}

// filterEnvironment retourne les variables d'environnement filtrées
func (e *Executor) filterEnvironment() []string {
	env := os.Environ()
	var filtered []string

	allowedSet := make(map[string]bool)
	for _, key := range e.allowedEnv {
		allowedSet[key] = true
	}

	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]

		// Garder les variables autorisées
		if allowedSet[key] {
			filtered = append(filtered, e)
			continue
		}

		// Filtrer les variables sensibles
		if strings.HasPrefix(key, "AWS_") ||
			strings.HasPrefix(key, "SSH_") ||
			strings.HasPrefix(key, "GIT_") ||
			strings.HasPrefix(key, "TOKEN") ||
			strings.HasPrefix(key, "SECRET") ||
			strings.HasPrefix(key, "PASSWORD") ||
			strings.HasPrefix(key, "API_KEY") ||
			strings.HasPrefix(key, "PRIVATE_KEY") {
			continue
		}

		// Garder quelques variables système utiles
		systemVars := []string{
			"SHELL", "PWD", "OLDPWD", "SHLVL", "HOSTNAME",
			"HOSTTYPE", "OSTYPE", "MACHTYPE", "LOGNAME",
		}
		for _, sysVar := range systemVars {
			if key == sysVar {
				filtered = append(filtered, e)
				break
			}
		}
	}

	return filtered
}

// limitedBuffer est un buffer avec une limite de taille
type limitedBuffer struct {
	bytes.Buffer
	limit      int
	truncated  bool
}

func (lb *limitedBuffer) Write(p []byte) (n int, err error) {
	if lb.Len()+len(p) > lb.limit {
		remaining := lb.limit - lb.Len()
		if remaining > 0 {
			lb.Buffer.Write(p[:remaining])
		}
		lb.truncated = true
		return len(p), nil
	}
	return lb.Buffer.Write(p)
}

// ExecuteSimple est une fonction utilitaire pour exécuter rapidement une commande
func ExecuteSimple(command string) *ExecutionResult {
	executor := NewExecutor()
	return executor.Execute(command)
}