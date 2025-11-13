-- ============================================================================
-- BRAINLOOP METADATA SCHEMA
-- Poison pill, secrets, télémétrie
-- ============================================================================

-- Poison pill
CREATE TABLE IF NOT EXISTS poisonpill (
    signal_type TEXT PRIMARY KEY,
    executed INTEGER DEFAULT 0,
    executed_at INTEGER,
    execution_result TEXT
);

-- Télémétrie events
CREATE TABLE IF NOT EXISTS telemetry_events (
    timestamp INTEGER NOT NULL,
    event_type TEXT NOT NULL,       -- 'generation' | 'read' | 'session_created' | 'pattern_detected'
    description TEXT
);

-- Secrets (Cerebras API key)
CREATE TABLE IF NOT EXISTS secrets (
    secret_name TEXT PRIMARY KEY,
    secret_value TEXT NOT NULL,     -- Plaintext pour dev, encrypted pour prod
    created_at INTEGER NOT NULL,
    last_rotated INTEGER
);

-- Configuration initiale
INSERT OR IGNORE INTO secrets (secret_name, secret_value, created_at) VALUES
    ('CEREBRAS_API_KEY', 'sk-placeholder-replace-me', strftime('%s', 'now'));

-- Index
CREATE INDEX IF NOT EXISTS idx_poisonpill_executed ON poisonpill(signal_type, executed);
CREATE INDEX IF NOT EXISTS idx_telemetry_timestamp ON telemetry_events(timestamp, event_type);
