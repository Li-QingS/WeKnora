CREATE TABLE IF NOT EXISTS model_prices (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL DEFAULT 0,
    model_id TEXT NOT NULL,
    input_price_per_million REAL,
    output_price_per_million REAL,
    cache_read_price_per_million REAL,
    cache_write_price_per_million REAL,
    unit_type TEXT NOT NULL DEFAULT '',
    unit_price REAL,
    currency TEXT NOT NULL DEFAULT 'USD',
    updated_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, model_id)
);

CREATE INDEX IF NOT EXISTS idx_model_prices_tenant
    ON model_prices(tenant_id);
