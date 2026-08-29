CREATE TABLE IF NOT EXISTS evaluation_runs (
    id VARCHAR(128) PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    dataset_id VARCHAR(128) NOT NULL,

    status INTEGER NOT NULL,
    start_time TIMESTAMP NOT NULL,
    err_msg TEXT NOT NULL DEFAULT '',

    total INTEGER NOT NULL DEFAULT 0,
    finished INTEGER NOT NULL DEFAULT 0,

    params JSONB NOT NULL DEFAULT '{}'::jsonb,
    metric JSONB,

    heartbeat_at TIMESTAMP,
    finished_at TIMESTAMP,
    config_hash VARCHAR(64) NOT NULL DEFAULT '',
    config_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    temporary_kb_id VARCHAR(128) NOT NULL DEFAULT '',

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_evaluation_runs_tenant_created
    ON evaluation_runs(tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_evaluation_runs_tenant_status
    ON evaluation_runs(tenant_id, status);
