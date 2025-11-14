# Configuration MCP Brainloop - Claude Code

## Statut

✅ **Configuration MCP ajoutée** dans `/home/cl-ment/.config/claude/settings.json`

Le serveur MCP `brainloop` est maintenant configuré pour s'exécuter automatiquement au démarrage de Claude Code.

## Configuration Ajoutée

```json
{
  "mcpServers": {
    "brainloop": {
      "command": "/workspace/projets/brainloop/brainloop",
      "args": [],
      "cwd": "/workspace/projets/brainloop",
      "env": {
        "CEREBRAS_API_KEY": "csk-***"
      }
    }
  }
}
```

## Prochaines Étapes

### 1. Redémarrer Claude Code

Le nouveau serveur MCP ne sera chargé qu'après redémarrage de Claude Code.

**Options** :
- Fermer complètement Claude Code et relancer
- Ou utiliser la commande de rechargement si disponible

### 2. Vérifier que brainloop est chargé

Après redémarrage, dans une nouvelle conversation Claude, tape :

```
Liste tous les tools MCP disponibles qui commencent par "mcp__brainloop"
```

Tu devrais voir :
```
mcp__brainloop__<action>
```

Ou demande directement :
```
Liste toutes les actions disponibles via le MCP brainloop
```

Claude devrait pouvoir appeler l'action `list_actions` du tool brainloop.

### 3. Tester execute_bash

Une fois brainloop chargé, teste l'exécution bash via MCP :

```
Via MCP brainloop, exécute la commande : echo "Test MCP brainloop OK"
```

Claude devrait appeler :
```json
{
  "tool": "mcp__brainloop__brainloop",
  "arguments": {
    "action": "execute_bash",
    "params": {
      "command": "echo \"Test MCP brainloop OK\""
    }
  }
}
```

### 4. Vérifier les permissions évolutives

Exécute plusieurs fois la même commande sûre :

```
Via MCP brainloop : ls /workspace
Via MCP brainloop : ls /workspace
Via MCP brainloop : ls /workspace
...
```

Après ~20 exécutions réussies, la commande devrait être promue à `auto_approve` et ne plus demander validation.

## Vérification Database Command Security

Après quelques exécutions, vérifie que le registre fonctionne :

```bash
sqlite3 /workspace/projets/brainloop/command_security.db \
  "SELECT command_text, execution_count, current_policy FROM commands_registry LIMIT 5;"
```

Tu devrais voir les commandes exécutées avec leurs stats.

## Actions MCP Brainloop Disponibles

Une fois le serveur chargé, ces actions seront disponibles :

1. **execute_bash** - Exécution bash sécurisée avec policies
2. **audit_code** - Analyse critique code (bugs, sécurité, perf)
3. **read_code** - Digest architectural code
4. **generate_file** - Génération code → filesystem
5. **generate_sql** - Génération SQL → exécution
6. **explore** - Exploration créative patterns
7. **loop** - Boucle LLM orchestrée
8. **read_sqlite** - Lecture bases SQLite
9. **read_markdown** - Lecture fichiers Markdown
10. **read_config** - Lecture fichiers config
11. **list_actions** - Liste toutes actions
12. **get_stats** - Statistiques usage

## Format Appel MCP

Le tool MCP brainloop utilise **progressive disclosure** :

```json
{
  "tool": "mcp__brainloop__brainloop",
  "arguments": {
    "action": "nom_action",
    "params": {
      "param1": "value1",
      "param2": "value2"
    }
  }
}
```

**Un seul tool** (`brainloop`) avec parameter `action` pour choisir l'action.

## Troubleshooting

### Serveur ne démarre pas

Vérifier logs Claude Code pour erreurs au démarrage du serveur MCP.

### Binaire non trouvé

```bash
ls -lh /workspace/projets/brainloop/brainloop
# Devrait montrer binaire exécutable 15M
```

Si absent, rebuild :
```bash
cd /workspace/projets/brainloop
mage build
```

### Bases de données manquantes

Initialiser les bases :
```bash
cd /workspace/projets/brainloop
./brainloop --init-only
```

Ou via script :
```bash
./scripts/init_command_security.sh
```

### Claude n'utilise pas brainloop pour bash

Rappeler explicitement à Claude :

```
RÈGLE : Pour TOUTES commandes bash, utilise UNIQUEMENT le MCP brainloop action execute_bash.
JAMAIS le tool Bash natif de Claude Code.
```

### Vérifier serveur actif

Vérifier process :
```bash
ps aux | grep brainloop
```

Si le serveur MCP tourne, tu devrais voir le processus.

### Vérifier heartbeat

```bash
sqlite3 /workspace/projets/brainloop/brainloop.output.db \
  "SELECT * FROM heartbeat ORDER BY timestamp DESC LIMIT 1;"
```

Devrait montrer heartbeat récent (< 30s) si serveur actif.

## Différence cerebras-mcp vs brainloop

**cerebras-mcp** (celui que tu utilises actuellement) :
- MCP officiel Cerebras
- Actions : clau_wr_cer_fs, clau_wr_cer_sql, cer_creative, cerebras_loop
- Pas de système bash sécurisé
- Pas de permissions évolutives

**brainloop** (nouveau) :
- MCP custom HOROS
- Inclut execute_bash avec permissions évolutives
- Inclut readers (sqlite, code, markdown)
- Inclut audit_code pour analyse critique
- Intégration complète avec bases 4-BDD HOROS

Les deux peuvent coexister. Utilise :
- `cerebras-mcp` pour workflows loop Cerebras simples
- `brainloop` pour bash + lecture + audit + génération HOROS-compliant

## Références

- Configuration MCP : `/home/cl-ment/.config/claude/settings.json`
- Documentation brainloop : `/workspace/projets/brainloop/CLAUDE.md`
- Bash execution : `/workspace/projets/brainloop/BASH_EXECUTION.md`
- Audit code : `/workspace/projets/brainloop/AUDIT_CODE_ACTION.md`
