package gormdb

import (
	"context"
	"errors"
	"time"

	"fluxa-api/internal/domain/release"
	"fluxa-api/internal/shared"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReleaseRepository struct{ db *gorm.DB }

const jobLeaseDuration = 30 * time.Second

func NewReleaseRepository(db *gorm.DB) *ReleaseRepository { return &ReleaseRepository{db: db} }

func (r *ReleaseRepository) Create(ctx context.Context, rel release.Release) (release.Release, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return createRelease(tx, rel)
	})
	if err != nil {
		return release.Release{}, err
	}
	return r.GetFull(ctx, rel.ID)
}

func (r *ReleaseRepository) CreateWithEvent(ctx context.Context, rel release.Release, event release.ReleaseEvent) (release.Release, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := createRelease(tx, rel); err != nil {
			return err
		}
		if event.ID != "" {
			row := releaseEventModel(event)
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return release.Release{}, err
	}
	return r.GetFull(ctx, rel.ID)
}

func createRelease(tx *gorm.DB, rel release.Release) error {
	row := ReleaseModel{
		ID: rel.ID, ProjectID: rel.ProjectID, Title: rel.Title, Description: rel.Description,
		Environment: rel.Environment, Status: string(rel.Status), Details: JSONMap(rel.Details),
		CreatedAt: rel.CreatedAt, UpdatedAt: rel.UpdatedAt,
	}
	if err := tx.Create(&row).Error; err != nil {
		return err
	}
	for _, rt := range rel.Tasks {
		taskRow := ReleaseTaskModel{
			ID: rt.ID, ReleaseID: rel.ID, TaskID: rt.TaskID, Title: rt.Title, Status: rt.Status, CreatedAt: rt.CreatedAt,
		}
		if err := tx.Create(&taskRow).Error; err != nil {
			return err
		}
	}
	for _, svc := range rel.Services {
		svcRow := releaseServiceModel(svc)
		if err := tx.Create(&svcRow).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *ReleaseRepository) List(ctx context.Context) ([]release.Release, error) {
	var rows []ReleaseModel
	if err := r.db.WithContext(ctx).Order("created_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]release.Release, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRelease(row))
	}
	return out, nil
}

func (r *ReleaseRepository) Get(ctx context.Context, id string) (release.Release, error) {
	var row ReleaseModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return release.Release{}, shared.ErrNotFound
		}
		return release.Release{}, err
	}
	return toRelease(row), nil
}

func (r *ReleaseRepository) GetFull(ctx context.Context, id string) (release.Release, error) {
	rel, err := r.Get(ctx, id)
	if err != nil {
		return release.Release{}, err
	}
	var taskRows []ReleaseTaskModel
	if err := r.db.WithContext(ctx).Where("release_id = ?", id).Order("created_at asc").Find(&taskRows).Error; err != nil {
		return release.Release{}, err
	}
	for _, row := range taskRows {
		rel.Tasks = append(rel.Tasks, release.ReleaseTask{
			ID: row.ID, ReleaseID: row.ReleaseID, TaskID: row.TaskID, Title: row.Title, Status: row.Status, CreatedAt: row.CreatedAt,
		})
	}
	services, err := r.ListServices(ctx, id)
	if err != nil {
		return release.Release{}, err
	}
	rel.Services = services
	events, err := r.ListEvents(ctx, id)
	if err != nil {
		return release.Release{}, err
	}
	rel.Events = events
	return rel, nil
}

func (r *ReleaseRepository) UpdateStatus(ctx context.Context, id string, status release.ReleaseStatus) (release.Release, error) {
	if err := r.db.WithContext(ctx).Model(&ReleaseModel{}).Where("id = ?", id).Updates(map[string]any{
		"status": string(status), "updated_at": time.Now(),
	}).Error; err != nil {
		return release.Release{}, err
	}
	return r.GetFull(ctx, id)
}

