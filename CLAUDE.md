# CLAUDE.md - Brainloop

## Vue d'ensemble

Brainloop est un worker autonome HOROS effectuant des boucles LLM continues avec Cerebras, exposant ses capacités via MCP (Model Context Protocol) et gérant un système d'exécution bash sécurisée avec permissions évolutives.

## Architecture Pattern HOROS

### Pattern 5-BDD (Extension 4-BDD)

Brainloop utilise cinq bases SQLite indépendantes :

```
/workspace/projets/brainloop/
├── brainloop.input.db          # Sources externes (lecture seule)
├── brainloop.lifecycle.db      # État local, processed_log, config, cache
├── brainloop.output.db         # Résultats publiés, heartbeat, metrics
├── brainloop.metadata.db       # Métriques système, telemetry, events
└── command_security.db         # Registry commandes bash + policies
```

La cinquième base `command_security.db` isole la responsabilité sécurité bash pour éviter contentions sur lifecycle.db.

### Conformité HOROS

**Obligatoire** :
- ✅ modernc.org/sqlite (JAMAIS mattn/go-sqlite3)
- ✅ Table processed_log dans lifecycle.db
- ✅ SHA256 comme identité (commandes bash)
- ✅ Heartbeat 15 secondes dans output.db
- ✅ Graceful shutdown < 60 secondes
- ✅ Aucun ATTACH vers meta databases au runtime
- ✅ Communication SQLite uniquement (pas HTTP/RPC)

**Déclaration** :
- Projet enregistré dans `/workspace/HOROS.db`
- Type : worker
- État : en production
- Pattern : 5-BDD
- Temporalité : streaming 24/7

## Système Bash Execution Sécurisée

### Principe

Système de permissions évolutives évitant validations manuelles répétitives tout en maintenant sécurité maximale.

### Fonctionnement

**Registry commandes** (`command_security.db`) :
- Une ligne par commande unique (identifiée par hash SHA256)
- Accumulation statistiques : execution_count, success_count, avg_duration_ms
- Historique : 100 derniers timestamps (format texte semicolon-separated)
- Policies évolutives : auto_approve | ask | ask_warning
- User override toujours prioritaire

**Évolution automatique** :
- Après 20+ exécutions + 95%+ succès + risk_score < 0.7 → promotion auto_approve
- Détection pattern monitoring (50+ exec, intervalle < 5s) → duplicate_check désactivé
- Détection commande rare (intervalle > 1h) → duplicate_threshold porté à 30 secondes

**Détection duplication** :
- Si commande exécutée < threshold_ms depuis dernière fois → warning duplication
- Seuil configurable par commande
- Peut être désactivé pour patterns monitoring

**Patterns dangereux bloqués** :
- `rm -rf /` : suppression récursive racine
- `chmod 777` : permissions tout-permissif
- `mkfs.*` : formatage filesystem
- Fork bombs, wget/curl pipe shell, élévation privilèges
- Validation via regex dans security.go

### Workflow MCP

L'action `execute_bash` via MCP suit ce workflow :

1. Validation syntaxe commande (validator.go)
2. Calcul risk_score (0.0 à 1.0)
3. GetOrCreateCommand dans registry (hash SHA256)
4. GetPolicy (user_override prioritaire sinon current_policy)
5. Si auto_approve ou force_execute → exécution immédiate
6. Si ask/ask_warning → vérification duplication puis demande validation
7. Exécution sécurisée (timeout 120s, output limité 10KB, env filtré)
8. UpdateExecution (stats + timestamps)
9. CheckAutoEvolution (promotion éventuelle)

### API MCP

**Action execute_bash** :
```json
{
  "action": "execute_bash",
  "params": {
    "command": "ls /workspace",
    "force_execute": false  // optionnel, bypass policy
  }
}
```

**Réponses possibles** :

Exécution immédiate :
```json
{
  "success": true,
  "exit_code": 0,
  "stdout": "...",
  "stderr": "",
  "duration_ms": 12,
  "policy_used": "auto_approve"
}
```

Validation requise :
```json
{
  "status": "pending_validation",
  "command": "curl https://api.example.com",
  "policy": "ask",
  "risk_score": 0.6
}
```

Duplication détectée :
```json
{
  "status": "duplicate_warning",
  "command": "rm /tmp/file",
  "seconds_since_last": 1.2
}
```

## Structure Code

