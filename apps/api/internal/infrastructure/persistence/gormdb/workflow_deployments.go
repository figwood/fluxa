package gormdb

import (
	"context"
	"errors"
	"fluxa-api/internal/domain/delivery"
	"fluxa-api/internal/domain/workflowrun"
	"fluxa-api/internal/shared"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

func (r *WorkflowRunRepository) RenewLease(ctx context.Context, item workflowrun.Execution) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&WorkflowExecutionModel{}).Where("id = ? AND lock_version = ? AND phase = 'running' AND wake_up_at > ?", item.ID, item.LockVersion, now).Update("wake_up_at", now.Add(r.leaseDuration))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return shared.ErrConflict
	}
	return nil
}

// Lock the execution before touching any deployment or service; cancellation and
// another lease holder cannot commit side effects through an obsolete worker.
func lockExecution(tx *gorm.DB, item workflowrun.Execution) error {
	var row WorkflowExecutionModel
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", item.ID).First(&row).Error; err != nil {
		return mapNotFound(err)
	}
	if row.LockVersion != item.LockVersion || row.Phase != "running" || row.WakeUpAt == nil || !row.WakeUpAt.After(time.Now()) {
		return shared.ErrConflict
	}
	return nil
}

func (r *WorkflowRunRepository) BeginDeployment(ctx context.Context, item workflowrun.Execution, dep delivery.Deployment) (delivery.Deployment, error) {
	var out DeploymentModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockExecution(tx, item); err != nil {
			return err
		}
		err := tx.Where("execution_id = ? AND step_id = ? AND project_service_id = ?", item.ID, item.CurrentStepID, dep.ProjectServiceID).Order("attempt DESC").First(&out).Error
		if err == nil && out.Status != "failed" {
			return nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		attempt := out.Attempt + 1
		dep.ExecutionID = item.ID
		dep.StepID = item.CurrentStepID
		dep.Attempt = attempt
		dep.ID = shared.NewID("dep")
		now := time.Now()
		dep.Status = "running"
		dep.CreatedAt = now
		dep.UpdatedAt = now
		dep.StartedAt = &now
		out = deploymentModel(dep)
		if err := tx.Create(&out).Error; err != nil {
			return err
		}
		return tx.Model(&ReleaseServiceModel{}).Where("release_id = ? AND project_service_id = ?", item.ReleaseID, dep.ProjectServiceID).Updates(map[string]any{"status": "running", "started_at": now, "finished_at": nil, "message": "", "updated_at": now}).Error
	})
	return deployment(out), err
}

func (r *WorkflowRunRepository) FinishDeployment(ctx context.Context, item workflowrun.Execution, dep delivery.Deployment) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockExecution(tx, item); err != nil {
			return err
		}
		if dep.Status != "success" && dep.Status != "failed" {
			return shared.ErrInvalidInput
		}
		row := deploymentModel(dep)
		result := tx.Model(&DeploymentModel{}).Where("id = ? AND execution_id = ? AND status = 'running'", dep.ID, item.ID).Updates(map[string]any{"status": row.Status, "external_id": row.ExternalID, "external_url": row.ExternalURL, "message": row.Message, "finished_at": row.FinishedAt, "updated_at": time.Now()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return shared.ErrConflict
		}
		if err := tx.Model(&ReleaseServiceModel{}).Where("release_id = ? AND project_service_id = ?", item.ReleaseID, dep.ProjectServiceID).Updates(map[string]any{"status": dep.Status, "external_id": dep.ExternalID, "external_url": dep.ExternalURL, "message": dep.Message, "finished_at": dep.FinishedAt, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		return tx.Create(&ReleaseEventModel{ID: shared.NewID("event"), ReleaseID: item.ReleaseID, Type: "service." + dep.Status, Title: dep.Environment + " deployment " + dep.Status, Message: dep.Message, Actor: "worker", CreatedAt: time.Now()}).Error
	})
}
