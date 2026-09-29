package task

import "context"

type Repository interface {
	Create(ctx context.Context, item Task) (Task, error)
	CreateWithEvent(ctx context.Context, item Task, event TaskEvent) (Task, error)
	List(ctx context.Context, projectID string, status Status) ([]Task, error)
	Get(ctx context.Context, id string) (Task, error)
	GetFull(ctx context.Context, id string) (Task, error)
	UpdateStatus(ctx context.Context, id string, status Status) (Task, error)
	TransitionStatus(ctx context.Context, id string, from, to Status, event TaskEvent) (Task, error)
	UpdateAssignee(ctx context.Context, id string, assignee string) (Task, error)
	BulkUpdateStatus(ctx context.Context, ids []string, status Status) error
	BulkUpdateStatusWithEvents(ctx context.Context, ids []string, status Status, actor, releaseID, jobID string) error
	AddEvent(ctx context.Context, event TaskEvent) error
	ListEvents(ctx context.Context, taskID string) ([]TaskEvent, error)
}
