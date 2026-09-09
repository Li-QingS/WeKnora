CREATE INDEX IF NOT EXISTS idx_model_call_records_tenant_started
    ON model_call_records(tenant_id, started_at DESC);
