# Système d'exécution bash sécurisée - Brainloop

## Vue d'ensemble

Brainloop dispose d'une capacité d'exécution sécurisée de commandes bash avec gestion évolutive des permissions basée sur l'historique d'utilisation et la détection des patterns suspects.

## Architecture

### Base de données dédiée

Une cinquième base `command_security.db` isole la responsabilité sécurité :

```
brainloop/
├── brainloop.input.db          # Sources externes
├── brainloop.lifecycle.db      # État local + référence légère commandes
├── brainloop.output.db         # Résultats + heartbeat
├── brainloop.metadata.db       # Métriques système
└── command_security.db         # ← NOUVEAU : registry commandes + policies
```

### Table principale : commands_registry

Chaque commande unique identifiée par hash SHA256 accumule :

- Statistiques : nombre exécutions, taux succès, durées min/moy/max
- Politique actuelle : auto_approve | ask | ask_warning
- Override utilisateur : always_allow | always_ask | never
- Détection duplication : seuil millisecondes + activation
- Classification : tags JSON + score risque zéro-un
- **Historique temporel : 100 derniers timestamps** (séparés par ;)

## Initialisation

```bash
cd /workspace/projets/brainloop
./scripts/init_command_security.sh
```

Pour réinitialiser :
```bash
./scripts/init_command_security.sh --force
```

## Utilisation via MCP

### Action execute_bash

```json
{
  "action": "execute_bash",
  "params": {
    "command": "ls /workspace"
  }
}
```

### Réponses possibles

**Exécution immédiate** (policy auto_approve) :
```json
{
  "success": true,
  "exit_code": 0,
  "stdout": "file1.txt\nfile2.txt",
  "stderr": "",
  "duration_ms": 12,
  "policy_used": "auto_approve"
}
```

**Validation requise** (policy ask) :
```json
{
  "status": "pending_validation",
  "command": "curl https://api.example.com",
  "policy": "ask",
  "risk_score": 0.6,
  "message": "Cette commande nécessite votre validation"
}
```

**Duplication détectée** :
```json
{
  "status": "duplicate_warning",
  "command": "rm /tmp/file",
  "seconds_since_last": 1.2,
  "threshold_ms": 10000,
  "message": "Cette commande a été exécutée il y a 1.2 secondes. Duplication volontaire ?"
}
```

### Forcer exécution après validation

```json
{
  "action": "execute_bash",
  "params": {
    "command": "curl https://api.example.com",
    "force_execute": true
  }
}
```

## Évolution automatique des politiques

### Promotion auto_approve

Conditions cumulatives :
- Vingt exécutions ou plus
- Taux succès ≥ quatre-vingt-quinze pour cent
- Policy actuelle = ask
- Risk score < zéro virgule sept
- Aucun pattern dangereux détecté

### Détection pattern monitoring

Si commande exécutée :
- Plus de cinquante fois
- Avec intervalle moyen < cinq secondes

→ Seuil duplication = zéro, vérification désactivée

### Commande rare

Si intervalle moyen > une heure :
→ Seuil duplication porté à trente secondes

## Sécurité

### Patterns dangereux bloqués

Jamais promus vers auto_approve :
- `rm -rf /` : suppression récursive racine
- `chmod 777` : permissions tout-permissif
- `mkfs.*` : formatage filesystem
- `dd if=/dev/` : accès direct devices
- `:(){:|:&};:` : fork bombs
- `wget.*|.*sh` : téléchargement pipe shell
- `curl.*|.*bash` : idem
- `eval $` : évaluation variables
- `sudo su`, `sudo -i` : élévation privilèges

### Validations avant exécution

1. Longueur max quatre mille quatre-vingt-seize caractères
2. Pas de null bytes
3. Pas d'injections détectées
4. Calcul score risque
5. Timeout cent vingt secondes
6. Capture stdout/stderr limitée à dix kilo-octets

### Filtrage environnement

Variables conservées : PATH, HOME, USER
Variables supprimées : AWS_*, SSH_*, toutes sensibles

## Gestion des données

### Historique timestamps

Fenêtre glissante cent dernières exécutions :
- Ajout nouveau timestamp en fin
- Suppression plus ancien si > cent
- Format texte : `1234567890;1234567895;1234567900`

Permet calcul :
- Intervalle moyen entre exécutions
- Détection rafales
- Détection patterns temporels

### Agrégation complète

Pas de table logs séparée. Toutes données agrégées dans registry unique. Économie espace massive versus approche ligne-par-exécution.

## Monitoring

### Requêtes statistiques

**Commandes à risque** :
```sql
SELECT command_text, risk_score, execution_count
FROM commands_registry
WHERE risk_score >= 0.7
ORDER BY risk_score DESC;
```

**Candidates promotion** :
```sql
SELECT * FROM promotion_candidates;
```

**Commandes fréquentes** :
```sql
SELECT command_text, execution_count,
       ROUND(CAST(success_count AS FLOAT) / execution_count * 100, 2) as success_rate
FROM commands_registry
WHERE execution_count > 10
ORDER BY execution_count DESC
LIMIT 20;
```

## Développement

### Tests sécurité

```bash
cd /workspace/projets/brainloop
go test ./internal/bash -v
```

Tests couverts :
- Injection commandes
- Escalade privilèges
- Patterns dangereux
- Validation syntaxe
- Calcul risk score
- Évolution automatique policies

### Build

```bash
cd /workspace/projets/brainloop
go build -o brainloop main.go
```

## Architecture code

```
internal/
├── bash/
│   ├── registry.go     # Gestion table commands_registry
│   ├── executor.go     # Exécution sécurisée bash
│   ├── policy.go       # Évolution automatique policies
│   ├── validator.go    # Validation syntaxe + injections
│   └── security.go     # Patterns dangereux + logging
└── mcp/
    ├── bash_handler.go # Orchestration workflow MCP
    ├── server.go       # Serveur MCP (modifié)
    └── tools.go        # Dispatcher actions (modifié)
```

## Troubleshooting

### Base command_security.db non trouvée

```bash
./scripts/init_command_security.sh
```

### Commande bloquée à tort

Vérifier policy actuelle :
```sql
sqlite3 command_security.db "SELECT command_text, current_policy, risk_score
FROM commands_registry WHERE command_hash = '<hash>';"
```

Forcer override :
```sql
UPDATE commands_registry
SET user_override = 'always_allow', user_override_reason = 'Validé manuellement'
WHERE command_hash = '<hash>';
```

### Reset complet

```bash
./scripts/init_command_security.sh --force
```

## Références

- Schéma complet : `command_security_schema.sql`
- Pattern HOROS 4-BDD : `/workspace/docs/architecture/horos-rules.md`
- Règles sécurité Cerebras : voir analyse dans historique conception