func (r *ReleaseRepository) TransitionStatus(ctx context.Context, id string, from release.ReleaseStatus, to release.ReleaseStatus, event release.ReleaseEvent) (release.Release, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ReleaseModel{}).
			Where("id = ? AND status = ?", id, string(from)).
			Updates(map[string]any{"status": string(to), "updated_at": time.Now()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return shared.ErrInvalidTransition
		}
		if event.ID != "" {
			row := releaseEventModel(event)
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return release.Release{}, err
	}
	return r.GetFull(ctx, id)
}

func (r *ReleaseRepository) UpdateService(ctx context.Context, svc release.ReleaseService) error {
	row := releaseServiceModel(svc)
	return r.db.WithContext(ctx).Model(&ReleaseServiceModel{}).Where("id = ?", svc.ID).Updates(map[string]any{
		"status": svc.Status, "external_id": row.ExternalID, "external_url": row.ExternalURL,
		"message": row.Message, "started_at": row.StartedAt, "finished_at": row.FinishedAt, "updated_at": time.Now(),
	}).Error
}

func (r *ReleaseRepository) ListServices(ctx context.Context, releaseID string) ([]release.ReleaseService, error) {
	var rows []ReleaseServiceModel
	if err := r.db.WithContext(ctx).Where("release_id = ?", releaseID).Order("order_index asc, created_at asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]release.ReleaseService, 0, len(rows))
	for _, row := range rows {
		out = append(out, toReleaseService(row))
	}
	return out, nil
}

func (r *ReleaseRepository) CreateJob(ctx context.Context, job release.ReleaseJob) (release.ReleaseJob, error) {
	row := ReleaseJobModel{
		ID: job.ID, ReleaseID: job.ReleaseID, Status: string(job.Status), WorkerID: job.WorkerID, Message: job.Message,
		StartedAt: job.StartedAt, FinishedAt: job.FinishedAt, LeaseUntil: job.LeaseUntil, HeartbeatAt: job.HeartbeatAt, Attempt: job.Attempt, CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return release.ReleaseJob{}, err
	}
	return toReleaseJob(row), nil
}

func (r *ReleaseRepository) Enqueue(ctx context.Context, id string, allowedFrom []release.ReleaseStatus, job release.ReleaseJob, event release.ReleaseEvent) (release.ReleaseJob, bool, error) {
	var out release.ReleaseJob
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rel ReleaseModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&rel).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return shared.ErrNotFound
			}
			return err
		}
		if rel.Status == string(release.ReleaseQueued) || rel.Status == string(release.ReleaseRunning) || rel.Status == string(release.ReleasePublishing) {
			var row ReleaseJobModel
			err := tx.Where("release_id = ?", id).Order("created_at desc").First(&row).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return shared.ErrConflict
			}
			if err != nil {
				return err
			}
			out = toReleaseJob(row)
			return nil
		}
		allowed := false
		for _, status := range allowedFrom {
			if rel.Status == string(status) {
				allowed = true
				break
			}
		}
		if !allowed {
			return shared.ErrInvalidTransition
		}
		now := time.Now()
		if err := tx.Model(&ReleaseModel{}).Where("id = ?", id).Updates(map[string]any{
			"status": string(release.ReleaseQueued), "updated_at": now,
		}).Error; err != nil {
			return err
		}
		row := ReleaseJobModel{
			ID: job.ID, ReleaseID: id, Status: string(job.Status), WorkerID: job.WorkerID, Message: job.Message,
			StartedAt: job.StartedAt, FinishedAt: job.FinishedAt, CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt,
		}
		if row.CreatedAt.IsZero() {
			row.CreatedAt = now
		}
		if row.UpdatedAt.IsZero() {
			row.UpdatedAt = row.CreatedAt
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if event.ID != "" {
			event.JobID = row.ID
			event.FromStatus = rel.Status
			event.ToStatus = string(release.ReleaseQueued)
			event.CreatedAt = row.CreatedAt
			eventRow := releaseEventModel(event)
			if err := tx.Create(&eventRow).Error; err != nil {
				return err
			}
		}
		out = toReleaseJob(row)
		created = true
		return nil
	})
	if err != nil {
		return release.ReleaseJob{}, false, err
	}
	return out, created, nil
}

