-- ============================================================================
-- MIGRATION 001: Ajout tables HOROS manquantes dans lifecycle.db
-- Pattern: 4-BDD minimum + tables custom brainloop
-- ============================================================================

PRAGMA foreign_keys=ON;

-- ============================================================================
-- 1. EGO_INDEX - CRITIQUE (15 dimensions universelles HOROS)
-- ============================================================================

CREATE TABLE IF NOT EXISTS ego_index (
    key TEXT PRIMARY KEY,
    description TEXT NOT NULL,
    type TEXT,
    value TEXT
);

-- Remplir 15 dimensions obligatoires
INSERT OR IGNORE INTO ego_index (key, description, type, value) VALUES
('dim_origines', 'Origine du composant', 'string', 'Worker HOROS MCP - boucles LLM Cerebras'),
('dim_composition', 'Composition technique', 'string', 'Go + SQLite + Cerebras API + MCP stdio + bash sandboxing'),
('dim_finalites', 'Finalité principale', 'string', 'Génération code via LLM, lecture intelligente sources, exécution bash sécurisée'),
('dim_interactions', 'Mode d''interaction', 'string', 'MCP stdio (12 actions progressive disclosure)'),
('dim_dependances', 'Dépendances externes', 'string', 'Cerebras API (llama-3.3-70b), modernc.org/sqlite, command_security.db'),
('dim_temporalite', 'Temporalité', 'string', 'Streaming 24/7 + sessions on-demand'),
('dim_cardinalite', 'Cardinalité', 'string', '1 instance unique par environnement'),
('dim_observabilite', 'Observabilité', 'string', 'Heartbeat 15s, métriques Cerebras, telemetry events'),
('dim_reversibilite', 'Réversibilité', 'string', 'Sessions abandonnées, blocks non-committed rollbackables'),
('dim_congruence', 'Congruence', 'string', 'brainloop/brainloop.*.db + command_security.db'),
('dim_anticipation', 'Anticipation', 'string', 'Quota Cerebras, injection bash, commandes dangereuses, cache invalidation'),
('dim_granularite', 'Granularité', 'string', 'Génération par block (SQL/Go/Python), lecture par fichier'),
('dim_conditionnalite', 'Conditionnalité', 'string', 'Actif si Cerebras API disponible + CEREBRAS_API_KEY configurée'),
('dim_autorite', 'Autorité', 'string', 'Lecture seule sources, write filesystem via generate_file, bash via policies évolutives'),
('dim_mutabilite', 'Mutabilité', 'string', 'Policies bash, cache TTL, température Cerebras configurables runtime');

CREATE INDEX IF NOT EXISTS idx_ego_index_key ON ego_index(key);

-- ============================================================================
-- 2. DEPENDENCIES - Contrats upstream
-- ============================================================================

CREATE TABLE IF NOT EXISTS dependencies (
    dependency_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT,
    type TEXT NOT NULL,  -- 'worker' | 'library' | 'api' | 'database'
    required BOOLEAN DEFAULT 1,
    health_check_url TEXT,
    last_check_at INTEGER,
    status TEXT  -- 'healthy' | 'degraded' | 'down'
);

CREATE INDEX IF NOT EXISTS idx_dependencies_type ON dependencies(type, status);

-- ============================================================================
-- 3. COMPONENT_SPECS - Spécifications composants
-- ============================================================================

