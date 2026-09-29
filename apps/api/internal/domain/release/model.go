package release

import (
	"time"

	"fluxa-api/internal/shared"
)

type ReleaseStatus string
type ReleaseServiceStatus string
type JobStatus string

const (
	ReleaseDraft           ReleaseStatus = "draft"
	ReleasePendingApproval ReleaseStatus = "pending_approval"
	ReleaseApproved        ReleaseStatus = "approved"
	ReleaseQueued          ReleaseStatus = "queued"
	ReleaseRunning         ReleaseStatus = "running"
	ReleasePublishing      ReleaseStatus = "publishing"
	ReleaseSuccess         ReleaseStatus = "success"
	ReleaseFailed          ReleaseStatus = "failed"
	ReleaseCancelled       ReleaseStatus = "cancelled"
)

const (
	ServicePending ReleaseServiceStatus = "pending"
	ServiceRunning ReleaseServiceStatus = "running"
	ServiceSuccess ReleaseServiceStatus = "success"
	ServiceFailed  ReleaseServiceStatus = "failed"
	ServiceSkipped ReleaseServiceStatus = "skipped"
)

const (
	JobPending JobStatus = "pending"
	JobRunning JobStatus = "running"
	JobSuccess JobStatus = "success"
	JobFailed  JobStatus = "failed"
)

type Release struct {
	ID          string           `json:"id"`
	ProjectID   string           `json:"project_id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Environment string           `json:"-"`
	Status      ReleaseStatus    `json:"status"`
	Details     map[string]any   `json:"details"`
	Tasks       []ReleaseTask    `json:"tasks,omitempty"`
	Services    []ReleaseService `json:"services,omitempty"`
	Events      []ReleaseEvent   `json:"events,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type ReleaseTask struct {
	ID        string    `json:"id"`
	ReleaseID string    `json:"release_id"`
	TaskID    string    `json:"task_id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type ReleaseService struct {
	ID               string               `json:"id"`
	ReleaseID        string               `json:"release_id"`
	ProjectServiceID string               `json:"project_service_id"`
	ArtifactID       string               `json:"artifact_id"`
	ServiceKey       string               `json:"service_key"`
	DisplayName      string               `json:"display_name"`
	DeployTarget     string               `json:"deploy_target"`
	DeployConfig     map[string]any       `json:"deploy_config"`
	Ref              string               `json:"ref"`
	OrderIndex       int                  `json:"order_index"`
	Status           ReleaseServiceStatus `json:"status"`
	ExternalID       string               `json:"external_id"`
	ExternalURL      string               `json:"external_url"`
	Message          string               `json:"message"`
	StartedAt        *time.Time           `json:"started_at,omitempty"`
	FinishedAt       *time.Time           `json:"finished_at,omitempty"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

type ReleaseJob struct {
	ID          string     `json:"id"`
	ReleaseID   string     `json:"release_id"`
	Status      JobStatus  `json:"status"`
	WorkerID    string     `json:"worker_id"`
	Message     string     `json:"message"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	LeaseUntil  *time.Time `json:"lease_until,omitempty"`
	HeartbeatAt *time.Time `json:"heartbeat_at,omitempty"`
	Attempt     int        `json:"attempt"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ReleaseEventType string

const (
	EventReleaseCreated       ReleaseEventType = "release.created"
	EventReleaseStatusChanged ReleaseEventType = "release.status_changed"
	EventReleaseSubmitted     ReleaseEventType = "release.submitted"
	EventReleaseApproved      ReleaseEventType = "release.approved"
	EventReleasePublishQueued ReleaseEventType = "release.publish_queued"
	EventReleaseSucceeded     ReleaseEventType = "release.succeeded"
	EventReleaseFailed        ReleaseEventType = "release.failed"
	EventJobClaimed           ReleaseEventType = "job.claimed"
	EventServiceStarted       ReleaseEventType = "service.started"
	EventServiceSucceeded     ReleaseEventType = "service.succeeded"
	EventServiceFailed        ReleaseEventType = "service.failed"
	EventServiceSkipped       ReleaseEventType = "service.skipped"
)

type ReleaseEvent struct {
	ID          string           `json:"id"`
	ReleaseID   string           `json:"release_id"`
	Type        ReleaseEventType `json:"type"`
	Title       string           `json:"title"`
	Message     string           `json:"message"`
	Actor       string           `json:"actor"`
	FromStatus  string           `json:"from_status"`
	ToStatus    string           `json:"to_status"`
	JobID       string           `json:"job_id"`
	ServiceID   string           `json:"service_id"`
	ServiceName string           `json:"service_name"`
	CreatedAt   time.Time        `json:"created_at"`
}

type CreateReleaseInput struct {
	ProjectID   string                  `json:"project_id"`
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	Environment string                  `json:"-"`
	Details     map[string]any          `json:"details"`
	TaskIDs     []string                `json:"task_ids"`
	Services    []CreateReleaseSvcInput `json:"services"`
}

type CreateReleaseSvcInput struct {
	ProjectServiceID string `json:"project_service_id"`
	ArtifactID       string `json:"artifact_id"`
	Ref              string `json:"ref"`
	OrderIndex       int    `json:"order_index"`
}

func (r Release) TransitionTo(next ReleaseStatus) (Release, error) {
	allowed := false
	switch r.Status {
	case ReleaseDraft:
		allowed = next == ReleasePendingApproval || next == ReleaseCancelled
	case ReleasePendingApproval:
		allowed = next == ReleaseApproved || next == ReleaseCancelled
	case ReleaseApproved:
		allowed = next == ReleaseQueued || next == ReleasePublishing || next == ReleaseCancelled
	case ReleaseQueued:
		allowed = next == ReleaseRunning || next == ReleaseCancelled
	case ReleaseRunning, ReleasePublishing:
		allowed = next == ReleaseSuccess || next == ReleaseFailed
	}
	if !allowed {
		return r, shared.ErrInvalidTransition
	}
	r.Status = next
	r.UpdatedAt = time.Now()
	return r, nil
}
