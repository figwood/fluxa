BEGIN;

CREATE TABLE IF NOT EXISTS fluxa_schema_migrations (
    version bigint PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

DO $$
DECLARE pair text[];
BEGIN
    FOREACH pair SLICE 1 IN ARRAY ARRAY[
        ['mvp_projects','projects'],
        ['mvp_gitlab_repositories','gitlab_repositories'],
        ['mvp_project_gitlab_repositories','project_gitlab_repositories'],
        ['mvp_project_services','project_services'],
        ['mvp_tasks','tasks'], ['mvp_task_events','task_events'],
        ['mvp_releases','releases'], ['mvp_release_tasks','release_tasks'],
        ['mvp_release_services','release_services'], ['mvp_artifacts','artifacts'],
        ['mvp_deployments','deployments'], ['mvp_environment_policies','environment_policies'],
        ['mvp_project_ci_tokens','project_ci_tokens'], ['mvp_release_jobs','release_jobs'],
        ['mvp_worker_settings','worker_settings'], ['mvp_worker_heartbeats','worker_heartbeats'],
        ['mvp_release_events','release_events'], ['mvp_project_workflows','project_workflows'],
        ['mvp_project_workflow_statuses','project_workflow_statuses'],
        ['mvp_project_workflow_stages','project_workflow_stages'],
        ['mvp_project_workflow_transitions','project_workflow_transitions'],
        ['mvp_users','users'], ['mvp_sys_roles','sys_roles'],
        ['mvp_sys_role_members','sys_role_members'], ['mvp_privileges','privileges'],
        ['mvp_role_privileges','role_privileges'], ['mvp_project_role_defs','project_role_defs'],
        ['mvp_project_members','project_members'],
        ['mvp_workflows','workflows'], ['mvp_workflow_stages','workflow_stages'],
        ['mvp_workflow_statuses','workflow_statuses'], ['mvp_workflow_transitions','workflow_transitions']
    ]::text[][]
    LOOP
        IF to_regclass('public.' || pair[1]) IS NOT NULL
           AND to_regclass('public.' || pair[2]) IS NULL THEN
            EXECUTE format('ALTER TABLE %I RENAME TO %I', pair[1], pair[2]);
        END IF;
    END LOOP;
END $$;

INSERT INTO fluxa_schema_migrations(version) VALUES (3) ON CONFLICT DO NOTHING;

COMMIT;
