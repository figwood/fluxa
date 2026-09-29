package gormdb

import (
	"context"
	"errors"
	"strconv"
	"time"

	"fluxa-api/internal/domain/worker"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const workerDesiredReplicasKey = "desired_replicas"

type WorkerRepository struct {
	db             *gorm.DB
	defaultDesired int
	pollInterval   time.Duration
	mode           string
}

func NewWorkerRepository(db *gorm.DB, defaultDesired int, pollInterval time.Duration) *WorkerRepository {
	if defaultDesired < 1 {
		defaultDesired = 1
	}
	return &WorkerRepository{db: db, defaultDesired: defaultDesired, pollInterval: pollInterval, mode: "db-polling"}
}

func (r *WorkerRepository) GetConfig(ctx context.Context) (worker.RuntimeConfig, error) {
	desired := r.defaultDesired
	var row WorkerSettingModel
	err := r.db.WithContext(ctx).Where("key = ?", workerDesiredReplicasKey).First(&row).Error
	if err == nil {
		if parsed, parseErr := strconv.Atoi(row.Value); parseErr == nil {
			desired = parsed
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return worker.RuntimeConfig{}, err
	}
	return r.runtimeConfig(desired), nil
}

func (r *WorkerRepository) UpdateDesiredReplicas(ctx context.Context, replicas int) (worker.RuntimeConfig, error) {
	now := time.Now()
	row := WorkerSettingModel{Key: workerDesiredReplicasKey, Value: strconv.Itoa(replicas), UpdatedAt: now}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&row).Error
	if err != nil {
		return worker.RuntimeConfig{}, err
	}
	return r.runtimeConfig(replicas), nil
}

func (r *WorkerRepository) Heartbeat(ctx context.Context, heartbeat worker.Heartbeat) error {
	row := WorkerHeartbeatModel{
		ID: heartbeat.ID, Hostname: heartbeat.Hostname, Status: heartbeat.Status, Mode: heartbeat.Mode,
		PollIntervalMS: heartbeat.PollIntervalMS, CurrentJobID: heartbeat.CurrentJobID,
		CurrentReleaseID: heartbeat.CurrentReleaseID, StartedAt: heartbeat.StartedAt,
		LastSeenAt: heartbeat.LastSeenAt, UpdatedAt: heartbeat.UpdatedAt,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"hostname", "status", "mode", "poll_interval_ms", "current_job_id", "current_release_id",
			"started_at", "last_seen_at", "updated_at",
		}),
	}).Create(&row).Error
}

func (r *WorkerRepository) ListHeartbeats(ctx context.Context) ([]worker.Heartbeat, error) {
	var rows []WorkerHeartbeatModel
	if err := r.db.WithContext(ctx).Order("last_seen_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]worker.Heartbeat, 0, len(rows))
	for _, row := range rows {
		out = append(out, toWorkerHeartbeat(row))
	}
	return out, nil
}

// Legacy jobs remain visible only when no workflow owns their release.
const workerJobProjection = `SELECT id, release_id,
 CASE WHEN phase = 'completed' THEN 'success'
 WHEN phase IN ('failed','waiting_retry','cancelled') THEN 'failed'
 WHEN phase IN ('waiting_time','waiting_approval') THEN 'pending'
 ELSE 'running' END AS status,
 '' AS worker_id, last_error AS message, created_at AS started_at,
 completed_at AS finished_at, created_at, updated_at FROM workflow_executions
 UNION ALL SELECT j.id, j.release_id, j.status, j.worker_id, j.message,
 j.started_at, j.finished_at, j.created_at, j.updated_at FROM release_jobs j
 WHERE NOT EXISTS (SELECT 1 FROM workflow_executions e WHERE e.release_id = j.release_id)`

func (r *WorkerRepository) ListJobs(ctx context.Context, limit int) ([]worker.JobSnapshot, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows := []worker.JobSnapshot{}
	err := r.db.WithContext(ctx).Raw("SELECT * FROM ("+workerJobProjection+") jobs ORDER BY created_at DESC LIMIT ?", limit).Scan(&rows).Error
	return rows, err
}
func (r *WorkerRepository) QueueStats(ctx context.Context) (worker.QueueStats, error) {
	var rows []struct {
		Status string
		Count  int64
	}
	err := r.db.WithContext(ctx).Raw("SELECT status, COUNT(*) AS count FROM (" + workerJobProjection + ") jobs GROUP BY status").Scan(&rows).Error
	stats := worker.QueueStats{}
	for _, row := range rows {
		switch row.Status {
		case "pending":
			stats.Pending = row.Count
		case "running":
			stats.Running = row.Count
		case "success":
			stats.Success = row.Count
		case "failed":
			stats.Failed = row.Count
		}
	}
	return stats, err
}

func (r *WorkerRepository) runtimeConfig(desired int) worker.RuntimeConfig {
	if desired < 1 {
		desired = 1
	}
	return worker.RuntimeConfig{
		DesiredReplicas: desired,
		PollInterval:    r.pollInterval,
		PollIntervalMS:  int(r.pollInterval / time.Millisecond),
		Mode:            r.mode,
	}
}

func toWorkerHeartbeat(row WorkerHeartbeatModel) worker.Heartbeat {
	return worker.Heartbeat{
		ID: row.ID, Hostname: row.Hostname, Status: row.Status, Mode: row.Mode,
		PollIntervalMS: row.PollIntervalMS, CurrentJobID: row.CurrentJobID,
		CurrentReleaseID: row.CurrentReleaseID, StartedAt: row.StartedAt,
		LastSeenAt: row.LastSeenAt, UpdatedAt: row.UpdatedAt,
	}
}
