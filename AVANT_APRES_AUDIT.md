# Avant / Après : Correction Bug Audit Code

## Problème Identifié

Quand on demandait un audit sur un fichier `main.go`, le MCP Cerebras **écrasait le fichier original** avec l'audit généré.

## Scénario Problématique (AVANT)

### Requête utilisateur

"Fait un audit de sécurité de `/workspace/projets/brainloop/main.go`"

### Ce qui se passait

L'utilisateur utilisait (ou Claude utilisait automatiquement) l'action `generate_file` :

```json
{
  "action": "generate_file",
  "params": {
    "verified_prompt": "Analyse ce fichier main.go pour sécurité, bugs, performance",
    "output_path": "/workspace/projets/brainloop/main.go",
    "code_type": "go"
  }
}
```

### Résultat désastreux

1. Cerebras génère un audit textuel (markdown ou commentaires Go)
2. Brainloop **écrit ce texte dans `/workspace/projets/brainloop/main.go`**
3. Le code Go original est **complètement écrasé**
4. Le fichier main.go contient maintenant un document markdown d'audit
5. L'application ne compile plus
6. Perte du code si pas de git commit récent

### Exemple concret

**main.go AVANT (code fonctionnel)** :
```go
package main

import (
    "fmt"
    "log"
)

func main() {
    log.Println("Worker starting...")
    // ... code applicatif
}
```

**main.go APRÈS écrasement (CASSÉ)** :
```markdown
# Audit de Sécurité - main.go

## Vue d'ensemble
Ce fichier main.go implémente un worker HOROS...

## Problèmes Identifiés

### Sécurité
1. Pas de validation des variables d'environnement
2. Erreurs non loggées correctement
3. ...

## Recommandations
...
```

**Résultat** : `go build` échoue, application cassée.

## Solution Implémentée (APRÈS)

### Nouvelle action dédiée

Ajout de l'action `audit_code` qui :
- ✅ Lit le fichier source
- ✅ Génère l'audit via Cerebras
- ✅ **Retourne l'audit comme texte dans la réponse JSON**
- ✅ **N'écrit dans AUCUN fichier**

### Requête utilisateur (identique)

"Fait un audit de sécurité de `/workspace/projets/brainloop/main.go`"

### Ce qui se passe maintenant

Claude utilise automatiquement `audit_code` :

```json
{
  "action": "audit_code",
  "params": {
    "file_path": "/workspace/projets/brainloop/main.go",
    "audit_prompt": "Analyse sécurité, bugs, performance, conformité HOROS"
  }
}
```

### Résultat sain

1. Brainloop lit `/workspace/projets/brainloop/main.go`
2. Construit prompt : audit_prompt + contenu fichier
3. Cerebras génère l'audit (température 0.3)
4. Brainloop **retourne l'audit dans le champ `audit` de la réponse**
5. **Le fichier main.go reste INTACT**
6. L'utilisateur reçoit l'audit comme texte
7. Aucun fichier n'a été modifié

### Réponse JSON

```json
{
  "success": true,
  "file_path": "/workspace/projets/brainloop/main.go",
  "audit": "# Code Audit Report\n\n## Summary\nThis Go application implements a HOROS worker...\n\n## Security Issues\n1. Environment variable validation missing in lines 45-52\n2. Error logging incomplete in shutdown sequence\n...\n\n## Performance\n1. Heartbeat goroutine could use buffered channel (line 67)\n...\n\n## Recommendations\n...",
  "tokens": 4523,
  "message": "Code audit completed for /workspace/projets/brainloop/main.go (no files modified)"
}
```

**Résultat** : Code intact, audit disponible, application fonctionne toujours.

## Comparaison Directe

| Critère | AVANT (generate_file) | APRÈS (audit_code) |
|---------|----------------------|-------------------|
| Fichier source | ❌ ÉCRASÉ | ✅ INTACT |
| Audit généré | ✅ Oui (mais dans fichier) | ✅ Oui (dans réponse JSON) |
| Compilation | ❌ Cassée | ✅ Fonctionne |
| Perte de code | ⚠️ Possible si pas git | ✅ Impossible |
| Utilisabilité | ❌ Dangereux | ✅ Sûr |
| Besoin rollback | ✅ Obligatoire (git) | ❌ Aucun |

