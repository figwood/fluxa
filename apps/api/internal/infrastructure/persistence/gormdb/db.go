package gormdb

import (
	"fmt"
	"os"
	"path/filepath"

	"fluxa-api/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch cfg.DBDriver {
	case "postgres":
		if cfg.PostgresDSN == "" {
			return nil, fmt.Errorf("POSTGRES_DSN is required")
		}
		dialector = postgres.Open(cfg.PostgresDSN)
	case "sqlite":
		if dir := filepath.Dir(cfg.SQLitePath); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, err
			}
		}
		dialector = sqlite.Open(cfg.SQLitePath)
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q", cfg.DBDriver)
	}
	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	if cfg.AutoMigrate {
		if cfg.DBDriver == "postgres" {
			if err := RenameLegacyTables(db); err != nil {
				return nil, err
			}
		}
		if err := AutoMigrate(db); err != nil {
			return nil, err
		}
	}
	return db, nil
}

// RenameLegacyTables preserves existing data while removing the historical
// mvp_ prefix. It runs before AutoMigrate so GORM cannot create parallel empty
// tables under the new names.
func RenameLegacyTables(db *gorm.DB) error {
	return db.Exec(`DO $$
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
			IF to_regclass('public.' || pair[1]) IS NOT NULL AND to_regclass('public.' || pair[2]) IS NULL THEN
				EXECUTE format('ALTER TABLE %I RENAME TO %I', pair[1], pair[2]);
			END IF;
		END LOOP;
	END $$`).Error
}

func AutoMigrate(db *gorm.DB) error {
	// The old MVP schema created a global unique index on idempotency_key.
	if db.Migrator().HasIndex(&ArtifactModel{}, "idx_mvp_artifacts_idempotency_key") {
		if err := db.Migrator().DropIndex(&ArtifactModel{}, "idx_mvp_artifacts_idempotency_key"); err != nil {
			return err
		}
	}
	return db.AutoMigrate(
		&ProjectModel{},
		&GitlabRepositoryModel{},
		&ProjectGitlabRepositoryModel{},
		&ProjectServiceModel{},
		&TaskModel{},
		&TaskEventModel{},
		&ReleaseModel{},
		&ReleaseTaskModel{},
		&ReleaseServiceModel{},
		&ArtifactModel{},
		&DeploymentModel{},
		&EnvironmentPolicyModel{},
		&ProjectCITokenModel{},
		&ReleaseJobModel{},
		&ReleaseEventModel{},
		&WorkerSettingModel{},
		&WorkerHeartbeatModel{},
		&WorkflowModel{},
		&WorkflowStageModel{},
		&WorkflowStatusModel{},
		&WorkflowTransitionModel{},
		&WorkflowExecutionModel{},
		&WorkflowStepExecutionModel{},
		&ApprovalTaskModel{},
		&WorkflowEventModel{},
		&UserModel{},
		&RoleModel{},
		&RoleMemberModel{},
		&PrivilegeModel{},
		&RolePrivilegeModel{},
		&ProjectRoleModel{},
		&ProjectMemberModel{},
	)
}
