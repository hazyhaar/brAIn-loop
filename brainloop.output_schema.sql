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

-- Latency histogram pour percentiles (p50, p95, p99)
CREATE TABLE IF NOT EXISTS latency_histogram (
    operation TEXT NOT NULL,
    bucket_ms INTEGER NOT NULL,     -- 10, 50, 100, 500, 1000, 5000, 10000
    count INTEGER DEFAULT 0,
    timestamp INTEGER NOT NULL,     -- Window timestamp (1-minute buckets)
    PRIMARY KEY (operation, bucket_ms, timestamp)
);

-- Health checks détaillés
CREATE TABLE IF NOT EXISTS health_checks (
    check_name TEXT NOT NULL,
    status TEXT NOT NULL,           -- 'healthy' | 'degraded' | 'unhealthy'
    last_check INTEGER NOT NULL,
    check_count INTEGER DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    details TEXT,
    PRIMARY KEY (check_name)
);

-- Index
CREATE INDEX IF NOT EXISTS idx_metrics_timestamp ON metrics(timestamp);
CREATE INDEX IF NOT EXISTS idx_reader_digests_type ON reader_digests(source_type);
CREATE INDEX IF NOT EXISTS idx_latency_histogram_op ON latency_histogram(operation, timestamp DESC);