func (r *ReleaseRepository) ClaimNextJob(ctx context.Context, workerID string) (release.ReleaseJob, bool, error) {
	var claimed release.ReleaseJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row ReleaseJobModel
		locking := clause.Locking{Strength: "UPDATE"}
		if tx.Dialector.Name() == "postgres" {
			locking.Options = "SKIP LOCKED"
		}
		err := tx.Clauses(locking).
			Where("status = ?", string(release.JobPending)).
			Order("created_at asc").
			First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		now := time.Now()
		row.Status = string(release.JobRunning)
		row.WorkerID = workerID
		row.StartedAt = &now
		leaseUntil := now.Add(jobLeaseDuration)
		row.LeaseUntil = &leaseUntil
		row.HeartbeatAt = &now
		row.Attempt++
		row.UpdatedAt = now
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		claimed = toReleaseJob(row)
		return nil
	})
	if err != nil {
		return release.ReleaseJob{}, false, err
	}
	if claimed.ID == "" {
		return release.ReleaseJob{}, false, nil
	}
	return claimed, true, nil
}

func (r *ReleaseRepository) RenewJobLease(ctx context.Context, jobID, workerID string) error {
	now := time.Now()
	leaseUntil := now.Add(jobLeaseDuration)
	result := r.db.WithContext(ctx).Model(&ReleaseJobModel{}).
		Where("id = ? AND worker_id = ? AND status = ?", jobID, workerID, string(release.JobRunning)).
		Updates(map[string]any{"heartbeat_at": now, "lease_until": leaseUntil, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return shared.ErrConflict
	}
	return nil
}

func (r *ReleaseRepository) ReapExpiredJobs(ctx context.Context) ([]release.ReleaseJob, error) {
	now := time.Now()
	var expired []ReleaseJobModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("status = ? AND lease_until IS NOT NULL AND lease_until < ?", string(release.JobRunning), now).Find(&expired).Error; err != nil {
			return err
		}
		for i := range expired {
			result := tx.Model(&ReleaseJobModel{}).Where("id = ? AND status = ? AND lease_until < ?", expired[i].ID, string(release.JobRunning), now).
				Updates(map[string]any{"status": string(release.JobFailed), "message": "worker_lost", "finished_at": now, "updated_at": now})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				expired[i].Status = string(release.JobFailed)
				expired[i].Message = "worker_lost"
				expired[i].FinishedAt = &now
				if err := tx.Model(&ReleaseModel{}).Where("id = ? AND status IN ?", expired[i].ReleaseID, []string{string(release.ReleaseQueued), string(release.ReleaseRunning)}).Updates(map[string]any{"status": string(release.ReleaseFailed), "updated_at": now}).Error; err != nil {
					return err
				}
			} else {
				expired[i].ID = ""
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]release.ReleaseJob, 0, len(expired))
	for _, row := range expired {
		if row.ID != "" {
			out = append(out, toReleaseJob(row))
		}
	}
	return out, nil
}

func (r *ReleaseRepository) GetJob(ctx context.Context, id string) (release.ReleaseJob, error) {
	var row ReleaseJobModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return release.ReleaseJob{}, shared.ErrNotFound
		}
		return release.ReleaseJob{}, err
	}
	return toReleaseJob(row), nil
}

func (r *ReleaseRepository) GetLatestJob(ctx context.Context, releaseID string) (release.ReleaseJob, error) {
	var row ReleaseJobModel
	if err := r.db.WithContext(ctx).Where("release_id = ?", releaseID).Order("created_at desc").First(&row).Error; err != nil {
		return release.ReleaseJob{}, err
	}
	return toReleaseJob(row), nil
}