CREATE TABLE IF NOT EXISTS component_specs (
    id TEXT PRIMARY KEY,
    component_name TEXT NOT NULL,
    spec TEXT NOT NULL,  -- JSON ou YAML
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_component_specs_name ON component_specs(component_name);

-- ============================================================================
-- 4. PROJECT_FUNCTIONS - Fonctions projet
-- ============================================================================

CREATE TABLE IF NOT EXISTS project_functions (
    name TEXT PRIMARY KEY,
    signature TEXT NOT NULL,
    implementation TEXT,
    language TEXT NOT NULL,  -- 'go' | 'sql' | 'python'
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_project_functions_language ON project_functions(language);

-- ============================================================================
-- 5. MANUAL_TASKS - Tâches manuelles
-- ============================================================================

CREATE TABLE IF NOT EXISTS manual_tasks (
    id TEXT PRIMARY KEY,
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',  -- 'pending' | 'in_progress' | 'completed' | 'cancelled'
    priority INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    completed_at INTEGER,
    assigned_to TEXT
);

CREATE INDEX IF NOT EXISTS idx_manual_tasks_status ON manual_tasks(status, priority DESC);

-- ============================================================================
-- 6. CACHE - Cache générique (complément reader_cache existant)
-- ============================================================================

CREATE TABLE IF NOT EXISTS cache (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cache_expires ON cache(expires_at);

-- ============================================================================
-- 7. LAST_CHECK_TIMESTAMPS - Timestamps checks santé
-- ============================================================================

CREATE TABLE IF NOT EXISTS last_check_timestamps (
    check_name TEXT PRIMARY KEY,
    timestamp INTEGER NOT NULL,
    status TEXT,
    details TEXT
);

-- ============================================================================
-- 8. TELEMETRY_TRACES - Traces distribuées
-- ============================================================================

CREATE TABLE IF NOT EXISTS telemetry_traces (
    trace_id TEXT PRIMARY KEY,
    span_id TEXT NOT NULL,
    parent_span_id TEXT,
    operation_name TEXT NOT NULL,
    start_time INTEGER NOT NULL,
    end_time INTEGER,
    duration_ms INTEGER,
    tags TEXT,  -- JSON
    status TEXT  -- 'ok' | 'error'
);

CREATE INDEX IF NOT EXISTS idx_telemetry_traces_start ON telemetry_traces(start_time DESC);
CREATE INDEX IF NOT EXISTS idx_telemetry_traces_operation ON telemetry_traces(operation_name);

-- ============================================================================
-- 9. TELEMETRY_LOGS - Logs structurés
-- ============================================================================

CREATE TABLE IF NOT EXISTS telemetry_logs (
    log_id TEXT PRIMARY KEY,
    timestamp INTEGER NOT NULL,
    level TEXT NOT NULL,  -- 'debug' | 'info' | 'warn' | 'error'
    message TEXT NOT NULL,
    context TEXT,  -- JSON
    trace_id TEXT
);

CREATE INDEX IF NOT EXISTS idx_telemetry_logs_timestamp ON telemetry_logs(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_telemetry_logs_level ON telemetry_logs(level);

-- ============================================================================
-- 10. TELEMETRY_LLM_METRICS - Métriques LLM (migrer cerebras_usage)
-- ============================================================================

CREATE TABLE IF NOT EXISTS telemetry_llm_metrics (
    metric_id TEXT PRIMARY KEY,
    timestamp INTEGER NOT NULL,
    model TEXT NOT NULL,
    operation TEXT NOT NULL,
    tokens_prompt INTEGER,
    tokens_completion INTEGER,
    tokens_total INTEGER,
    response_time_ms INTEGER,
    cost REAL,
    temperature REAL,
    trace_id TEXT
);

CREATE INDEX IF NOT EXISTS idx_telemetry_llm_timestamp ON telemetry_llm_metrics(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_telemetry_llm_model ON telemetry_llm_metrics(model, operation);

-- Migrer données existantes cerebras_usage vers telemetry_llm_metrics
INSERT OR IGNORE INTO telemetry_llm_metrics (
    metric_id, timestamp, model, operation,
    tokens_prompt, tokens_completion,
    tokens_total, response_time_ms, temperature
)
SELECT
    request_id,
    timestamp,
    model,
    operation,
    tokens_prompt,
    tokens_completion,
    tokens_prompt + tokens_completion,
    latency_ms,
    temperature
FROM cerebras_usage;

-- ============================================================================
-- 11. TELEMETRY_SECURITY_EVENTS - Events sécurité
-- ============================================================================

CREATE TABLE IF NOT EXISTS telemetry_security_events (
    event_id TEXT PRIMARY KEY,
    timestamp INTEGER NOT NULL,
    event_type TEXT NOT NULL,  -- 'injection_detected' | 'dangerous_command' | 'policy_violation'
    severity TEXT NOT NULL,  -- 'low' | 'medium' | 'high' | 'critical'
    details TEXT,  -- JSON
    command_hash TEXT,
    blocked BOOLEAN DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_telemetry_security_timestamp ON telemetry_security_events(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_telemetry_security_severity ON telemetry_security_events(severity);

-- ============================================================================
-- 12. SECRETS_REGISTRY - Registry secrets
-- ============================================================================

CREATE TABLE IF NOT EXISTS secrets_registry (
    secret_name TEXT PRIMARY KEY,
    encrypted_value TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    last_accessed INTEGER,
    last_rotated INTEGER,
    rotation_policy TEXT  -- 'manual' | 'auto_30d' | 'auto_90d'
);

CREATE INDEX IF NOT EXISTS idx_secrets_registry_rotated ON secrets_registry(last_rotated);

-- ============================================================================
-- 13. ENVIRONMENT_CONFIG - Configuration environnement
-- ============================================================================

CREATE TABLE IF NOT EXISTS environment_config (
    env_key TEXT PRIMARY KEY,
    env_value TEXT NOT NULL,
    is_encrypted BOOLEAN DEFAULT 0,
    description TEXT,
    updated_at INTEGER NOT NULL
);

-- ============================================================================
-- 14. NETWORK_CONFIG - Configuration réseau
-- ============================================================================

CREATE TABLE IF NOT EXISTS network_config (
    config_key TEXT PRIMARY KEY,
    config_value TEXT NOT NULL,
    protocol TEXT,  -- 'http' | 'https' | 'grpc' | 'stdio'
    description TEXT,
    updated_at INTEGER NOT NULL
);

-- ============================================================================
-- 15. SSH_AUTHORIZED_KEYS - Clés SSH autorisées
-- ============================================================================

CREATE TABLE IF NOT EXISTS ssh_authorized_keys (
    key_id TEXT PRIMARY KEY,
    key_content TEXT NOT NULL,
    key_type TEXT NOT NULL,  -- 'rsa' | 'ed25519' | 'ecdsa'
    comment TEXT,
    added_at INTEGER NOT NULL,
    last_used INTEGER
);

CREATE INDEX IF NOT EXISTS idx_ssh_keys_added ON ssh_authorized_keys(added_at DESC);

-- ============================================================================
-- VALIDATION FINALE
-- ============================================================================

-- Compter tables après migration
SELECT COUNT(*) as total_tables
FROM sqlite_master
WHERE type='table' AND name NOT LIKE 'sqlite_%';
