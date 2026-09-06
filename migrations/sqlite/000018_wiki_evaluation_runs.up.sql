ALTER TABLE evaluation_runs ADD COLUMN evaluation_type TEXT NOT NULL DEFAULT 'rag';
ALTER TABLE evaluation_runs ADD COLUMN stage TEXT NOT NULL DEFAULT '';
ALTER TABLE evaluation_runs ADD COLUMN failure_stage TEXT NOT NULL DEFAULT '';
ALTER TABLE evaluation_runs ADD COLUMN stage_progress TEXT NOT NULL DEFAULT '{}';
ALTER TABLE evaluation_runs ADD COLUMN result_detail TEXT;

CREATE INDEX IF NOT EXISTS idx_evaluation_runs_tenant_type_created
    ON evaluation_runs(tenant_id, evaluation_type, created_at DESC);
