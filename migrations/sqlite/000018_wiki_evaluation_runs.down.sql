DROP INDEX IF EXISTS idx_evaluation_runs_tenant_type_created;

ALTER TABLE evaluation_runs DROP COLUMN result_detail;
ALTER TABLE evaluation_runs DROP COLUMN stage_progress;
ALTER TABLE evaluation_runs DROP COLUMN failure_stage;
ALTER TABLE evaluation_runs DROP COLUMN stage;
ALTER TABLE evaluation_runs DROP COLUMN evaluation_type;
