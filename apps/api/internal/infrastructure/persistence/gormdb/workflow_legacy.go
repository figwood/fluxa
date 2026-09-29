package gormdb

import (
	"context"
	"fluxa-api/internal/domain/workflowrun"
	"fluxa-api/internal/shared"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// Adopt one legacy release under its row lock. Never run old and new workers
// together during upgrade. Unknown old side effects have no idempotency key:
// require operator reconciliation before resuming them.
func (r *WorkflowRunRepository) AdoptLegacy(ctx context.Context) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []ReleaseModel
		query := tx.Where("status IN ? AND NOT EXISTS (SELECT 1 FROM workflow_executions e WHERE e.release_id = releases.id) AND EXISTS (SELECT 1 FROM release_jobs j WHERE j.release_id = releases.id)", []string{"queued", "running", "publishing", "failed"})
		lock := clause.Locking{Strength: "UPDATE"}
		if tx.Dialector.Name() == "postgres" {
			lock.Options = "SKIP LOCKED"
		}
		if err := query.Clauses(lock).Order("updated_at asc").Limit(1).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		rel := rows[0]
		var jobs []ReleaseJobModel
		if err := tx.Where("release_id = ?", rel.ID).Order("created_at desc").Find(&jobs).Error; err != nil {
			return err
		}
		now := time.Now()
		uncertain := rel.Status == "failed"
		for _, job := range jobs {
			if job.Status == "running" && job.LeaseUntil != nil && job.LeaseUntil.After(now) {
				return nil
			}
			if job.Status != "pending" {
				uncertain = true
			}
		}
		def := workflowrun.DefaultReleaseDefinition()
		item := workflowrun.Execution{ID: shared.NewID("wfexec"), ReleaseID: rel.ID, ProjectID: rel.ProjectID, DefinitionName: def.Name, DefinitionVersion: def.Version, DefinitionSnapshot: def, PublicStatus: "publishing", Phase: workflowrun.PhaseRunning, CurrentStepID: def.Start, Context: map[string]any{"legacy_job_id": jobs[0].ID, "deployment_tracking_v1": true}, LockVersion: 1, CreatedAt: now, UpdatedAt: now}
		if uncertain {
			item.Phase = workflowrun.PhaseWaitingRetry
			item.PublicStatus = "failed"
			item.LastError = "旧作业结果不确定，请先核对外部部署结果，再手动重试；旧执行没有幂等标识。"
		}
		row, err := workflowExecutionModel(item)
		if err != nil {
			return err
		}
		if err = tx.Create(&row).Error; err != nil {
			return err
		}
		if err = syncRelease(tx, item, "execution.migrated"); err != nil {
			return err
		}
		return createWorkflowEvent(tx, workflowrun.Event{ExecutionID: item.ID, Type: "execution.migrated", Payload: map[string]any{"requires_reconciliation": uncertain}})
	})
}
