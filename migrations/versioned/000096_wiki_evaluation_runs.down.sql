DROP INDEX IF EXISTS idx_evaluation_runs_tenant_type_created;

ALTER TABLE IF EXISTS evaluation_runs DROP COLUMN IF EXISTS result_detail;
ALTER TABLE IF EXISTS evaluation_runs DROP COLUMN IF EXISTS stage_progress;
ALTER TABLE IF EXISTS evaluation_runs DROP COLUMN IF EXISTS failure_stage;
ALTER TABLE IF EXISTS evaluation_runs DROP COLUMN IF EXISTS stage;
ALTER TABLE IF EXISTS evaluation_runs DROP COLUMN IF EXISTS evaluation_type;
