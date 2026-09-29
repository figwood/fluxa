package gormdb

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type JSONMap map[string]any

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	return string(b), err
}

func (m *JSONMap) Scan(value any) error {
	if value == nil {
		*m = JSONMap{}
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("unsupported JSONMap scan type %T", value)
	}
	if len(raw) == 0 {
		*m = JSONMap{}
		return nil
	}
	return json.Unmarshal(raw, m)
}

type ProjectModel struct {
	ID          string `gorm:"primaryKey;size:64"`
	Name        string `gorm:"size:160;not null"`
	Key         string `gorm:"size:64;not null;uniqueIndex"`
	Description string `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (ProjectModel) TableName() string { return "projects" }

type GitlabRepositoryModel struct {
	ID              string `gorm:"primaryKey;size:64"`
	GitlabProjectID int64  `gorm:"not null;uniqueIndex"`
	Name            string `gorm:"size:160;not null"`
	Path            string `gorm:"size:255;not null"`
	URL             string `gorm:"size:512"`
	DefaultBranch   string `gorm:"size:128;not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (GitlabRepositoryModel) TableName() string { return "gitlab_repositories" }

type ProjectGitlabRepositoryModel struct {
	ID              int64  `gorm:"primaryKey;autoIncrement"`
	ProjectID       string `gorm:"size:64;not null;index;uniqueIndex:uk_project_gitlab_repository"`
	GitlabProjectID int64  `gorm:"not null;index;uniqueIndex:uk_project_gitlab_repository"`
	CreatedAt       time.Time
}

func (ProjectGitlabRepositoryModel) TableName() string { return "project_gitlab_repositories" }

type ProjectServiceModel struct {
	ID              string  `gorm:"primaryKey;size:64"`
	ProjectID       string  `gorm:"size:64;not null;index;uniqueIndex:uk_project_service"`
	GitlabProjectID int64   `gorm:"not null;index"`
	ServiceKey      string  `gorm:"size:128;not null;uniqueIndex:uk_project_service"`
	DisplayName     string  `gorm:"size:255;not null"`
	ModulePath      string  `gorm:"size:255"`
	ImageName       string  `gorm:"size:255"`
	DeployTarget    string  `gorm:"size:32;not null"`
	DeployConfig    JSONMap `gorm:"type:text"`
	Status          string  `gorm:"size:32;not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (ProjectServiceModel) TableName() string { return "project_services" }

type TaskModel struct {
	ID          string  `gorm:"primaryKey;size:64"`
	ProjectID   string  `gorm:"size:64;not null;index"`
	Title       string  `gorm:"size:255;not null"`
	Description string  `gorm:"type:text"`
	Creator     string  `gorm:"size:128"`
	Assignee    string  `gorm:"size:128"`
	Status      string  `gorm:"size:32;not null;index"`
	Details     JSONMap `gorm:"type:jsonb"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (TaskModel) TableName() string { return "tasks" }

type TaskEventModel struct {
	ID         string `gorm:"primaryKey;size:64"`
	TaskID     string `gorm:"size:64;not null;index"`
	Type       string `gorm:"size:64;not null;index"`
	Title      string `gorm:"size:160;not null"`
	Message    string `gorm:"type:text"`
	Actor      string `gorm:"size:128"`
	FromStatus string `gorm:"size:64"`
	ToStatus   string `gorm:"size:64"`
	ReleaseID  string `gorm:"size:64;index"`
	JobID      string `gorm:"size:64;index"`
	CreatedAt  time.Time
}

func (TaskEventModel) TableName() string { return "task_events" }

type ReleaseModel struct {
	ID          string  `gorm:"primaryKey;size:64"`
	ProjectID   string  `gorm:"size:64;not null;index"`
	Title       string  `gorm:"size:255;not null"`
	Description string  `gorm:"type:text"`
	Environment string  `gorm:"size:64;not null"`
	Status      string  `gorm:"size:32;not null;index"`
	Details     JSONMap `gorm:"type:jsonb"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (ReleaseModel) TableName() string { return "releases" }

type ReleaseTaskModel struct {
	ID        string `gorm:"primaryKey;size:64"`
	ReleaseID string `gorm:"size:64;not null;index"`
	TaskID    string `gorm:"size:64;not null;index"`
	Title     string `gorm:"size:255;not null"`
	Status    string `gorm:"size:32;not null"`
	CreatedAt time.Time
}

func (ReleaseTaskModel) TableName() string { return "release_tasks" }

type ReleaseServiceModel struct {
	ID               string  `gorm:"primaryKey;size:64"`
	ReleaseID        string  `gorm:"size:64;not null;index"`
	ProjectServiceID string  `gorm:"size:64;not null;index"`
	ArtifactID       string  `gorm:"size:64;index"`
	ServiceKey       string  `gorm:"size:128;not null"`
	DisplayName      string  `gorm:"size:255;not null"`
	DeployTarget     string  `gorm:"size:32;not null"`
	DeployConfig     JSONMap `gorm:"type:text"`
	Ref              string  `gorm:"size:255"`
	OrderIndex       int     `gorm:"not null;index"`
	Status           string  `gorm:"size:32;not null;index"`
	ExternalID       string  `gorm:"size:255"`
	ExternalURL      string  `gorm:"size:512"`
	Message          string  `gorm:"type:text"`
	StartedAt        *time.Time
	FinishedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (ReleaseServiceModel) TableName() string { return "release_services" }

type ArtifactModel struct {
	ID               string `gorm:"primaryKey;size:64"`
	ProjectID        string `gorm:"size:64;not null;index;uniqueIndex:uk_project_artifact_idempotency"`
	ProjectServiceID string `gorm:"size:64;not null;index"`
	CommitSHA        string `gorm:"size:128;not null;index"`
	RefType          string `gorm:"size:32;not null;index"`
	Ref              string `gorm:"size:255;not null;index"`
	ImageRepository  string `gorm:"size:512;not null"`
	ImageTag         string `gorm:"size:255;not null"`
	ImageDigest      string `gorm:"size:255;not null;index"`
	PipelineID       string `gorm:"size:128;index"`
	PipelineURL      string `gorm:"size:512"`
	IdempotencyKey   string `gorm:"size:255;not null;uniqueIndex:uk_project_artifact_idempotency"`
	BuiltAt          time.Time
	CreatedAt        time.Time
}

func (ArtifactModel) TableName() string { return "artifacts" }

type DeploymentModel struct {
	ExecutionID      string `gorm:"size:64;index;uniqueIndex:uk_deployment_attempt,where:execution_id <> ''"`
	StepID           string `gorm:"size:128;uniqueIndex:uk_deployment_attempt,where:execution_id <> ''"`
	Attempt          int    `gorm:"uniqueIndex:uk_deployment_attempt,where:execution_id <> ''"`
	ID               string `gorm:"primaryKey;size:64"`
	ProjectID        string `gorm:"size:64;not null;index"`
	ProjectServiceID string `gorm:"size:64;not null;index;uniqueIndex:uk_deployment_attempt,where:execution_id <> ''"`
	ArtifactID       string `gorm:"size:64;not null;index"`
	ReleaseID        string `gorm:"size:64;index"`
	Environment      string `gorm:"size:32;not null;index"`
	Source           string `gorm:"size:32;not null"`
	Status           string `gorm:"size:32;not null;index"`
	ExternalID       string `gorm:"size:255"`
	ExternalURL      string `gorm:"size:512"`
	Message          string `gorm:"type:text"`
	StartedAt        *time.Time
	FinishedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (DeploymentModel) TableName() string { return "deployments" }

type EnvironmentPolicyModel struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	ProjectID      string `gorm:"size:64;not null;uniqueIndex:uk_project_environment"`
	Environment    string `gorm:"size:32;not null;uniqueIndex:uk_project_environment"`
	AutoDeploy     bool   `gorm:"not null;default:false"`
	Approval       bool   `gorm:"not null;default:false"`
	CompletesTasks bool   `gorm:"not null;default:false"`
}

func (EnvironmentPolicyModel) TableName() string { return "environment_policies" }

type ProjectCITokenModel struct {
	ProjectID string `gorm:"primaryKey;size:64"`
	TokenHash string `gorm:"size:64;not null;uniqueIndex"`
	UpdatedAt time.Time
}

func (ProjectCITokenModel) TableName() string { return "project_ci_tokens" }

type ReleaseJobModel struct {
	ID          string `gorm:"primaryKey;size:64"`
	ReleaseID   string `gorm:"size:64;not null;index"`
	Status      string `gorm:"size:32;not null;index"`
	WorkerID    string `gorm:"size:128;index"`
	Message     string `gorm:"type:text"`
	StartedAt   *time.Time
	FinishedAt  *time.Time
	LeaseUntil  *time.Time `gorm:"index"`
	HeartbeatAt *time.Time
	Attempt     int `gorm:"not null;default:0"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (ReleaseJobModel) TableName() string { return "release_jobs" }

type WorkerSettingModel struct {
	Key       string `gorm:"primaryKey;size:128"`
	Value     string `gorm:"type:text;not null"`
	UpdatedAt time.Time
}

func (WorkerSettingModel) TableName() string { return "worker_settings" }

type WorkerHeartbeatModel struct {
	ID               string `gorm:"primaryKey;size:128"`
	Hostname         string `gorm:"size:255"`
	Status           string `gorm:"size:32;not null;index"`
	Mode             string `gorm:"size:64;not null"`
	PollIntervalMS   int
	CurrentJobID     string `gorm:"size:64;index"`
	CurrentReleaseID string `gorm:"size:64;index"`
	StartedAt        time.Time
	LastSeenAt       time.Time `gorm:"index"`
	UpdatedAt        time.Time
}

func (WorkerHeartbeatModel) TableName() string { return "worker_heartbeats" }

type ReleaseEventModel struct {
	ID          string `gorm:"primaryKey;size:64"`
	ReleaseID   string `gorm:"size:64;not null;index"`
	Type        string `gorm:"size:64;not null;index"`
	Title       string `gorm:"size:160;not null"`
	Message     string `gorm:"type:text"`
	Actor       string `gorm:"size:128"`
	FromStatus  string `gorm:"size:64"`
	ToStatus    string `gorm:"size:64"`
	JobID       string `gorm:"size:64;index"`
	ServiceID   string `gorm:"size:64;index"`
	ServiceName string `gorm:"size:255"`
	CreatedAt   time.Time
}

func (ReleaseEventModel) TableName() string { return "release_events" }

type WorkflowModel struct {
	ID            string `gorm:"primaryKey;size:160"`
	ProjectID     string `gorm:"size:64;not null;index;uniqueIndex:uk_project_workflow"`
	Kind          string `gorm:"size:32;not null;index;uniqueIndex:uk_project_workflow"`
	Name          string `gorm:"size:160;not null"`
	Description   string `gorm:"type:text"`
	InitialStatus string `gorm:"size:64;not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (WorkflowModel) TableName() string { return "project_workflows" }

type WorkflowStatusModel struct {
	ID           string `gorm:"primaryKey;size:128"`
	ProjectID    string `gorm:"size:64;not null;index;uniqueIndex:uk_project_workflow_status"`
	WorkflowKind string `gorm:"size:32;not null;index;uniqueIndex:uk_project_workflow_status"`
	StatusID     string `gorm:"size:64;not null;uniqueIndex:uk_project_workflow_status"`
	Name         string `gorm:"size:160;not null"`
	StageID      string `gorm:"size:64;index"`
	Category     string `gorm:"size:32"`
	OrderIndex   int    `gorm:"not null;index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (WorkflowStatusModel) TableName() string { return "project_workflow_statuses" }

type WorkflowStageModel struct {
	ID           string `gorm:"primaryKey;size:128"`
	ProjectID    string `gorm:"size:64;not null;index;uniqueIndex:uk_project_workflow_stage"`
	WorkflowKind string `gorm:"size:32;not null;index;uniqueIndex:uk_project_workflow_stage"`
	StageID      string `gorm:"size:64;not null;uniqueIndex:uk_project_workflow_stage"`
	Name         string `gorm:"size:160;not null"`
	OrderIndex   int    `gorm:"not null;index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (WorkflowStageModel) TableName() string { return "project_workflow_stages" }

type WorkflowTransitionModel struct {
	ID           string `gorm:"primaryKey;size:128"`
	ProjectID    string `gorm:"size:64;not null;index;uniqueIndex:uk_project_workflow_transition"`
	WorkflowKind string `gorm:"size:32;not null;index;uniqueIndex:uk_project_workflow_transition"`
	TransitionID string `gorm:"size:64;not null;uniqueIndex:uk_project_workflow_transition"`
	Name         string `gorm:"size:160;not null"`
	FromStatus   string `gorm:"size:64;not null;index"`
	ToStatus     string `gorm:"size:64;not null;index"`
	Action       string `gorm:"size:64"`
	OrderIndex   int    `gorm:"not null;index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (WorkflowTransitionModel) TableName() string { return "project_workflow_transitions" }

type WorkflowExecutionModel struct {
	ID                 string     `gorm:"primaryKey;size:64"`
	ReleaseID          string     `gorm:"size:64;not null;uniqueIndex"`
	ProjectID          string     `gorm:"size:64;not null;index"`
	DefinitionName     string     `gorm:"size:128;not null"`
	DefinitionVersion  int        `gorm:"not null"`
	DefinitionSnapshot JSONMap    `gorm:"type:jsonb;not null"`
	PublicStatus       string     `gorm:"size:32;not null;index"`
	Phase              string     `gorm:"size:32;not null;index"`
	CurrentStepID      string     `gorm:"size:128;not null"`
	WakeUpAt           *time.Time `gorm:"index"`
	LastError          string     `gorm:"type:text"`
	Context            JSONMap    `gorm:"type:jsonb;not null"`
	LockVersion        int        `gorm:"not null;default:1"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
	CompletedAt        *time.Time
}

func (WorkflowExecutionModel) TableName() string { return "workflow_executions" }

type WorkflowStepExecutionModel struct {
	ID             string  `gorm:"primaryKey;size:64"`
	ExecutionID    string  `gorm:"size:64;not null;index;uniqueIndex:uk_execution_step_attempt"`
	StepID         string  `gorm:"size:128;not null;index;uniqueIndex:uk_execution_step_attempt"`
	Attempt        int     `gorm:"not null;uniqueIndex:uk_execution_step_attempt"`
	Status         string  `gorm:"size:32;not null;index"`
	IdempotencyKey string  `gorm:"size:255;not null;uniqueIndex"`
	Input          JSONMap `gorm:"type:jsonb;not null"`
	Output         JSONMap `gorm:"type:jsonb;not null"`
	Error          string  `gorm:"type:text"`
	StartedAt      time.Time
	FinishedAt     *time.Time
}

func (WorkflowStepExecutionModel) TableName() string { return "workflow_step_executions" }

type ApprovalTaskModel struct {
	ID          string `gorm:"primaryKey;size:64"`
	ExecutionID string `gorm:"size:64;not null;index;uniqueIndex:uk_execution_step_approver"`
	StepID      string `gorm:"size:128;not null;index;uniqueIndex:uk_execution_step_approver"`
	Approver    string `gorm:"size:255;not null;index;uniqueIndex:uk_execution_step_approver"`
	Decision    string `gorm:"size:32;not null;index"`
	DecidedBy   string `gorm:"size:255"`
	DecidedAt   *time.Time
	CreatedAt   time.Time
}

func (ApprovalTaskModel) TableName() string { return "approval_tasks" }

type WorkflowEventModel struct {
	ID          string  `gorm:"primaryKey;size:64"`
	ExecutionID string  `gorm:"size:64;not null;index;uniqueIndex:uk_execution_sequence"`
	Sequence    int64   `gorm:"not null;uniqueIndex:uk_execution_sequence"`
	Type        string  `gorm:"size:64;not null;index"`
	Payload     JSONMap `gorm:"type:jsonb;not null"`
	CreatedAt   time.Time
}

func (WorkflowEventModel) TableName() string { return "workflow_events" }

type UserModel struct {
	ID           int64  `gorm:"primaryKey;autoIncrement"`
	UserName     string `gorm:"size:255;not null;uniqueIndex"`
	UserNameCN   string `gorm:"size:255;not null"`
	UserEmail    string `gorm:"size:255;not null;uniqueIndex"`
	UserType     int    `gorm:"not null;default:0"`
	UserPassword string `gorm:"size:255;not null"`
	AuthVersion  int    `gorm:"not null;default:1"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (UserModel) TableName() string { return "users" }

type RoleModel struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	RoleName  string `gorm:"size:255;not null;uniqueIndex"`
	RoleDesc  string `gorm:"size:255;not null"`
	IsBuiltin bool   `gorm:"not null;default:false"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (RoleModel) TableName() string { return "sys_roles" }

type RoleMemberModel struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	RoleID      int64  `gorm:"not null;index;uniqueIndex:uk_role_member"`
	MemberEmail string `gorm:"size:255;not null;uniqueIndex:uk_role_member"`
	OnDuty      bool   `gorm:"not null;default:false"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (RoleMemberModel) TableName() string { return "sys_role_members" }

type PrivilegeModel struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	PrivName    string `gorm:"size:255;not null;uniqueIndex"`
	Description string `gorm:"size:255;not null"`
	PrivType    string `gorm:"size:50;not null"`
	Permission  string `gorm:"size:255;not null"`
	IsBuiltin   bool   `gorm:"not null;default:false"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (PrivilegeModel) TableName() string { return "privileges" }

type RolePrivilegeModel struct {
	ID          int64 `gorm:"primaryKey;autoIncrement"`
	RoleID      int64 `gorm:"not null;index;uniqueIndex:uk_role_privilege"`
	PrivilegeID int64 `gorm:"not null;index;uniqueIndex:uk_role_privilege"`
	CreatedAt   time.Time
}

func (RolePrivilegeModel) TableName() string { return "role_privileges" }

type ProjectRoleModel struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	RoleName  string `gorm:"size:255;not null;uniqueIndex"`
	RoleDesc  string `gorm:"size:255;not null"`
	IsBuiltin bool   `gorm:"not null;default:false"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (ProjectRoleModel) TableName() string { return "project_role_defs" }

type ProjectMemberModel struct {
	ID            int64  `gorm:"primaryKey;autoIncrement"`
	ProjectID     string `gorm:"size:64;not null;index;uniqueIndex:uk_project_member"`
	UserID        int64  `gorm:"not null;index;uniqueIndex:uk_project_member"`
	ProjectRoleID int64  `gorm:"not null;index"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (ProjectMemberModel) TableName() string { return "project_members" }
