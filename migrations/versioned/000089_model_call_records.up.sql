CREATE TABLE IF NOT EXISTS model_call_records (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    model_id VARCHAR(128) NOT NULL,
    model_name VARCHAR(255) NOT NULL DEFAULT '',
    model_type VARCHAR(32) NOT NULL DEFAULT '',
    purpose VARCHAR(128) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT '',
    started_at TIMESTAMP NOT NULL,
    finished_at TIMESTAMP NOT NULL,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    cache_write_tokens INTEGER NOT NULL DEFAULT 0,
    cache_miss_tokens INTEGER NOT NULL DEFAULT 0,
    unit_type VARCHAR(32) NOT NULL DEFAULT '',
    unit_count BIGINT NOT NULL DEFAULT 0,
    error_type VARCHAR(128) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    session_id VARCHAR(128) NOT NULL DEFAULT '',
    user_id VARCHAR(255) NOT NULL DEFAULT '',
    principal_type VARCHAR(32) NOT NULL DEFAULT '',
    principal_id VARCHAR(255) NOT NULL DEFAULT '',
    request_group_id VARCHAR(128) NOT NULL DEFAULT '',
    trace_id VARCHAR(64) NOT NULL DEFAULT '',
    estimated_cost_usd DECIMAL(20,8),
    price_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_model_call_records_tenant_created
    ON model_call_records(tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_model_call_records_model_id
    ON model_call_records(model_id);
