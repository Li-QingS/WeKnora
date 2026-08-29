ALTER TABLE IF EXISTS evaluation_runs DROP COLUMN IF EXISTS heartbeat_at;
ALTER TABLE IF EXISTS evaluation_runs DROP COLUMN IF EXISTS finished_at;
ALTER TABLE IF EXISTS evaluation_runs DROP COLUMN IF EXISTS config_hash;
ALTER TABLE IF EXISTS evaluation_runs DROP COLUMN IF EXISTS config_snapshot;
ALTER TABLE IF EXISTS evaluation_runs DROP COLUMN IF EXISTS temporary_kb_id;

DROP TABLE IF EXISTS evaluation_runs;
