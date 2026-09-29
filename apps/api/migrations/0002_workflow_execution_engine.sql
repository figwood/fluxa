BEGIN;

CREATE TABLE IF NOT EXISTS workflow_executions (
    id varchar(64) PRIMARY KEY,
    release_id varchar(64) NOT NULL UNIQUE,
    project_id varchar(64) NOT NULL,
    definition_name varchar(128) NOT NULL,
    definition_version integer NOT NULL,
    definition_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    public_status varchar(32) NOT NULL,
    phase varchar(32) NOT NULL,
    current_step_id varchar(128) NOT NULL,
    wake_up_at timestamptz,
    last_error text NOT NULL DEFAULT '',
    context jsonb NOT NULL DEFAULT '{}'::jsonb,
    lock_version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    completed_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_workflow_executions_project_id ON workflow_executions(project_id);
CREATE INDEX IF NOT EXISTS idx_workflow_executions_phase ON workflow_executions(phase);
CREATE INDEX IF NOT EXISTS idx_workflow_executions_wake_up_at ON workflow_executions(wake_up_at);

CREATE TABLE IF NOT EXISTS workflow_step_executions (
    id varchar(64) PRIMARY KEY,
    execution_id varchar(64) NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
    step_id varchar(128) NOT NULL,
    attempt integer NOT NULL,
    status varchar(32) NOT NULL,
    idempotency_key varchar(255) NOT NULL UNIQUE,
    input jsonb NOT NULL DEFAULT '{}'::jsonb,
    output jsonb NOT NULL DEFAULT '{}'::jsonb,
    error text NOT NULL DEFAULT '',
    started_at timestamptz NOT NULL,
    finished_at timestamptz,
    UNIQUE(execution_id, step_id, attempt)
);
CREATE INDEX IF NOT EXISTS idx_workflow_step_executions_execution_id ON workflow_step_executions(execution_id);

CREATE TABLE IF NOT EXISTS approval_tasks (
    id varchar(64) PRIMARY KEY,
    execution_id varchar(64) NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
    step_id varchar(128) NOT NULL,
    approver varchar(255) NOT NULL,
    decision varchar(32) NOT NULL DEFAULT 'pending',
    decided_by varchar(255) NOT NULL DEFAULT '',
    decided_at timestamptz,
    created_at timestamptz NOT NULL,
    UNIQUE(execution_id, step_id, approver)
);
CREATE INDEX IF NOT EXISTS idx_approval_tasks_approver ON approval_tasks(approver);
CREATE INDEX IF NOT EXISTS idx_approval_tasks_decision ON approval_tasks(decision);

CREATE TABLE IF NOT EXISTS workflow_events (
    id varchar(64) PRIMARY KEY,
    execution_id varchar(64) NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
    sequence bigint NOT NULL,
    type varchar(64) NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL,
    UNIQUE(execution_id, sequence)
);
CREATE INDEX IF NOT EXISTS idx_workflow_events_execution_id ON workflow_events(execution_id);
CREATE INDEX IF NOT EXISTS idx_workflow_events_type ON workflow_events(type);

CREATE TABLE IF NOT EXISTS fluxa_schema_migrations (
    version bigint PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO fluxa_schema_migrations(version) VALUES (2) ON CONFLICT DO NOTHING;

COMMIT;
