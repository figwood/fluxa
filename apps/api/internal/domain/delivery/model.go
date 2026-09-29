package delivery

import "time"

type Artifact struct {
	ID               string    `json:"id"`
	ProjectID        string    `json:"project_id"`
	ProjectServiceID string    `json:"project_service_id"`
	CommitSHA        string    `json:"commit_sha"`
	RefType          string    `json:"ref_type"`
	Ref              string    `json:"ref"`
	ImageRepository  string    `json:"image_repository"`
	ImageTag         string    `json:"image_tag"`
	ImageDigest      string    `json:"image_digest"`
	PipelineID       string    `json:"pipeline_id"`
	PipelineURL      string    `json:"pipeline_url"`
	IdempotencyKey   string    `json:"idempotency_key"`
	BuiltAt          time.Time `json:"built_at"`
	CreatedAt        time.Time `json:"created_at"`
}

type Deployment struct {
	ExecutionID      string     `json:"execution_id,omitempty"`
	StepID           string     `json:"step_id,omitempty"`
	Attempt          int        `json:"attempt"`
	ID               string     `json:"id"`
	ProjectID        string     `json:"project_id"`
	ProjectServiceID string     `json:"project_service_id"`
	ArtifactID       string     `json:"artifact_id"`
	ReleaseID        string     `json:"release_id,omitempty"`
	Environment      string     `json:"environment"`
	Source           string     `json:"source"`
	Status           string     `json:"status"`
	ExternalID       string     `json:"external_id"`
	ExternalURL      string     `json:"external_url"`
	Message          string     `json:"message"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type EnvironmentPolicy struct {
	ProjectID      string `json:"project_id"`
	Environment    string `json:"environment"`
	AutoDeploy     bool   `json:"auto_deploy"`
	Approval       bool   `json:"approval_required"`
	CompletesTasks bool   `json:"completes_tasks"`
}

type ReportArtifactInput struct {
	ProjectServiceID string    `json:"project_service_id"`
	CommitSHA        string    `json:"commit_sha"`
	RefType          string    `json:"ref_type"`
	Ref              string    `json:"ref"`
	ImageRepository  string    `json:"image_repository"`
	ImageTag         string    `json:"image_tag"`
	ImageDigest      string    `json:"image_digest"`
	PipelineID       string    `json:"pipeline_id"`
	PipelineURL      string    `json:"pipeline_url"`
	IdempotencyKey   string    `json:"idempotency_key"`
	BuiltAt          time.Time `json:"built_at"`
}

type ReportResult struct {
	Artifact   Artifact    `json:"artifact"`
	Deployment *Deployment `json:"deployment,omitempty"`
}
