-- ============================================================================
-- BRAINLOOP OUTPUT SCHEMA
-- Résultats finaux, heartbeat, metrics, digests publiés
-- ============================================================================

-- Résultats finaux (sessions committed)
CREATE TABLE IF NOT EXISTS results (
    hash TEXT PRIMARY KEY,          -- sha256(session_id)
    session_id TEXT NOT NULL,
    blocks_committed INTEGER NOT NULL,
    data_json TEXT NOT NULL,        -- Session complète
    created_at INTEGER NOT NULL
);

-- Heartbeat worker
CREATE TABLE IF NOT EXISTS heartbeat (
    worker_id TEXT PRIMARY KEY,
    timestamp INTEGER NOT NULL,
    status TEXT NOT NULL,           -- 'running' | 'shutting_down'
    sessions_active INTEGER,
    sessions_completed INTEGER,
    cache_hit_rate REAL
);

-- Métriques observabilité
CREATE TABLE IF NOT EXISTS metrics (
    timestamp INTEGER NOT NULL,
    metric_name TEXT NOT NULL,
    metric_value REAL NOT NULL
);

-- Reader digests publiés
CREATE TABLE IF NOT EXISTS reader_digests (
    hash TEXT PRIMARY KEY,          -- sha256(source_path + digest)
    source_type TEXT NOT NULL,      -- 'sqlite' | 'markdown' | 'code' | 'config'
    source_path TEXT NOT NULL,
    digest_json TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

-- Index
CREATE INDEX IF NOT EXISTS idx_metrics_timestamp ON metrics(timestamp);
CREATE INDEX IF NOT EXISTS idx_reader_digests_type ON reader_digests(source_type);
