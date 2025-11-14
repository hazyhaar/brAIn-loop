-- ============================================================================
-- COMMAND SECURITY SCHEMA
-- Registre des commandes bash avec policies évolutives et détection duplication
-- ============================================================================

-- Table principale : registry des commandes uniques
CREATE TABLE IF NOT EXISTS commands_registry (
    -- Identité
    command_hash TEXT PRIMARY KEY,
    command_text TEXT NOT NULL,

    -- Statistiques accumulées
    first_seen INTEGER NOT NULL,
    last_executed INTEGER,
    execution_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,

    -- Métriques performance
    avg_duration_ms INTEGER,
    min_duration_ms INTEGER,
    max_duration_ms INTEGER,
    total_duration_ms INTEGER DEFAULT 0,

    -- Policy dynamique
    current_policy TEXT DEFAULT 'ask' CHECK(current_policy IN ('auto_approve','ask','ask_warning')),
    policy_reason TEXT,
    policy_last_updated INTEGER,
    promoted_at INTEGER,

    -- Override utilisateur
    user_override TEXT CHECK(user_override IS NULL OR user_override IN ('always_allow','always_ask','never')),
    user_override_reason TEXT,
    user_override_at INTEGER,

    -- Détection duplication
    duplicate_threshold_ms INTEGER DEFAULT 2000,
    duplicate_check_enabled INTEGER DEFAULT 1,

    -- Classification
    tags TEXT,  -- JSON array
    risk_score REAL DEFAULT 0.5 CHECK(risk_score >= 0 AND risk_score <= 1),

    -- Historique temporel : 100 derniers timestamps séparés par ;
    last_100_timestamps TEXT,

    -- Métadonnées analyse
    typical_exit_codes TEXT,  -- JSON: {"0": 95, "1": 5}
    common_errors TEXT,       -- JSON array

    -- Timestamps
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Index performance critiques
CREATE INDEX IF NOT EXISTS idx_commands_policy ON commands_registry(current_policy);
CREATE INDEX IF NOT EXISTS idx_commands_risk_score ON commands_registry(risk_score);
CREATE INDEX IF NOT EXISTS idx_commands_exec_count ON commands_registry(execution_count DESC);
CREATE INDEX IF NOT EXISTS idx_commands_last_exec ON commands_registry(last_executed DESC);
CREATE INDEX IF NOT EXISTS idx_commands_user_override ON commands_registry(user_override) WHERE user_override IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_commands_updated ON commands_registry(updated_at DESC);

-- Vue statistiques temps réel
CREATE VIEW IF NOT EXISTS command_stats AS
SELECT
    command_hash,
    command_text,
    current_policy,
    user_override,
    execution_count,
    ROUND(CAST(success_count AS FLOAT) / NULLIF(execution_count, 0) * 100, 2) as success_rate_pct,
    avg_duration_ms,
    risk_score,
    last_executed,
    duplicate_threshold_ms,
    duplicate_check_enabled,
    -- Calcul intervalle moyen entre exécutions à partir des timestamps
    CASE
        WHEN execution_count >= 2 AND last_100_timestamps IS NOT NULL THEN
            (last_executed - first_seen) / NULLIF(execution_count - 1, 0)
        ELSE NULL
    END as avg_interval_seconds
FROM commands_registry
WHERE execution_count > 0;

-- Vue commandes à risque
CREATE VIEW IF NOT EXISTS high_risk_commands AS
SELECT
    command_hash,
    command_text,
    current_policy,
    execution_count,
    risk_score,
    tags
FROM commands_registry
WHERE risk_score >= 0.7
ORDER BY risk_score DESC, execution_count DESC;

-- Vue commandes candidates promotion auto_approve
CREATE VIEW IF NOT EXISTS promotion_candidates AS
SELECT
    command_hash,
    command_text,
    execution_count,
    ROUND(CAST(success_count AS FLOAT) / execution_count * 100, 2) as success_rate_pct,
    current_policy,
    risk_score
FROM commands_registry
WHERE current_policy = 'ask'
  AND execution_count >= 20
  AND CAST(success_count AS FLOAT) / execution_count >= 0.95
  AND risk_score < 0.7
ORDER BY execution_count DESC;

-- Metadata schéma
CREATE TABLE IF NOT EXISTS schema_metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT OR IGNORE INTO schema_metadata (key, value) VALUES
    ('version', '1.0.0'),
    ('created_at', datetime('now')),
    ('description', 'Command security registry with evolving policies');

-- Pragmas optimisés
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA cache_size = -64000;  -- 64MB
PRAGMA foreign_keys = ON;
PRAGMA temp_store = MEMORY;
