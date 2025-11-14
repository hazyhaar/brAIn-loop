-- ============================================================================
-- BRAINLOOP LIFECYCLE SCHEMA
-- Sessions, blocks, cache, processed_log pour idempotence
-- ============================================================================

-- Configuration runtime
CREATE TABLE IF NOT EXISTS config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Idempotence tracking
CREATE TABLE IF NOT EXISTS processed_log (
    hash TEXT PRIMARY KEY,          -- sha256(session_id + block_id + code)
    operation TEXT NOT NULL,        -- 'propose' | 'refine' | 'commit' | 'read'
    timestamp INTEGER NOT NULL,
    result_json TEXT
);

-- Sessions cerebras_loop
CREATE TABLE IF NOT EXISTS sessions (
    session_id TEXT PRIMARY KEY,
    status TEXT NOT NULL,           -- 'pending_audit' | 'committed' | 'abandoned'
    created_at INTEGER NOT NULL,
    completed_at INTEGER
);

-- Blocks dans sessions
CREATE TABLE IF NOT EXISTS session_blocks (
    block_id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    description TEXT NOT NULL,
    type TEXT NOT NULL,             -- 'sql' | 'go' | 'python' | 'code'
    target TEXT NOT NULL,           -- file_path ou db_path
    code TEXT,                      -- Code généré actuel
    iterations INTEGER DEFAULT 0,
    status TEXT DEFAULT 'pending',  -- 'pending' | 'committed'
    generated_at INTEGER NOT NULL,
    last_refined_at INTEGER,
    committed_at INTEGER,
    FOREIGN KEY (session_id) REFERENCES sessions(session_id)
);

-- Audit feedback sur blocks
CREATE TABLE IF NOT EXISTS block_refinements (
    refinement_id TEXT PRIMARY KEY,
    block_id TEXT NOT NULL,
    feedback TEXT NOT NULL,
    temperature REAL NOT NULL,
    refined_code TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (block_id) REFERENCES session_blocks(block_id)
);

-- Reader cache (éviter re-lecture)
CREATE TABLE IF NOT EXISTS reader_cache (
    hash TEXT PRIMARY KEY,          -- sha256(file_path + file_mtime)
    source_type TEXT NOT NULL,      -- 'sqlite' | 'markdown' | 'code' | 'config'
    source_path TEXT NOT NULL,
    digest_json TEXT NOT NULL,
    cached_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

-- Queue de génération
CREATE TABLE IF NOT EXISTS processing_queue (
    queue_id INTEGER PRIMARY KEY AUTOINCREMENT,
    operation_type TEXT NOT NULL,   -- 'generate' | 'read' | 'refine'
    payload_json TEXT NOT NULL,
    priority INTEGER DEFAULT 0,
    status TEXT DEFAULT 'pending',  -- 'pending' | 'processing' | 'completed' | 'failed'
    added_at INTEGER,
    started_at INTEGER,
    completed_at INTEGER,
    attempts INTEGER DEFAULT 0,
    error_message TEXT
);

-- Metadata schéma
CREATE TABLE IF NOT EXISTS schema_metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT OR IGNORE INTO schema_metadata (key, value) VALUES
    ('version', '1.0.0'),
    ('created_at', datetime('now'));

-- Cerebras API usage tracking
CREATE TABLE IF NOT EXISTS cerebras_usage (
    request_id TEXT PRIMARY KEY,
    operation TEXT NOT NULL,        -- Action name
    model TEXT NOT NULL,            -- zai-glm-4.6
    temperature REAL NOT NULL,
    tokens_prompt INTEGER,
    tokens_completion INTEGER,
    latency_ms INTEGER,
    timestamp INTEGER NOT NULL
);

-- Pattern detection cache
CREATE TABLE IF NOT EXISTS detected_patterns (
    pattern_id TEXT PRIMARY KEY,
    source_path TEXT NOT NULL,
    pattern_type TEXT NOT NULL,     -- 'naming' | 'imports' | 'error_handling'
    pattern_json TEXT NOT NULL,
    confidence_score REAL NOT NULL,
    detected_at INTEGER NOT NULL
);

-- Référence légère vers command_security.db
-- Permet ATTACH et jointures sans dupliquer données
CREATE TABLE IF NOT EXISTS command_security_refs (
    command_hash TEXT PRIMARY KEY,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_command_security_refs_created ON command_security_refs(created_at DESC);

-- Index pour performance
CREATE INDEX IF NOT EXISTS idx_session_blocks_session ON session_blocks(session_id);
CREATE INDEX IF NOT EXISTS idx_reader_cache_hash ON reader_cache(hash);
CREATE INDEX IF NOT EXISTS idx_processing_queue_status ON processing_queue(status, priority);
CREATE INDEX IF NOT EXISTS idx_cerebras_usage_operation ON cerebras_usage(operation, timestamp);