```
internal/
├── bash/                      # Système exécution sécurisée
│   ├── registry.go           # Gestion table commands_registry
│   ├── executor.go           # Exécution bash sandboxée
│   ├── policy.go             # Évolution automatique policies
│   ├── validator.go          # Validation syntaxe + injections
│   └── security.go           # Patterns dangereux + logging
│
├── cerebras/                  # Client Cerebras LLM
│   ├── client.go             # API HTTP Cerebras
│   ├── reader.go             # Lecture réponses streaming
│   └── generation.go         # Génération complétions
│
├── database/                  # Helpers 4-BDD
│   ├── database.go           # Orchestration init
│   ├── lifecycle.go          # Schéma lifecycle.db
│   ├── output.go             # Schéma output.db
│   └── metadata.go           # Schéma metadata.db
│
├── loop/                      # Boucle LLM principale
│   ├── manager.go            # Orchestration boucle
│   ├── session.go            # Gestion sessions
│   └── storage.go            # Persistance état
│
├── mcp/                       # Serveur MCP
│   ├── server.go             # Serveur JSON-RPC stdio
│   ├── tools.go              # Dispatcher actions
│   └── bash_handler.go       # Handler execute_bash
│
├── patterns/                  # Extracteurs patterns
│   ├── extractor.go          # Interface extraction
│   ├── go_patterns.go        # Patterns Go
│   └── sql_patterns.go       # Patterns SQL
│
└── readers/                   # Lecteurs sources
    ├── hub.go                # Hub central
    ├── code.go               # Lecteur code
    ├── sqlite.go             # Lecteur SQLite
    ├── markdown.go           # Lecteur Markdown
    └── config.go             # Lecteur config
```

## Commandes Développement

### Build et Test

```bash
cd /workspace/projets/brainloop

# Build binaire
mage build

# Tests unitaires
mage test

# Lint
mage lint

# Nettoyage
mage clean

# Initialiser bases uniquement
./brainloop --init-only

# Mode développement
mage dev
```

### Gestion Command Security

**Initialiser base sécurité** :
```bash
./scripts/init_command_security.sh
```

**Forcer réinitialisation** :
```bash
./scripts/init_command_security.sh --force
```

**Requêtes utiles** :

Commandes à risque :
```sql
SELECT command_text, risk_score, execution_count
FROM commands_registry
WHERE risk_score >= 0.7
ORDER BY risk_score DESC;
```

Statistiques commande :
```sql
SELECT command_text, execution_count, success_count,
       ROUND(CAST(success_count AS FLOAT) / execution_count * 100, 2) as success_rate,
       current_policy
FROM commands_registry
WHERE command_hash = '<hash>';
```

Override manuel :
```sql
UPDATE commands_registry
SET user_override = 'always_allow',
    user_override_reason = 'Validé manuellement'
WHERE command_hash = '<hash>';
```

## Progressive Disclosure MCP

Brainloop expose **un seul tool MCP** nommé `brainloop` avec parameter `action` enum de douze valeurs :

- `generate_file` : Génération code via Cerebras → filesystem
- `generate_sql` : Génération SQL via Cerebras → exécution SQLite
- `explore` : Exploration créative patterns code/données
- `loop` : Boucle LLM continue orchestrée
- `execute_bash` : Exécution bash sécurisée avec policies
- `audit_code` : **Analyse code fichier → retourne audit SANS écrire** (nouveau)
- `read_sqlite` : Lecture bases SQLite avec cache
- `read_markdown` : Lecture fichiers Markdown avec cache
- `read_code` : Lecture code source avec extraction patterns
- `read_config` : Lecture fichiers configuration
- `list_actions` : Liste actions disponibles + schémas
- `get_schema` : Schéma action spécifique
- `get_stats` : Statistiques usage (tokens Cerebras, cache)

Économie contexte : environ quatre-vingt-trois pour cent par rapport à exposition multiples tools séparés.

### Différence read_code vs audit_code (IMPORTANT)

Deux actions analysent le code via LLM mais avec objectifs différents :

**`read_code`** - Résumé architectural :
- Extraction métadonnées (fonctions, imports, types) par regex
- Génère digest structuré JSON : QU'EST-CE que fait le code, COMMENT est-il organisé
- Prompt : "Résume architecture, patterns, conventions"
- Use case : Comprendre rapidement structure projet/fichier

**`audit_code`** - Analyse critique :
- Lit code complet brut
- Génère analyse markdown : PROBLÈMES (bugs, sécurité, performance) + RECOMMANDATIONS
- Prompt : "Identifie vulnérabilités, inefficacités, violations best practices"
- Use case : Détecter risques, améliorer qualité

