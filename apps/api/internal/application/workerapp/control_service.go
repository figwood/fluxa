package workerapp

import (
	"context"
	"time"

	workerdomain "fluxa-api/internal/domain/worker"
	"fluxa-api/internal/shared"
)

type ControlService struct {
	workers workerdomain.Repository
}

func NewControl(workers workerdomain.Repository) *ControlService {
	return &ControlService{workers: workers}
}

func (s *ControlService) Snapshot(ctx context.Context) (workerdomain.Snapshot, error) {
	cfg, err := s.workers.GetConfig(ctx)
	if err != nil {
		return workerdomain.Snapshot{}, err
	}
	stats, err := s.workers.QueueStats(ctx)
	if err != nil {
		return workerdomain.Snapshot{}, err
	}
	heartbeats, err := s.workers.ListHeartbeats(ctx)
	if err != nil {
		return workerdomain.Snapshot{}, err
	}
	jobs, err := s.workers.ListJobs(ctx, 50)
	if err != nil {
		return workerdomain.Snapshot{}, err
	}

	staleAfter := time.Now().Add(-staleWindow(cfg.PollInterval))
	for i := range heartbeats {
		heartbeats[i].StaleAfter = &staleAfter
		if heartbeats[i].LastSeenAt.Before(staleAfter) {
			heartbeats[i].Stale = true
			heartbeats[i].Status = "offline"
		}
	}

	return workerdomain.Snapshot{Config: cfg, Stats: stats, Workers: heartbeats, Jobs: jobs}, nil
}

func (s *ControlService) UpdateDesiredReplicas(ctx context.Context, replicas int) (workerdomain.Snapshot, error) {
	if replicas < 1 || replicas > 20 {
		return workerdomain.Snapshot{}, shared.ErrInvalidInput
	}
	if _, err := s.workers.UpdateDesiredReplicas(ctx, replicas); err != nil {
		return workerdomain.Snapshot{}, err
	}
	return s.Snapshot(ctx)
}

func staleWindow(interval time.Duration) time.Duration {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	window := interval * 3
	if window < 10*time.Second {
		return 10 * time.Second
	}
	return window
}
