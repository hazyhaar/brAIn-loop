# Action audit_code - Brainloop

## Problème Résolu

Avant l'ajout de `audit_code`, demander un audit de code via `generate_file` écrasait le fichier source avec l'audit généré par Cerebras.

**Mauvais workflow (AVANT)** :
```json
{
  "action": "generate_file",
  "params": {
    "verified_prompt": "Fait un audit de sécurité de ce fichier",
    "output_path": "main.go",  // ⚠️ ÉCRASE main.go !
    "code_type": "go"
  }
}
```

Résultat : `main.go` original perdu, remplacé par l'audit généré.

## Solution

**Bon workflow (APRÈS)** :
```json
{
  "action": "audit_code",
  "params": {
    "file_path": "main.go",
    "audit_prompt": "Analyse sécurité, performance, best practices"
  }
}
```

Résultat : `main.go` intact, audit retourné comme texte dans la réponse JSON.

## Fonctionnement

L'action `audit_code` effectue :

1. **Lecture fichier** : Charge le contenu complet du fichier spécifié
2. **Construction prompt** : Combine audit_prompt + contenu fichier en prompt Cerebras
3. **Génération audit** : Appelle Cerebras (température 0.3 pour cohérence analytique)
4. **Retour texte** : Retourne l'audit dans le champ `audit` de la réponse
5. **Aucune écriture** : Aucun fichier n'est modifié

## API

### Paramètres

**file_path** (string, requis) :
- Chemin absolu ou relatif vers le fichier à auditer
- Exemple : `/workspace/projets/brainloop/main.go`

**audit_prompt** (string, optionnel) :
- Instructions personnalisées pour l'audit
- Par défaut : "Analyze this code for: bugs, security issues, performance problems, code quality, best practices violations, and potential improvements. Provide detailed feedback."
- Exemple : "Focus sur les vulnérabilités SQL injection et XSS"

### Réponse

```json
{
  "success": true,
  "file_path": "/workspace/projets/brainloop/main.go",
  "audit": "# Code Audit Report\n\n## Summary\nThis Go application...\n\n## Security Issues\n1. Potential SQL injection...\n\n## Performance\n...",
  "tokens": 4523,
  "message": "Code audit completed for /workspace/projets/brainloop/main.go (no files modified)"
}
```

Champs :
- **success** : Booléen succès opération
- **file_path** : Chemin fichier audité
- **audit** : Rapport audit complet (format Markdown)
- **tokens** : Total tokens consommés (prompt + completion)
- **message** : Message confirmation

## Exemples d'Utilisation

### Audit sécurité basique

```json
{
  "action": "audit_code",
  "params": {
    "file_path": "internal/bash/executor.go"
  }
}
```

Utilise le prompt par défaut (analyse complète).

### Audit sécurité ciblé

```json
{
  "action": "audit_code",
  "params": {
    "file_path": "internal/bash/executor.go",
    "audit_prompt": "Vérifie uniquement les injections de commandes, l'échappement de variables, et les timeouts. Ignore le reste."
  }
}
```

### Audit performance

```json
{
  "action": "audit_code",
  "params": {
    "file_path": "internal/loop/manager.go",
    "audit_prompt": "Analyse uniquement les problèmes de performance : allocations mémoire inutiles, boucles inefficaces, goroutine leaks, deadlocks potentiels."
  }
}
```

### Audit conformité HOROS

```json
{
  "action": "audit_code",
  "params": {
    "file_path": "internal/database/lifecycle.go",
    "audit_prompt": "Vérifie conformité HOROS : modernc.org/sqlite utilisé, pas de mattn/go-sqlite3, SHA256 pour identités, processed_log pour idempotence, pas d'ATTACH vers meta databases."
  }
}
```

## Workflow Recommandé

### Audit avant commit

1. Développeur modifie `internal/bash/policy.go`
2. Lance `audit_code` avec prompt sécurité
3. Cerebras retourne audit avec issues détectées
4. Développeur corrige les issues
5. Relance `audit_code` pour confirmer corrections
6. Commit une fois audit propre

### Audit code existant

1. Identifier fichiers critiques (handlers, executors, validators)
2. Pour chaque fichier :
   - `audit_code` avec prompt sécurité
   - Noter issues dans backlog
   - Prioriser par gravité
3. Créer tickets correctifs
4. Vérifier fixes avec `audit_code`

### Audit batch

Pour auditer plusieurs fichiers, utiliser `execute_bash` avec script :

```bash
for file in internal/bash/*.go; do
  echo "Auditing $file..."
  # Call MCP audit_code via script/CLI wrapper
done
```

## Comparaison Actions Analyse Code

| Action | Input LLM | Output LLM | Écrit fichier | Objectif |
|--------|-----------|------------|---------------|----------|
| `read_code` | Métadonnées extraites (fonctions, types) | Digest JSON architectural | ❌ | Comprendre QUOI/COMMENT |
| `audit_code` | Code complet brut | Analyse critique markdown | ❌ | Trouver PROBLÈMES/RISQUES |
| `generate_file` | Prompt génération | Code généré | ✅ | Créer nouveau fichier |
| `explore` | Description créative | Code exploratoire | ❌ | Prototypage rapide |

