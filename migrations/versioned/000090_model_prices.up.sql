CREATE TABLE IF NOT EXISTS model_prices (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id BIGINT NOT NULL DEFAULT 0,
    model_id VARCHAR(128) NOT NULL,
    input_price_per_million DECIMAL(20,8),
    output_price_per_million DECIMAL(20,8),
    cache_read_price_per_million DECIMAL(20,8),
    cache_write_price_per_million DECIMAL(20,8),
    unit_type VARCHAR(32) NOT NULL DEFAULT '',
    unit_price DECIMAL(20,8),
    currency VARCHAR(8) NOT NULL DEFAULT 'USD',
    updated_by VARCHAR(128) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, model_id)
);

CREATE INDEX IF NOT EXISTS idx_model_prices_tenant
    ON model_prices(tenant_id);
