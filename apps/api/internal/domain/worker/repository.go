package worker

import "context"

type Repository interface {
	GetConfig(ctx context.Context) (RuntimeConfig, error)
	UpdateDesiredReplicas(ctx context.Context, replicas int) (RuntimeConfig, error)
	Heartbeat(ctx context.Context, heartbeat Heartbeat) error
	ListHeartbeats(ctx context.Context) ([]Heartbeat, error)
	ListJobs(ctx context.Context, limit int) ([]JobSnapshot, error)
	QueueStats(ctx context.Context) (QueueStats, error)
}
