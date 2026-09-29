package worker

import "time"

type RuntimeConfig struct {
	DesiredReplicas int           `json:"desired_replicas"`
	PollInterval    time.Duration `json:"-"`
	PollIntervalMS  int           `json:"poll_interval_ms"`
	Mode            string        `json:"mode"`
}

type Heartbeat struct {
	ID               string     `json:"id"`
	Hostname         string     `json:"hostname"`
	Status           string     `json:"status"`
	Mode             string     `json:"mode"`
	PollIntervalMS   int        `json:"poll_interval_ms"`
	CurrentJobID     string     `json:"current_job_id"`
	CurrentReleaseID string     `json:"current_release_id"`
	StartedAt        time.Time  `json:"started_at"`
	LastSeenAt       time.Time  `json:"last_seen_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Stale            bool       `json:"stale"`
	StaleAfter       *time.Time `json:"stale_after,omitempty"`
}

type Snapshot struct {
	Config  RuntimeConfig `json:"config"`
	Stats   QueueStats    `json:"stats"`
	Workers []Heartbeat   `json:"workers"`
	Jobs    []JobSnapshot `json:"jobs"`
}

type QueueStats struct {
	Pending int64 `json:"pending"`
	Running int64 `json:"running"`
	Success int64 `json:"success"`
	Failed  int64 `json:"failed"`
}

type JobSnapshot struct {
	ID         string     `json:"id"`
	ReleaseID  string     `json:"release_id"`
	Status     string     `json:"status"`
	WorkerID   string     `json:"worker_id"`
	Message    string     `json:"message"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
