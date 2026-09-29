BEGIN;

CREATE TABLE IF NOT EXISTS fluxa_schema_migrations (
    version bigint PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

DROP INDEX IF EXISTS idx_mvp_artifacts_idempotency_key;
CREATE UNIQUE INDEX IF NOT EXISTS uk_project_artifact_idempotency
    ON mvp_artifacts (project_id, idempotency_key);

ALTER TABLE mvp_tasks ADD COLUMN IF NOT EXISTS creator varchar(128) NOT NULL DEFAULT '';
ALTER TABLE mvp_users ADD COLUMN IF NOT EXISTS auth_version integer NOT NULL DEFAULT 1;
UPDATE mvp_tasks AS task
SET creator = COALESCE(
    NULLIF((
        SELECT event.actor
        FROM mvp_task_events AS event
        WHERE event.task_id = task.id AND event.type = 'task.created' AND event.actor <> ''
        ORDER BY event.created_at ASC
        LIMIT 1
    ), ''),
    NULLIF(task.assignee, ''),
    '-'
)
WHERE task.creator = '';

ALTER TABLE mvp_task_events ADD COLUMN IF NOT EXISTS release_id varchar(64) NOT NULL DEFAULT '';
ALTER TABLE mvp_task_events ADD COLUMN IF NOT EXISTS job_id varchar(64) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_mvp_task_events_release_id ON mvp_task_events (release_id);
CREATE INDEX IF NOT EXISTS idx_mvp_task_events_job_id ON mvp_task_events (job_id);

ALTER TABLE mvp_release_jobs ADD COLUMN IF NOT EXISTS lease_until timestamptz;
ALTER TABLE mvp_release_jobs ADD COLUMN IF NOT EXISTS heartbeat_at timestamptz;
ALTER TABLE mvp_release_jobs ADD COLUMN IF NOT EXISTS attempt integer NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_mvp_release_jobs_lease_until ON mvp_release_jobs (lease_until);

INSERT INTO fluxa_schema_migrations (version) VALUES (1)
ON CONFLICT (version) DO NOTHING;

COMMIT;
