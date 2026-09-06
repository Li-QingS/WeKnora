ALTER TABLE evaluation_runs
    ADD COLUMN IF NOT EXISTS evaluation_type VARCHAR(16) NOT NULL DEFAULT 'rag';

ALTER TABLE evaluation_runs
    ADD COLUMN IF NOT EXISTS stage VARCHAR(32) NOT NULL DEFAULT '';

ALTER TABLE evaluation_runs
    ADD COLUMN IF NOT EXISTS failure_stage VARCHAR(32) NOT NULL DEFAULT '';

ALTER TABLE evaluation_runs
    ADD COLUMN IF NOT EXISTS stage_progress JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE evaluation_runs
    ADD COLUMN IF NOT EXISTS result_detail JSONB;

CREATE INDEX IF NOT EXISTS idx_evaluation_runs_tenant_type_created
    ON evaluation_runs(tenant_id, evaluation_type, created_at DESC);
