BEGIN;

ALTER TABLE deployments ADD COLUMN IF NOT EXISTS execution_id varchar(64) NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS step_id varchar(128) NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS attempt integer NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_deployments_execution_id ON deployments(execution_id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_deployment_attempt
 ON deployments(execution_id, step_id, attempt, project_service_id)
 WHERE execution_id <> '';

COMMIT;