### read_code vs audit_code

**read_code** - Résumé architectural :
- Extraction statique par regex (packages, imports, fonctions, types, constantes)
- Envoie métadonnées à Cerebras avec prompt "Résume structure"
- Retourne digest JSON : architecture, patterns, conventions de nommage
- Tokens : ~500-2000 (métadonnées compactes)
- Use case : "Comment ce fichier est-il organisé ?"

**audit_code** - Analyse critique :
- Lit code complet (pas d'extraction préalable)
- Envoie code brut à Cerebras avec prompt "Analyse bugs/sécurité/performance"
- Retourne audit markdown : problèmes identifiés + recommandations
- Tokens : ~3000-15000 (code complet + analyse détaillée)
- Use case : "Quels problèmes ce code a-t-il ?"

**Workflow combiné recommandé** :
1. `read_code` d'abord → comprendre architecture rapidement
2. `audit_code` ensuite → identifier problèmes spécifiques
3. Corriger issues détectées
4. Re-`audit_code` pour valider corrections

## Idempotence et Tracking

Chaque appel `audit_code` est tracé dans `lifecycle.db` via `processed_log` :

```sql
SELECT hash, operation, timestamp, result_json
FROM processed_log
WHERE operation = 'audit_code'
ORDER BY timestamp DESC
LIMIT 10;
```

Le hash est calculé sur : `file_path + audit_prompt + audit_content`

Cela permet de :
- Éviter duplications audits identiques
- Tracker historique audits par fichier
- Mesurer tokens consommés par audit

## Métriques Cerebras

Les tokens consommés par `audit_code` sont enregistrés dans `output.db` :

```sql
SELECT metric_name, SUM(metric_value) as total
FROM metrics
WHERE metric_name LIKE 'cerebras_tokens_%'
  AND timestamp > strftime('%s', 'now', '-1 day')
GROUP BY metric_name;
```

Métriques :
- `cerebras_tokens_prompt` : Tokens du code source + prompt
- `cerebras_tokens_completion` : Tokens de l'audit généré
- `cerebras_latency_ms` : Durée génération

## Limites

### Taille fichiers

Cerebras llama-3.3-70b a contexte 128K tokens. Un fichier Go typique consomme :
- 100 lignes ≈ 400 tokens
- 1000 lignes ≈ 4000 tokens
- 5000 lignes ≈ 20000 tokens

Limite pratique : fichiers < 10000 lignes (≈ 40K tokens) pour laisser espace au prompt et à la réponse.

Pour fichiers plus grands, utiliser :
1. `read_code` avec extraction patterns ciblée
2. Découpage manuel en sections
3. Audit par fonction/classe plutôt que fichier entier

### Langages supportés

L'audit fonctionne sur n'importe quel langage texte :
- Go, Python, JavaScript, TypeScript, Rust, C, C++
- SQL, HTML, CSS
- YAML, JSON, TOML
- Bash, Shell scripts
- Markdown, Documentation

Cerebras comprend la syntaxe de tous langages courants.

### Faux positifs

L'audit LLM peut générer faux positifs. Toujours :
- Valider manuellement les issues critiques
- Cross-checker avec linters statiques (golangci-lint, etc.)
- Prioriser selon gravité et probabilité

## Troubleshooting

### Erreur "failed to read file"

Vérifier :
- Chemin fichier correct (absolu ou relatif depuis working dir)
- Permissions lecture
- Fichier existe

### Audit incomplet / tronqué

Si fichier trop grand :
- Réduire scope via `audit_prompt` ciblé
- Découper en sections
- Utiliser `read_code` pour extraction patterns d'abord

### Tokens élevés

Optimiser consommation :
- Prompts spécifiques au lieu de "analyse tout"
- Auditer fichiers modifiés uniquement, pas tout le projet
- Utiliser cache : même fichier + même prompt = même hash processed_log

### Audit peu pertinent

Améliorer `audit_prompt` :
- Spécifier critères précis
- Donner exemples de ce qu'on cherche
- Exclure explicitement ce qui n'intéresse pas

Exemple mauvais prompt :
```
"Audit ce fichier"
```

Exemple bon prompt :
```
"Vérifie ce validateur Go pour :
1. Injections SQL/commandes (priorité haute)
2. Buffer overflows dans parsing
3. Race conditions sur variables partagées
4. Timeout manquants sur opérations I/O
Ignore styling et conventions nommage."
```

## Références

- Documentation MCP brainloop : `CLAUDE.md`
- Actions MCP : `internal/mcp/tools.go`
- Client Cerebras : `internal/cerebras/client.go`
- Pattern HOROS : `/workspace/docs/architecture/horos-rules.md`
