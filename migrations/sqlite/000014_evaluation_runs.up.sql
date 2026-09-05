CREATE TABLE IF NOT EXISTS evaluation_runs (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    dataset_id TEXT NOT NULL,

    status INTEGER NOT NULL,
    start_time DATETIME NOT NULL,
    err_msg TEXT NOT NULL DEFAULT '',

    total INTEGER NOT NULL DEFAULT 0,
    finished INTEGER NOT NULL DEFAULT 0,

    params TEXT NOT NULL DEFAULT '{}',
    metric TEXT,

    heartbeat_at DATETIME,
    finished_at DATETIME,
    config_hash TEXT NOT NULL DEFAULT '',
    config_snapshot TEXT NOT NULL DEFAULT '{}',
    temporary_kb_id TEXT NOT NULL DEFAULT '',

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_evaluation_runs_tenant_created
    ON evaluation_runs(tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_evaluation_runs_tenant_status
    ON evaluation_runs(tenant_id, status);