**Règle** : `read_code` pour comprendre, `audit_code` pour corriger.

### Action audit_code

**Utiliser audit_code au lieu de generate_file pour analyser du code existant.**

L'action `audit_code` lit le fichier, génère un audit détaillé via Cerebras, et **retourne l'audit comme texte** sans modifier aucun fichier.

```json
{
  "action": "audit_code",
  "params": {
    "file_path": "/workspace/projets/brainloop/main.go",
    "audit_prompt": "Analyse sécurité, performance, best practices"
  }
}
```

Réponse :
```json
{
  "success": true,
  "file_path": "/workspace/projets/brainloop/main.go",
  "audit": "# Code Audit Report\n\n## Summary\n...",
  "tokens": 4523,
  "message": "Code audit completed for main.go (no files modified)"
}
```

**ATTENTION** : Ne jamais utiliser `generate_file` avec `output_path` pointant vers un fichier existant si vous voulez juste un audit. Cela écraserait le fichier !

## Sécurité

### Sandboxing Bash

**Limitations** :
- Timeout : 120 secondes max
- Output : 10 KB max (stdout + stderr)
- Working dir : /workspace par défaut
- Commande : 4096 caractères max

**Filtrage environnement** :
- Variables autorisées : PATH, HOME, USER, LANG, TERM
- Variables bloquées : AWS_*, SSH_*, GIT_*, TOKEN*, SECRET*, PASSWORD*, API_KEY*, PRIVATE_KEY*

**Validation** :
- Pas de null bytes
- Pas d'injections détectées
- Patterns dangereux regex (security.go)
- Score risque calculé avant exécution

### Audit Trail

Toutes exécutions bash tracées dans command_security.db :
- Hash commande (SHA256)
- Timestamps cent dernières exécutions
- Exit codes, durées, taux succès
- Évolutions policies avec raisons
- User overrides avec justifications

## Observabilité

### Heartbeat

Heartbeat toutes les quinze secondes dans output.db :
```sql
SELECT worker_id, timestamp, status, sessions_active, cache_hit_rate
FROM heartbeat
ORDER BY timestamp DESC
LIMIT 1;
```

Worker considéré mort si pas de heartbeat pendant trente secondes.

### Métriques

Métriques publiées dans output.db :
- `cerebras_tokens_prompt` : Tokens prompt Cerebras
- `cerebras_tokens_completion` : Tokens completion Cerebras
- `cerebras_latency_ms` : Latence requêtes Cerebras
- `reader_cache_hit` : Hits cache lecteurs
- `reader_cache_miss` : Misses cache lecteurs

### Telemetry

Events dans metadata.db :
- Startup/shutdown worker
- Sessions audit/commit
- Erreurs critiques

## Dépendances

**Runtime** :
- Cerebras API (zai-glm-4.6)
- modernc.org/sqlite v1.28.0
- MCP protocol stdio

**Build** :
- Go 1.21+
- Mage (build automation)
- golangci-lint (optionnel)

## Troubleshooting

### Base command_security.db manquante

```bash
cd /workspace/projets/brainloop
./scripts/init_command_security.sh
```

### Worker ne démarre pas

Vérifier initialisation bases :
```bash
./brainloop --init-only
```

Vérifier logs :
```bash
tail -f logs/brainloop.log
```

### Commande bloquée indûment

Vérifier policy actuelle :
```sql
sqlite3 command_security.db \
  "SELECT command_text, current_policy, user_override, risk_score
   FROM commands_registry
   WHERE command_hash = '<hash>';"
```

Forcer override si légitime :
```sql
UPDATE commands_registry
SET user_override = 'always_allow'
WHERE command_hash = '<hash>';
```

### MCP ne répond pas

Vérifier serveur actif :
```bash
ps aux | grep brainloop
```

Vérifier heartbeat récent :
```bash
sqlite3 brainloop.output.db \
  "SELECT * FROM heartbeat ORDER BY timestamp DESC LIMIT 1;"
```

## Références

- Documentation complète : `/workspace/projets/brainloop/BASH_EXECUTION.md`
- Schéma sécurité : `command_security_schema.sql`
- Règles HOROS : `/workspace/docs/architecture/horos-rules.md`
- **Pattern ATTACH & 4-BDD** : `/workspace/docs/architecture/horos-patterns-4bdd-attach.md` ⭐
- Pattern 4-BDD : `/workspace/docs/architecture/15-dimensions.md`
- Worker lifecycle : `/workspace/docs/development/worker-lifecycle-pattern.md`
