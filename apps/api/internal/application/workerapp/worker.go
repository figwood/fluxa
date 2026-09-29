package workerapp

import (
	"context"
	"fmt"
	"os"
	"time"

	workerdomain "fluxa-api/internal/domain/worker"
	"fluxa-api/internal/shared"

	"go.uber.org/zap"
)

type Worker struct {
	workers        workerdomain.Repository
	log            *zap.Logger
	interval       time.Duration
	id             string
	hostname       string
	startedAt      time.Time
	workflowRunner interface{ Tick(context.Context) error }
}

func (w *Worker) SetWorkflowRunner(runner interface{ Tick(context.Context) error }) {
	w.workflowRunner = runner
}

func New(id string, workers workerdomain.Repository, log *zap.Logger, interval time.Duration) *Worker {
	if log == nil {
		log = zap.NewNop()
	}
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if id == "" {
		id = shared.NewID("worker")
	}
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "local"
	}
	return &Worker{
		id: id, hostname: hostname, workers: workers,
		log: log, interval: interval, startedAt: time.Now(),
	}
}

func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		w.heartbeat(ctx, "idle", "", "")
		if err := w.tick(ctx); err != nil {
			w.log.Warn("worker tick failed", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (w *Worker) tick(ctx context.Context) error {
	if w.workflowRunner == nil {
		return fmt.Errorf("workflow runner is not configured")
	}
	// Keep the worker itself visible during a long deployment, independently of
	// the execution lease maintained by the workflow engine.
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-workCtx.Done():
				return
			case <-ticker.C:
				w.heartbeat(workCtx, "running", "", "")
			}
		}
	}()
	w.heartbeat(ctx, "running", "", "")
	err := w.workflowRunner.Tick(ctx)
	cancel()
	<-done
	w.heartbeat(ctx, "idle", "", "")
	return err
}

func (w *Worker) heartbeat(ctx context.Context, status, jobID, releaseID string) {
	if w.workers == nil {
		return
	}
	now := time.Now()
	err := w.workers.Heartbeat(ctx, workerdomain.Heartbeat{
		ID: w.id, Hostname: w.hostname, Status: status, Mode: "db-polling",
		PollIntervalMS: int(w.interval / time.Millisecond), CurrentJobID: jobID,
		CurrentReleaseID: releaseID, StartedAt: w.startedAt, LastSeenAt: now, UpdatedAt: now,
	})
	if err != nil {
		w.log.Warn("worker heartbeat failed", zap.Error(err), zap.String("worker_id", w.id))
	}
}