func (r *ReleaseRepository) UpdateJob(ctx context.Context, job release.ReleaseJob) error {
	query := r.db.WithContext(ctx).Model(&ReleaseJobModel{}).Where("id = ?", job.ID)
	if job.Status == release.JobSuccess || job.Status == release.JobFailed {
		query = query.Where("status = ?", string(release.JobRunning))
	}
	result := query.Updates(map[string]any{
		"status": job.Status, "worker_id": job.WorkerID, "message": job.Message, "started_at": job.StartedAt,
		"finished_at": job.FinishedAt, "lease_until": nil, "heartbeat_at": job.HeartbeatAt, "attempt": job.Attempt, "updated_at": time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return shared.ErrConflict
	}
	return nil
}

func (r *ReleaseRepository) AddEvent(ctx context.Context, event release.ReleaseEvent) error {
	row := releaseEventModel(event)
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *ReleaseRepository) ListEvents(ctx context.Context, releaseID string) ([]release.ReleaseEvent, error) {
	var rows []ReleaseEventModel
	if err := r.db.WithContext(ctx).Where("release_id = ?", releaseID).Order("created_at asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]release.ReleaseEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, toReleaseEvent(row))
	}
	return out, nil
}

func toRelease(row ReleaseModel) release.Release {
	return release.Release{
		ID: row.ID, ProjectID: row.ProjectID, Title: row.Title, Description: row.Description,
		Environment: row.Environment, Status: release.ReleaseStatus(row.Status), Details: map[string]any(row.Details),
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func releaseEventModel(event release.ReleaseEvent) ReleaseEventModel {
	return ReleaseEventModel{
		ID: event.ID, ReleaseID: event.ReleaseID, Type: string(event.Type), Title: event.Title,
		Message: event.Message, Actor: event.Actor, FromStatus: event.FromStatus, ToStatus: event.ToStatus,
		JobID: event.JobID, ServiceID: event.ServiceID, ServiceName: event.ServiceName, CreatedAt: event.CreatedAt,
	}
}

func toReleaseEvent(row ReleaseEventModel) release.ReleaseEvent {
	return release.ReleaseEvent{
		ID: row.ID, ReleaseID: row.ReleaseID, Type: release.ReleaseEventType(row.Type), Title: row.Title,
		Message: row.Message, Actor: row.Actor, FromStatus: row.FromStatus, ToStatus: row.ToStatus,
		JobID: row.JobID, ServiceID: row.ServiceID, ServiceName: row.ServiceName, CreatedAt: row.CreatedAt,
	}
}

func releaseServiceModel(svc release.ReleaseService) ReleaseServiceModel {
	return ReleaseServiceModel{
		ID: svc.ID, ReleaseID: svc.ReleaseID, ProjectServiceID: svc.ProjectServiceID, ArtifactID: svc.ArtifactID,
		ServiceKey: svc.ServiceKey, DisplayName: svc.DisplayName, DeployTarget: svc.DeployTarget,
		DeployConfig: JSONMap(svc.DeployConfig), Ref: svc.Ref, OrderIndex: svc.OrderIndex,
		Status: string(svc.Status), ExternalID: svc.ExternalID, ExternalURL: svc.ExternalURL,
		Message: svc.Message, StartedAt: svc.StartedAt, FinishedAt: svc.FinishedAt,
		CreatedAt: svc.CreatedAt, UpdatedAt: svc.UpdatedAt,
	}
}

func toReleaseService(row ReleaseServiceModel) release.ReleaseService {
	return release.ReleaseService{
		ID: row.ID, ReleaseID: row.ReleaseID, ProjectServiceID: row.ProjectServiceID, ArtifactID: row.ArtifactID,
		ServiceKey: row.ServiceKey, DisplayName: row.DisplayName, DeployTarget: row.DeployTarget,
		DeployConfig: map[string]any(row.DeployConfig), Ref: row.Ref, OrderIndex: row.OrderIndex,
		Status: release.ReleaseServiceStatus(row.Status), ExternalID: row.ExternalID, ExternalURL: row.ExternalURL,
		Message: row.Message, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func toReleaseJob(row ReleaseJobModel) release.ReleaseJob {
	return release.ReleaseJob{
		ID: row.ID, ReleaseID: row.ReleaseID, Status: release.JobStatus(row.Status), Message: row.Message,
		WorkerID: row.WorkerID, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt, LeaseUntil: row.LeaseUntil,
		HeartbeatAt: row.HeartbeatAt, Attempt: row.Attempt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
