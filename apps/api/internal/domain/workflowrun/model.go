package workflowrun

import "time"

type Phase string
type StepStatus string

const (
	PhaseRunning         Phase = "running"
	PhaseWaitingApproval Phase = "waiting_approval"
	PhaseWaitingTime     Phase = "waiting_time"
	PhaseWaitingRetry    Phase = "waiting_retry"
	PhaseCompleted       Phase = "completed"
	PhaseFailed          Phase = "failed"
	PhaseCancelled       Phase = "cancelled"
)

const (
	StepPending StepStatus = "pending"
	StepRunning StepStatus = "running"
	StepSuccess StepStatus = "success"
	StepFailed  StepStatus = "failed"
)

type Definition struct {
	Name    string          `json:"name"`
	Version int             `json:"version"`
	Start   string          `json:"start"`
	Steps   map[string]Step `json:"steps"`
}

type Step struct {
	ID            string         `json:"id,omitempty"`
	Type          string         `json:"type"`
	Expression    string         `json:"expression,omitempty"`
	Next          string         `json:"next,omitempty"`
	OnTrue        string         `json:"on_true,omitempty"`
	OnFalse       string         `json:"on_false,omitempty"`
	OnSuccess     string         `json:"on_success,omitempty"`
	OnFailure     string         `json:"on_failure,omitempty"`
	Action        string         `json:"action,omitempty"`
	Params        map[string]any `json:"params,omitempty"`
	Approvers     []string       `json:"approvers,omitempty"`
	Policy        string         `json:"policy,omitempty"`
	Quorum        int            `json:"quorum,omitempty"`
	Until         string         `json:"until,omitempty"`
	MaxAttempts   int            `json:"max_attempts,omitempty"`
	RetryDelaySec int            `json:"retry_delay_seconds,omitempty"`
}

type Execution struct {
	ID                 string          `json:"id"`
	ReleaseID          string          `json:"release_id"`
	ProjectID          string          `json:"project_id"`
	DefinitionName     string          `json:"definition_name"`
	DefinitionVersion  int             `json:"definition_version"`
	DefinitionSnapshot Definition      `json:"definition_snapshot"`
	PublicStatus       string          `json:"public_status"`
	Phase              Phase           `json:"phase"`
	CurrentStepID      string          `json:"current_step_id"`
	WakeUpAt           *time.Time      `json:"wake_up_at,omitempty"`
	LastError          string          `json:"last_error"`
	Context            map[string]any  `json:"context"`
	LockVersion        int             `json:"lock_version"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	CompletedAt        *time.Time      `json:"completed_at,omitempty"`
	Steps              []StepExecution `json:"step_executions,omitempty"`
	Approvals          []ApprovalTask  `json:"approvals,omitempty"`
	Events             []Event         `json:"events,omitempty"`
}

type StepExecution struct {
	ID             string         `json:"id"`
	ExecutionID    string         `json:"execution_id"`
	StepID         string         `json:"step_id"`
	Attempt        int            `json:"attempt"`
	Status         StepStatus     `json:"status"`
	IdempotencyKey string         `json:"idempotency_key"`
	Input          map[string]any `json:"input"`
	Output         map[string]any `json:"output"`
	Error          string         `json:"error"`
	StartedAt      time.Time      `json:"started_at"`
	FinishedAt     *time.Time     `json:"finished_at,omitempty"`
}

type ApprovalTask struct {
	ID          string     `json:"id"`
	ExecutionID string     `json:"execution_id"`
	StepID      string     `json:"step_id"`
	Approver    string     `json:"approver"`
	Decision    string     `json:"decision"`
	DecidedBy   string     `json:"decided_by"`
	DecidedAt   *time.Time `json:"decided_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Event struct {
	ID          string         `json:"id"`
	ExecutionID string         `json:"execution_id"`
	Sequence    int64          `json:"sequence"`
	Type        string         `json:"type"`
	Payload     map[string]any `json:"payload"`
	CreatedAt   time.Time      `json:"created_at"`
}

type StartInput struct {
	Definition *Definition    `json:"definition,omitempty"`
	Context    map[string]any `json:"context,omitempty"`
}

func DefaultReleaseDefinition() Definition {
	return Definition{Name: "default_release", Version: 1, Start: "publish_stg", Steps: map[string]Step{
		"publish_stg":  {Type: "action", Action: "publish_environment", Params: map[string]any{"environment": "stg"}, OnSuccess: "publish_prod", MaxAttempts: 3, RetryDelaySec: 5},
		"publish_prod": {Type: "action", Action: "publish_environment", Params: map[string]any{"environment": "prod"}, OnSuccess: "completed", MaxAttempts: 2, RetryDelaySec: 10},
		"completed":    {Type: "complete"},
	}}
}
