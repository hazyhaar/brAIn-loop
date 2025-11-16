-- ============================================================================
-- MIGRATION 002: Ajout tables HOROS manquantes dans metadata.db
-- Pattern: 4-BDD minimum + tables custom brainloop
-- ============================================================================

PRAGMA foreign_keys=ON;

-- ============================================================================
-- 1. SYSTEM_METRICS - Métriques système (CPU, RAM, disk)
-- ============================================================================

CREATE TABLE IF NOT EXISTS system_metrics (
    metric_id TEXT PRIMARY KEY,
    metric_type TEXT NOT NULL,  -- 'cpu' | 'memory' | 'disk' | 'network' | 'goroutines'
    metric_value REAL NOT NULL,
    metric_unit TEXT,  -- 'percent' | 'bytes' | 'count' | 'ms'
    recorded_at INTEGER NOT NULL,
    metadata TEXT  -- JSON pour contexte additionnel
);

CREATE INDEX IF NOT EXISTS idx_system_metrics_type ON system_metrics(metric_type, recorded_at DESC);

-- ============================================================================
-- 2. BUILD_METRICS - Métriques build et déploiement
-- ============================================================================

CREATE TABLE IF NOT EXISTS build_metrics (
    build_id TEXT PRIMARY KEY,
    build_timestamp INTEGER NOT NULL,
    duration_ms INTEGER NOT NULL,
    status TEXT NOT NULL,  -- 'success' | 'failed' | 'cancelled'
    go_version TEXT,
    binary_size_bytes INTEGER,
    test_count INTEGER,
    test_passed INTEGER,
    test_failed INTEGER,
    coverage_percent REAL,
    git_commit TEXT,
    git_branch TEXT
);

CREATE INDEX IF NOT EXISTS idx_build_metrics_timestamp ON build_metrics(build_timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_build_metrics_status ON build_metrics(status);

-- ============================================================================
-- 3. SECRETS_AUDIT_LOG - Audit accès secrets
-- ============================================================================

CREATE TABLE IF NOT EXISTS secrets_audit_log (
    audit_id TEXT PRIMARY KEY,
    secret_name TEXT NOT NULL,
    action TEXT NOT NULL,  -- 'created' | 'read' | 'rotated' | 'deleted' | 'failed_access'
    actor TEXT,  -- user ou process
    timestamp INTEGER NOT NULL,
    ip_address TEXT,
    details TEXT  -- JSON
);

CREATE INDEX IF NOT EXISTS idx_secrets_audit_timestamp ON secrets_audit_log(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_secrets_audit_secret ON secrets_audit_log(secret_name);

-- ============================================================================
-- 4. IMPORT_STATS - Statistiques imports/sources externes
-- ============================================================================

CREATE TABLE IF NOT EXISTS import_stats (
    import_id TEXT PRIMARY KEY,
    source_name TEXT NOT NULL,
    source_type TEXT NOT NULL,  -- 'mcp' | 'file' | 'database' | 'api'
    items_imported INTEGER NOT NULL DEFAULT 0,
    items_failed INTEGER DEFAULT 0,
    bytes_imported INTEGER,
    duration_ms INTEGER,
    imported_at INTEGER NOT NULL,
    error_log TEXT
);

CREATE INDEX IF NOT EXISTS idx_import_stats_source ON import_stats(source_name, imported_at DESC);

-- ============================================================================
-- 5. PERFORMANCE_BASELINE - Baselines performance (SLA)
-- ============================================================================

CREATE TABLE IF NOT EXISTS performance_baseline (
    operation_name TEXT PRIMARY KEY,
    p50_ms REAL,  -- Median
    p95_ms REAL,  -- 95th percentile
    p99_ms REAL,  -- 99th percentile
    avg_ms REAL,
    min_ms REAL,
    max_ms REAL,
    samples_count INTEGER NOT NULL,
    last_updated INTEGER NOT NULL,
    sla_target_ms INTEGER  -- SLA cible si défini
);

CREATE INDEX IF NOT EXISTS idx_performance_baseline_updated ON performance_baseline(last_updated DESC);

-- ============================================================================
-- VALIDATION FINALE
-- ============================================================================

-- Compter tables après migration
SELECT COUNT(*) as total_tables
FROM sqlite_master
WHERE type='table' AND name NOT LIKE 'sqlite_%';