## Impact Utilisateur

### Avant

1. Demande audit → fichier écrasé
2. Panique en voyant code disparu
3. `git checkout main.go` pour récupérer
4. Perte audit généré (était dans fichier écrasé)
5. Frustration

### Après

1. Demande audit → reçoit audit comme texte
2. Lit audit dans réponse
3. Implémente corrections suggérées
4. Code jamais touché, aucun risque
5. Workflow fluide

## Commandes Impactées

### Actions qui ÉCRIVENT (DANGER si utilisées pour audit)

- ❌ `generate_file` : ÉCRIT dans output_path
- ❌ `generate_sql` : EXÉCUTE SQL (modifie DB)
- ❌ `loop` mode `commit` : ÉCRIT dans target

**Ne jamais utiliser ces actions pour auditer du code existant.**

### Actions qui LISENT uniquement (SAFE)

- ✅ `audit_code` : LIT fichier, RETOURNE audit
- ✅ `read_code` : LIT fichier, RETOURNE contenu + patterns
- ✅ `read_markdown` : LIT fichier, RETOURNE digest
- ✅ `explore` : GÉNÈRE code, RETOURNE sans écrire

### Action spéciale

- ⚠️ `execute_bash` : EXÉCUTE commande (peut modifier si commande écrit)

## Migration

### Si vous aviez des workflows utilisant generate_file pour auditer

**Ancien workflow** :
```json
{
  "action": "generate_file",
  "params": {
    "verified_prompt": "Audit security of this handler",
    "output_path": "internal/handlers/auth.go",
    "code_type": "go"
  }
}
```

**Nouveau workflow** :
```json
{
  "action": "audit_code",
  "params": {
    "file_path": "internal/handlers/auth.go",
    "audit_prompt": "Audit security: authentication bypass, injection, session fixation"
  }
}
```

**Bénéfices** :
- Fichier source jamais touché
- Audit dans réponse, pas dans fichier
- Aucun risque de casser le build
- Workflow CI/CD compatible

## Tests

### Test régression

Fichier de test fourni : `test_audit_code.json`

```bash
cd /workspace/projets/brainloop

# Vérifier que main.go existe et est valide
go build -o /tmp/test_build main.go
md5sum main.go > /tmp/main_go_before.md5

# Exécuter audit via MCP (à adapter selon votre client MCP)
cat test_audit_code.json | ./brainloop

# Vérifier que main.go n'a PAS changé
md5sum main.go > /tmp/main_go_after.md5
diff /tmp/main_go_before.md5 /tmp/main_go_after.md5
# Devrait retourner : aucune différence

# Vérifier que build fonctionne toujours
go build -o /tmp/test_build2 main.go
```

Si le test passe : ✅ `audit_code` n'écrase pas les fichiers

## Documentation

- Guide complet : `AUDIT_CODE_ACTION.md`
- Configuration MCP : `.claude.json`
- Documentation projet : `CLAUDE.md`

## Leçons Apprises

1. **Nommer les actions explicitement** : `generate_file` implique écriture, `audit_code` implique lecture seule
2. **Séparer lecture et écriture** : Ne jamais mélanger analyse et modification dans même action
3. **Progressive disclosure prudente** : Un seul tool MCP ne doit pas masquer les risques d'écriture
4. **Documentation claire** : Indiquer explicitement "ÉCRIT FICHIER" vs "RETOURNE RÉSULTAT"
5. **Tests de régression** : Tester que les actions read-only n'écrivent jamais

## Conclusion

Le bug critique d'écrasement de fichiers lors d'audits est résolu par l'ajout de l'action dédiée `audit_code`.

**Règle d'or** : Toujours utiliser `audit_code` pour analyser du code existant, jamais `generate_file`.
