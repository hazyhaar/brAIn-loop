-- ============================================================================
-- BRAINLOOP INPUT SCHEMA
-- Sources externes (autres workers, bases HOROS, fichiers)
-- ============================================================================

-- Sources externes
CREATE TABLE IF NOT EXISTS input_sources (
    id TEXT PRIMARY KEY,
    source_type TEXT NOT NULL,      -- 'worker_output' | 'filesystem' | 'database'
    source_path TEXT NOT NULL,
    attached_at INTEGER NOT NULL
);

-- Dépendances vers autres projets HOROS
CREATE TABLE IF NOT EXISTS input_dependencies (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL,
    dependency_name TEXT NOT NULL,
    dependency_version TEXT,
    FOREIGN KEY (source_id) REFERENCES input_sources(id)
);

-- Schémas des sources
CREATE TABLE IF NOT EXISTS input_schemas (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL,
    schema_name TEXT NOT NULL,
    schema_json TEXT NOT NULL,
    FOREIGN KEY (source_id) REFERENCES input_sources(id)
);

-- Contrats MCP
CREATE TABLE IF NOT EXISTS input_contracts (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL,
    contract_type TEXT NOT NULL,    -- 'mcp_tool' | 'api_endpoint'
    contract_json TEXT NOT NULL,
    FOREIGN KEY (source_id) REFERENCES input_sources(id)
);

-- Santé des dépendances
CREATE TABLE IF NOT EXISTS input_health (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL,
    status TEXT NOT NULL,           -- 'healthy' | 'degraded' | 'down'
    last_check INTEGER NOT NULL,
    FOREIGN KEY (source_id) REFERENCES input_sources(id)
);
