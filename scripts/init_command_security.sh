#!/bin/bash
# Script d'initialisation de command_security.db pour brainloop

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
SCHEMA_FILE="$PROJECT_DIR/command_security_schema.sql"
DB_FILE="$PROJECT_DIR/command_security.db"

echo "Initialisation de command_security.db..."

# Vérifier que le schéma existe
if [ ! -f "$SCHEMA_FILE" ]; then
    echo "Erreur : fichier schéma non trouvé : $SCHEMA_FILE"
    exit 1
fi

# Supprimer DB existante si demandé
if [ "$1" = "--force" ]; then
    echo "Suppression de la base existante..."
    rm -f "$DB_FILE"
fi

# Créer la base de données
echo "Création de la base de données..."
sqlite3 "$DB_FILE" < "$SCHEMA_FILE"

# Vérifier que les tables ont été créées
TABLE_COUNT=$(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='commands_registry';")

if [ "$TABLE_COUNT" -eq "1" ]; then
    echo "✓ Table commands_registry créée avec succès"
else
    echo "✗ Erreur : table commands_registry non créée"
    exit 1
fi

# Afficher les statistiques
echo ""
echo "Base command_security.db initialisée avec succès :"
echo "  Emplacement : $DB_FILE"
echo "  Taille : $(du -h "$DB_FILE" | cut -f1)"
echo ""
sqlite3 "$DB_FILE" "SELECT COUNT(*) as tables FROM sqlite_master WHERE type='table';" | while read count; do
    echo "  Tables créées : $count"
done

echo ""
echo "Pour vérifier le schéma :"
echo "  sqlite3 $DB_FILE '.schema'"
