#!/bin/bash
# Wrapper pour brainloop - garantit l'exécution depuis le bon répertoire
# Respect du pattern HOROS : chaque projet dans son dossier

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Exporter les variables d'environnement si nécessaire
if [ -z "$CEREBRAS_API_KEY" ]; then
    # Récupérer la clé depuis la base locale si non fournie
    export CEREBRAS_API_KEY=$(sqlite3 brainloop.metadata.db "SELECT secret_value FROM secrets WHERE secret_name = 'CEREBRAS_API_KEY'" 2>/dev/null || echo "")
fi

exec ./brainloop "$@"