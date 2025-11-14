# Contributing to Brainloop

Merci de contribuer à Brainloop ! Ce guide vous aidera à démarrer.

## 🚀 Quick Start

### Prérequis

- Go 1.21+
- Git
- Make ou Mage (optionnel)

### Installation Développement

```bash
# Clone repository
git clone https://github.com/YOUR-ORG/brainloop.git
cd brainloop

# Install dependencies
go mod download

# Run tests
go test -v ./...

# Build
go build -o brainloop main.go
```

## 📝 Development Workflow

### 1. Créer une Branche

```bash
git checkout -b feature/ma-fonctionnalite
# ou
git checkout -b fix/mon-bug
```

**Convention nommage** :
- `feature/` : Nouvelle fonctionnalité
- `fix/` : Correction bug
- `refactor/` : Refactoring
- `docs/` : Documentation
- `test/` : Ajout/amélioration tests

### 2. Développer

**Code Style** :
- Suivre [Effective Go](https://go.dev/doc/effective_go)
- Utiliser `gofmt` et `goimports`
- Linter : `golangci-lint run`

**Tests** :
```bash
# Run all tests
go test -v ./...

# Run with race detector
go test -v -race ./...

# Run with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**Benchmarks** :
```bash
go test -bench=. -benchmem ./...
```

### 3. Commit

**Convention Commits** :
```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types** :
- `feat` : Nouvelle fonctionnalité
- `fix` : Correction bug
- `refactor` : Refactoring
- `test` : Ajout tests
- `docs` : Documentation
- `chore` : Maintenance

**Example** :
```bash
git commit -m "feat(bash): add rate limiting for command execution

Implement token bucket algorithm for bash command rate limiting
to prevent abuse. Default limit: 100 commands/minute.

Closes #123"
```

### 4. Push & Pull Request

```bash
git push origin feature/ma-fonctionnalite
```

Créer une Pull Request sur GitHub avec :
- **Titre clair** : résumé en une ligne
- **Description** : contexte, changements, tests
- **Références** : issues résolues (#123)
- **Screenshots** : si applicable

## 🧪 Tests

### Test Coverage Target

- **Minimum** : 30%
- **Target** : 60%
- **Goal** : 80%

### Types de Tests

**1. Tests Unitaires** :
```go
// internal/bash/security_test.go
func TestDangerousPatterns(t *testing.T) {
    tests := []struct {
        command string
        dangerous bool
    }{
        {"rm -rf /", true},
        {"ls -la", false},
    }

    for _, tt := range tests {
        t.Run(tt.command, func(t *testing.T) {
            result := MatchesDangerousPattern(tt.command)
            if result != tt.dangerous {
                t.Errorf("expected %v, got %v", tt.dangerous, result)
            }
        })
    }
}
```

**2. Tests Intégration** :
```go
// tests/integration_test.go
func TestMCPEndToEnd(t *testing.T) {
    // Setup MCP server
    // Send request
    // Verify response
    // Cleanup
}
```

**3. Tests Table-Driven** :
```go
func TestValidator(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        // test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

## 🔒 Sécurité

### Patterns Dangereux

Ajouter patterns dans `internal/bash/security.go` :
```go
var DangerousPatterns = []string{
    `(?i)rm\s+-rf\s+/`,      // Delete root
    `(?i)chmod\s+777`,       // Insecure permissions
    // Add new patterns here
}
```

### Tests Sécurité

Chaque nouveau pattern dangereux **DOIT** avoir un test :
```go
func TestNewDangerousPattern(t *testing.T) {
    matched, _ := MatchesDangerousPattern("dangerous command")
    if !matched {
        t.Error("Pattern not detected")
    }
}
```

## 📚 Documentation

### Code Documentation

```go
// CalculateRiskScore calculates a risk score (0-100) for a bash command.
//
// The score is based on:
// - Presence of destructive operations (rm, dd, etc.)
// - Use of sudo/privilege escalation
// - Pipe redirection to shell
// - File operations on system directories
//
// Returns:
//   - 0-20: Safe (ls, echo, etc.)
//   - 21-50: Low risk (cat, grep with filters)
//   - 51-80: Medium risk (rm, chmod on user files)
//   - 81-100: High risk (rm -rf /, sudo operations)
func CalculateRiskScore(command string) int {
    // implementation
}
```

### README Updates

Mettre à jour README.md si :
- Nouvelle fonctionnalité exposée via MCP
- Changement API/configuration
- Nouvelle dépendance

## 🐛 Bug Reports

### Template Issue

```markdown
**Description**
Brève description du bug

**To Reproduce**
1. Step 1
2. Step 2
3. See error

**Expected behavior**
Comportement attendu

**Actual behavior**
Comportement observé

**Environment**
- OS: [Linux/macOS/Windows]
- Go version: [1.21/1.22]
- Brainloop version: [commit hash]

**Logs**
```
error logs here
```

**Additional context**
Screenshots, configuration, etc.
```

## ✅ Pull Request Checklist

Avant de soumettre votre PR, vérifier :

- [ ] Code compile sans erreurs
- [ ] Tests passent : `go test ./...`
- [ ] Linter passe : `golangci-lint run`
- [ ] Coverage ≥ 30% (ou inchangée)
- [ ] Documentation mise à jour
- [ ] Commits suivent convention
- [ ] Branche à jour avec `main`
- [ ] Pas de secrets/credentials dans code
- [ ] Tests sécurité passent (si applicable)

## 🔄 Code Review Process

### Review Criteria

1. **Correctness** : Code fonctionne comme attendu
2. **Tests** : Coverage suffisante + tests pertinents
3. **Security** : Pas de vulnérabilités
4. **Performance** : Pas de régressions
5. **Maintainability** : Code lisible + bien documenté
6. **HOROS Compliance** : Respect patterns HOROS v2

### Reviewers

- **Security changes** : 2 reviewers minimum
- **Bash execution** : 1 reviewer minimum + tests sécurité
- **Other changes** : 1 reviewer minimum

### Merge Conditions

- ✅ All CI checks pass
- ✅ Approved by required reviewers
- ✅ No unresolved comments
- ✅ Branch up to date with `main`

## 📊 CI/CD

### GitHub Actions

**CI Workflow** (`.github/workflows/ci.yml`) :
- Tests Go 1.21 et 1.22
- Coverage report
- Linting (golangci-lint)
- Security scan (gosec)
- Build binary

**CodeQL** (`.github/workflows/codeql.yml`) :
- Static analysis
- Vulnerability detection
- Weekly scheduled scans

**Release** (`.github/workflows/release.yml`) :
- Multi-platform builds
- Checksums generation
- GitHub release creation

### Local CI Simulation

```bash
# Run same checks as CI locally
make ci

# Or manually:
go test -v -race ./...
golangci-lint run
go build -o brainloop main.go
```

## 🏗️ Architecture

### HOROS v2 Compliance

Brainloop suit le pattern **5-BDD** (extension 4-BDD) :

```
brainloop.input.db       - Sources externes
brainloop.lifecycle.db   - État local + processed_log
brainloop.output.db      - Résultats + heartbeat
brainloop.metadata.db    - Métriques + secrets
command_security.db      - Registry bash + policies
```

**Règles HOROS** :
- ✅ `modernc.org/sqlite` (jamais mattn/go-sqlite3)
- ✅ `processed_log` obligatoire (idempotence)
- ✅ SHA256 comme identité
- ✅ Heartbeat 15s
- ✅ Graceful shutdown <60s
- ✅ Pas d'ATTACH meta au runtime

### Module Structure

```
internal/
├── bash/        # Sécurité bash + exécution
├── mcp/         # Serveur MCP + tools
├── cerebras/    # Client API Cerebras
├── readers/     # Lecteurs intelligents
├── patterns/    # Extraction patterns Go/SQL
├── loop/        # Session management
└── database/    # Initialisation 4-BDD
```

## 💡 Tips

### Debug

```bash
# Enable debug logs
export DEBUG=1
./brainloop

# Profile CPU
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof

# Profile memory
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof
```

### Common Issues

**Issue** : Tests failing with "database locked"
**Solution** : Check no brainloop instance running, remove `*.db-wal` files

**Issue** : Linter errors about imports
**Solution** : Run `goimports -w .`

**Issue** : Coverage too low
**Solution** : Add table-driven tests for edge cases

## 📞 Contact

- **Issues** : GitHub Issues
- **Discussions** : GitHub Discussions
- **Security** : security@example.com

## 📜 License

By contributing, you agree that your contributions will be licensed under the same license as the project (MIT).

---

**Merci de contribuer à Brainloop !** 🎉
