# Configuration MCP Brainloop - HOROS

## Statut

✅ **Serveur MCP brainloop configuré** dans `/home/cl-ment/.config/claude/settings.json`
✅ **Package npm `cerebras-code-mcp` désinstallé**
✅ **Script Node.js `cerebras-claude-loop` désactivé** (renommé en `.DISABLED`)
✅ **GitHub MCP conservé** (builtin Claude Code)

**Architecture finale** :
- `brainloop` - Orchestrateur principal (12 actions Cerebras + bash + audit + readers)
- `monolabo-filesystem` - Accès filesystem externe
- `github` - MCP builtin Claude Code (recherche code, commits, PRs)

## Changements Effectués

### Avant (cerebras-loop)
- Package npm global `cerebras-code-mcp@1.3.2`
- Script Node.js `/home/cl-ment/cerebras-claude-loop/index.js`
- Configuré dans `.claude/projects/-workspace/` (config projet)
- 4 actions : clau_wr_cer_fs, clau_wr_cer_sql, cer_creative, cerebras_loop
- Pas de bash sécurisé
- Bug audit écrasait fichiers sources

**Problème découvert** : La config projet dans `.claude/projects` prenait le dessus sur la config globale, forçant cerebras-loop au lieu de brainloop.

### Après (brainloop)
- Serveur MCP custom HOROS `/workspace/projets/brainloop/brainloop`
- 12 actions (bash, audit, readers, generators)
- Permissions bash évolutives (ask → auto_approve)
- Audit sans écriture fichier
- Pattern 5-BDD HOROS complet

## Configuration Globale

**Fichier** : `/home/cl-ment/.config/claude/settings.json`

```json
{
  "mcpServers": {
    "brainloop": {
      "command": "/workspace/projets/brainloop/brainloop",
      "args": [],
      "cwd": "/workspace/projets/brainloop",
      "env": {
        "CEREBRAS_API_KEY": "csk-2jfxp5dk2eftvtkd54tjkxxvv3eyt45ymnth6vnrmym3v58n"
      }
    },
    "monolabo-filesystem": {
      "command": "node",
      "args": [
        "/media/cl-ment/STORAGE/monolabo/node_modules/@modelcontextprotocol/server-filesystem/dist/index.js",
        "/media/cl-ment/STORAGE/monolabo"
      ],
      "cwd": "/media/cl-ment/STORAGE/monolabo",
      "env": {
        "FILESYSTEM_ROOT": "/media/cl-ment/STORAGE/monolabo"
      }
    }
  }
}
```

## Permissions Projet

**Fichier** : `/workspace/.claude/settings.local.json`

```json
{
  "permissions": {
    "allow": [
      "mcp__brainloop__brainloop"
    ]
  }
}
```

Le tool `mcp__brainloop__brainloop` est pré-approuvé pour toutes ses actions.

## Actions Brainloop (12 total)

1. **execute_bash** - Exécution bash sécurisée avec policies évolutives
2. **audit_code** - Analyse critique code → retourne audit SANS écrire fichier
3. **read_code** - Digest architectural (structure, patterns) via LLM
4. **generate_file** - Génération code Cerebras → filesystem (ÉCRASE fichier !)
5. **generate_sql** - Génération SQL Cerebras → exécution SQLite
6. **explore** - Exploration créative patterns
7. **loop** - Boucle LLM orchestrée (propose, audit, refine, commit)
8. **read_sqlite** - Lecture bases SQLite avec cache
9. **read_markdown** - Lecture fichiers Markdown avec cache
10. **read_config** - Lecture fichiers config
11. **list_actions** - Liste actions disponibles + schémas
12. **get_stats** - Statistiques usage (tokens Cerebras, cache)

## Format Appel MCP

**Progressive Disclosure** : Un seul tool avec parameter `action` :

```json
{
  "tool": "mcp__brainloop__brainloop",
  "arguments": {
    "action": "nom_action",
    "params": {
      "param1": "value1"
    }
  }
}
```

## Exemples d'Utilisation

### Bash Sécurisé

```json
{
  "action": "execute_bash",
  "params": {
    "command": "ls /workspace/projets"
  }
}
```

**Permissions évolutives** :
- `ask` → validation requise (défaut)
- `auto_approve` → exécution automatique (après 20+ exec, 95%+ succès)

### Audit Code (NOUVEAU)

```json
{
  "action": "audit_code",
  "params": {
    "file_path": "/workspace/projets/HORUM/internal/http/handlers/auth_handler.go",
    "audit_prompt": "Analyse sécurité : injections, auth, sessions"
  }
}
```

**Retourne audit comme texte, ne modifie AUCUN fichier.**

### Read Code vs Audit Code

| Action | Input | Output | Écrit fichier | Use Case |
|--------|-------|--------|---------------|----------|
| `read_code` | Métadonnées extraites | Digest JSON architectural | ❌ | Comprendre structure |
| `audit_code` | Code complet | Analyse critique markdown | ❌ | Trouver problèmes |
| `generate_file` | Prompt génération | Code généré | ✅ | Créer fichier |

