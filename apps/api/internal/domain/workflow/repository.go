package workflow

import "context"

type Repository interface {
	EnsureDefaults(ctx context.Context) error
	List(ctx context.Context, projectID string) ([]Workflow, error)
	Get(ctx context.Context, projectID string, kind Kind) (Workflow, error)
	Save(ctx context.Context, item Workflow) (Workflow, error)
	AllowedTransitions(ctx context.Context, projectID string, kind Kind, fromStatus string) ([]Transition, error)
}
