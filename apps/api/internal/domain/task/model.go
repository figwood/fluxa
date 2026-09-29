package task

import (
	"time"

	"fluxa-api/internal/shared"
)

type Status string

const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusPublishing Status = "publishing"
	StatusPublished  Status = "published"
	StatusDone       Status = "done"
	StatusCancelled  Status = "cancelled"
)

type Task struct {
	ID          string         `json:"id"`
	ProjectID   string         `json:"project_id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Creator     string         `json:"creator"`
	Assignee    string         `json:"assignee"`
	Status      Status         `json:"status"`
	Details     map[string]any `json:"details"`
	Events      []TaskEvent    `json:"events,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type CreateTaskInput struct {
	ProjectID   string         `json:"project_id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Assignee    string         `json:"assignee"`
	Details     map[string]any `json:"details"`
}

type EventType string

const (
	EventTaskCreated       EventType = "task.created"
	EventTaskStatusChanged EventType = "task.status_changed"
)

type TaskEvent struct {
	ID         string    `json:"id"`
	TaskID     string    `json:"task_id"`
	Type       EventType `json:"type"`
	Title      string    `json:"title"`
	Message    string    `json:"message"`
	Actor      string    `json:"actor"`
	FromStatus string    `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	ReleaseID  string    `json:"release_id,omitempty"`
	JobID      string    `json:"job_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func (t Task) CanTransitionTo(next Status) bool {
	if t.Status == StatusCancelled {
		return false
	}
	switch t.Status {
	case StatusTodo:
		return next == StatusInProgress || next == StatusCancelled
	case StatusInProgress:
		return next == StatusPublishing || next == StatusCancelled
	case StatusPublishing:
		return next == StatusPublished || next == StatusCancelled
	case StatusPublished:
		return next == StatusDone
	case StatusDone:
		return false
	default:
		return false
	}
}

func (t Task) TransitionTo(next Status) (Task, error) {
	if !t.CanTransitionTo(next) {
		return t, shared.ErrInvalidTransition
	}
	t.Status = next
	t.UpdatedAt = time.Now()
	return t, nil
}