**Workflow recommandé** :
1. `read_code` → comprendre architecture
2. `audit_code` → identifier bugs/vulnérabilités
3. Corriger manuellement
4. `audit_code` → valider corrections

## Vérification

Après redémarrage Claude Code, taper `/mcp` devrait montrer :

```
Tools for brainloop (12 tools)
❯ 1. execute_bash
  2. audit_code
  3. read_code
  ...

Tools for github (XX tools)
❯ 1. get_file_contents
  2. list_commits
  ...

Tools for monolabo-filesystem (X tools)
...
```

**cerebras-loop NE DOIT PLUS apparaître.**

## Tests Recommandés

### Test 1 : Lister actions
```
Liste toutes les actions disponibles via MCP brainloop
```

### Test 2 : Execute bash
```
Via MCP brainloop, exécute : echo "Test MCP brainloop OK"
```

### Test 3 : Audit code
```
Via MCP brainloop, audite le fichier /workspace/projets/brainloop/main.go
```

## Observabilité

### Heartbeat
```bash
sqlite3 /workspace/projets/brainloop/brainloop.output.db \
  "SELECT * FROM heartbeat ORDER BY timestamp DESC LIMIT 1;"
```

Worker mort si pas de heartbeat depuis 30s.

### Métriques Cerebras
```bash
sqlite3 /workspace/projets/brainloop/brainloop.output.db \
  "SELECT metric_name, SUM(metric_value) as total
   FROM metrics
   WHERE metric_name LIKE 'cerebras_tokens_%'
   GROUP BY metric_name;"
```

### Registry Bash
```bash
sqlite3 /workspace/projets/brainloop/command_security.db \
  "SELECT command_text, execution_count, current_policy, risk_score
   FROM commands_registry
   ORDER BY execution_count DESC
   LIMIT 20;"
```

## Troubleshooting

### cerebras-loop apparaît encore dans /mcp

1. Vérifier désinstallation npm :
```bash
npm list -g cerebras-code-mcp
# Doit montrer (empty)
```

2. Redémarrer complètement Claude Code (pas juste fenêtre)

3. Vérifier config globale :
```bash
cat /home/cl-ment/.config/claude/settings.json | grep -A5 mcpServers
```

### brainloop n'apparaît pas dans /mcp

1. Vérifier binaire exécutable :
```bash
ls -lh /workspace/projets/brainloop/brainloop
# Doit être 15M et -rwxrwxr-x
```

2. Vérifier bases :
```bash
ls -1 /workspace/projets/brainloop/*.db | wc -l
# Doit être 5 (input, lifecycle, output, metadata, command_security)
```

3. Tester démarrage manuel :
```bash
cd /workspace/projets/brainloop
./brainloop --init-only
```

### Serveur démarre mais pas de heartbeat

```bash
sqlite3 /workspace/projets/brainloop/brainloop.output.db \
  "SELECT * FROM heartbeat ORDER BY timestamp DESC LIMIT 5;"
```

Si pas de heartbeat récent (< 30s), vérifier logs erreurs.

## Architecture Sécurité

### Bash Execution

**Sandboxing** :
- Timeout 120s max
- Output 10 KB max
- Working dir `/workspace` par défaut
- Commande 4096 chars max

**Environnement filtré** :
- Autorisés : PATH, HOME, USER, LANG, TERM
- Bloqués : AWS_*, SSH_*, TOKEN*, SECRET*, PASSWORD*, API_KEY*

**Patterns dangereux bloqués** :
- `rm -rf /`
- `chmod 777`
- `mkfs.*`
- `dd if=/dev/`
- Fork bombs
- `wget/curl | sh`

### Évolution Policies

Après 20 exécutions + 95% succès + risk_score < 0.7 → promotion `auto_approve`

```sql
-- Forcer override manuel si nécessaire
UPDATE commands_registry
SET user_override = 'always_allow',
    user_override_reason = 'Commande safe validée manuellement'
WHERE command_hash = '<hash>';
```

## Références

- Config globale : `/home/cl-ment/.config/claude/settings.json`
- Permissions projet : `/workspace/.claude/settings.local.json`
- Documentation brainloop : `/workspace/projets/brainloop/CLAUDE.md`
- Bash execution : `/workspace/projets/brainloop/BASH_EXECUTION.md`
- Audit code : `/workspace/projets/brainloop/AUDIT_CODE_ACTION.md`
- Setup MCP : `/workspace/projets/brainloop/MCP_SETUP.md`

## Prochaines Étapes

1. ✅ Package npm `cerebras-code-mcp` désinstallé
2. ✅ Script `/home/cl-ment/cerebras-claude-loop/` désactivé (renommé `.DISABLED`)
3. ✅ Config MCP globale à jour avec `brainloop`
4. ⏳ **Redémarrer Claude Code complètement**
5. ⏳ Taper `/mcp` → devrait montrer `brainloop` (12 actions), PAS `cerebras-loop`
6. ⏳ Tester `execute_bash` via MCP brainloop
7. ⏳ Tester `audit_code` sur un fichier Go

**Si cerebras-loop apparaît encore**, la config projet dans `.claude/projects/-workspace/` doit être réinitialisée via l'interface Claude Code.
