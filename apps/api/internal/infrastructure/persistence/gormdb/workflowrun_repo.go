package gormdb

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"fluxa-api/internal/domain/task"
	"fluxa-api/internal/domain/workflowrun"
	"fluxa-api/internal/shared"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorkflowRunRepository struct {
	db            *gorm.DB
	leaseDuration time.Duration
}

func NewWorkflowRunRepository(db *gorm.DB, leaseDuration ...time.Duration) *WorkflowRunRepository {
	d := 30 * time.Second
	if len(leaseDuration) > 0 && leaseDuration[0] > 0 {
		d = leaseDuration[0]
	}
	return &WorkflowRunRepository{db: db, leaseDuration: d}
}

func (r *WorkflowRunRepository) Create(ctx context.Context, item workflowrun.Execution) (workflowrun.Execution, error) {
	row, err := workflowExecutionModel(item)
	if err != nil {
		return item, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rel ReleaseModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", item.ReleaseID).First(&rel).Error; err != nil {
			return mapNotFound(err)
		}
		var existing WorkflowExecutionModel
		err := tx.Where("release_id = ?", item.ReleaseID).First(&existing).Error
		if err == nil {
			row = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if rel.Status != "draft" && rel.Status != "approved" {
			return shared.ErrInvalidTransition
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if err := syncRelease(tx, item, "execution.started"); err != nil {
			return err
		}
		return createWorkflowEvent(tx, workflowrun.Event{ExecutionID: item.ID, Type: "execution.started", Payload: map[string]any{"step_id": item.CurrentStepID}})
	})
	if err != nil {
		return workflowrun.Execution{}, err
	}
	return r.Get(ctx, row.ID)
}

func (r *WorkflowRunRepository) Get(ctx context.Context, id string) (workflowrun.Execution, error) {
	var row WorkflowExecutionModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return workflowrun.Execution{}, mapNotFound(err)
	}
	return r.full(ctx, row)
}

func (r *WorkflowRunRepository) GetByRelease(ctx context.Context, releaseID string) (workflowrun.Execution, error) {
	var row WorkflowExecutionModel
	if err := r.db.WithContext(ctx).Where("release_id = ?", releaseID).First(&row).Error; err != nil {
		return workflowrun.Execution{}, mapNotFound(err)
	}
	return r.full(ctx, row)
}

func (r *WorkflowRunRepository) ClaimNext(ctx context.Context) (workflowrun.Execution, bool, error) {
	var out WorkflowExecutionModel
	now := time.Now()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		locking := clause.Locking{Strength: "UPDATE"}
		if tx.Dialector.Name() == "postgres" {
			locking.Options = "SKIP LOCKED"
		}
		err := tx.Clauses(locking).Where("phase IN ? AND (wake_up_at IS NULL OR wake_up_at <= ?)", []string{string(workflowrun.PhaseRunning), string(workflowrun.PhaseWaitingTime)}, now).Order("updated_at asc").First(&out).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		// Old executions predate persisted external operation keys. Never replay
		// an interrupted action automatically during upgrade.
		if out.Context["deployment_tracking_v1"] != true {
			out.Phase = "waiting_retry"
			out.PublicStatus = "failed"
			out.LastError = "旧执行没有持久化部署标识，请核对外部部署结果后手动重试。"
			out.WakeUpAt = nil
			out.LockVersion++
			if err := tx.Model(&WorkflowExecutionModel{}).Where("id = ?", out.ID).Updates(map[string]any{"phase": out.Phase, "public_status": out.PublicStatus, "last_error": out.LastError, "wake_up_at": nil, "lock_version": out.LockVersion, "updated_at": now}).Error; err != nil {
				return err
			}
			item, err := toWorkflowExecution(out)
			if err != nil {
				return err
			}
			if err := syncRelease(tx, item, "execution.reconciliation_required"); err != nil {
				return err
			}
			if err := createWorkflowEvent(tx, workflowrun.Event{ExecutionID: out.ID, Type: "execution.reconciliation_required", Payload: map[string]any{}}); err != nil {
				return err
			}
			out.ID = ""
			return nil
		}
		lease := now.Add(r.leaseDuration)
		result := tx.Model(&WorkflowExecutionModel{}).Where("id = ? AND lock_version = ?", out.ID, out.LockVersion).Updates(map[string]any{"phase": string(workflowrun.PhaseRunning), "wake_up_at": lease, "lock_version": gorm.Expr("lock_version + 1"), "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return shared.ErrConflict
		}
		out.LockVersion++
		out.Phase = string(workflowrun.PhaseRunning)
		out.WakeUpAt = &lease
		return nil
	})
	if err != nil {
		return workflowrun.Execution{}, false, err
	}
	if out.ID == "" {
		return workflowrun.Execution{}, false, nil
	}
	item, err := r.full(ctx, out)
	return item, err == nil, err
}

func (r *WorkflowRunRepository) Advance(ctx context.Context, item workflowrun.Execution, step workflowrun.StepExecution, event workflowrun.Event) error {
	return r.persist(ctx, item, step, nil, event)
}

func (r *WorkflowRunRepository) Pause(ctx context.Context, item workflowrun.Execution, step workflowrun.StepExecution, approvals []workflowrun.ApprovalTask, event workflowrun.Event) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.saveExecution(tx, item); err != nil {
			return err
		}
		if err := saveStep(tx, step); err != nil {
			return err
		}
		for _, task := range approvals {
			row := approvalTaskModel(task)
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
				return err
			}
		}
		return createWorkflowEvent(tx, event)
	})
}

func (r *WorkflowRunRepository) Complete(ctx context.Context, item workflowrun.Execution, step workflowrun.StepExecution, event workflowrun.Event) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.saveExecution(tx, item); err != nil {
			return err
		}
		// Only actual successful PROD operations may complete linked tasks.
		var services []ReleaseServiceModel
		if err := tx.Where("release_id = ?", item.ReleaseID).Find(&services).Error; err != nil {
			return err
		}
		allProd := len(services) > 0
		for _, svc := range services {
			var n int64
			if err := tx.Model(&DeploymentModel{}).Where("execution_id = ? AND project_service_id = ? AND environment = 'prod' AND status = 'success'", item.ID, svc.ProjectServiceID).Count(&n).Error; err != nil {
				return err
			}
			allProd = allProd && n > 0
		}
		policy, err := NewDeliveryRepository(tx).GetPolicy(ctx, item.ProjectID, "prod")
		if err != nil {
			return err
		}
		if allProd && policy.CompletesTasks {
			var links []ReleaseTaskModel
			if err := tx.Where("release_id = ?", item.ReleaseID).Find(&links).Error; err != nil {
				return err
			}
			ids := []string{}
			seen := map[string]bool{}
			for _, link := range links {
				if !seen[link.TaskID] {
					ids = append(ids, link.TaskID)
					seen[link.TaskID] = true
				}
			}
			if err := NewTaskRepository(tx).BulkUpdateStatusWithEvents(ctx, ids, task.StatusPublished, "worker", item.ReleaseID, ""); err != nil {
				return err
			}
			if err := tx.Model(&ReleaseTaskModel{}).Where("release_id = ?", item.ReleaseID).Update("status", string(task.StatusPublished)).Error; err != nil {
				return err
			}
		}
		if err := saveStep(tx, step); err != nil {
			return err
		}
		return createWorkflowEvent(tx, event)
	})
}

func (r *WorkflowRunRepository) FailAttempt(ctx context.Context, item workflowrun.Execution, step workflowrun.StepExecution, next time.Time, event workflowrun.Event) error {
	item.WakeUpAt = &next
	return r.persist(ctx, item, step, nil, event)
}

func (r *WorkflowRunRepository) persist(ctx context.Context, item workflowrun.Execution, step workflowrun.StepExecution, approvals []workflowrun.ApprovalTask, event workflowrun.Event) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.saveExecution(tx, item); err != nil {
			return err
		}
		if step.ID != "" {
			if err := saveStep(tx, step); err != nil {
				return err
			}
		}
		return createWorkflowEvent(tx, event)
	})
}

func (r *WorkflowRunRepository) saveExecution(tx *gorm.DB, item workflowrun.Execution) error {
	row, err := workflowExecutionModel(item)
	if err != nil {
		return err
	}
	result := tx.Model(&WorkflowExecutionModel{}).Where("id = ? AND lock_version = ? AND phase = ? AND wake_up_at > ?", item.ID, item.LockVersion, string(workflowrun.PhaseRunning), time.Now()).Updates(map[string]any{
		"public_status": row.PublicStatus, "phase": row.Phase, "current_step_id": row.CurrentStepID, "wake_up_at": row.WakeUpAt,
		"last_error": row.LastError, "context": row.Context, "completed_at": row.CompletedAt, "lock_version": gorm.Expr("lock_version + 1"), "updated_at": time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return shared.ErrConflict
	}
	return syncRelease(tx, item, "execution."+string(item.Phase))
}

func (r *WorkflowRunRepository) Approve(ctx context.Context, executionID, taskID, approver, decision string) (workflowrun.Execution, error) {
	if decision != "approved" && decision != "rejected" {
		return workflowrun.Execution{}, shared.ErrInvalidInput
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var execution WorkflowExecutionModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", executionID).First(&execution).Error; err != nil {
			return mapNotFound(err)
		}
		if execution.Phase != string(workflowrun.PhaseWaitingApproval) {
			return shared.ErrInvalidTransition
		}
		now := time.Now()
		result := tx.Model(&ApprovalTaskModel{}).Where("id = ? AND execution_id = ? AND step_id = ? AND approver = ? AND decision = 'pending'", taskID, executionID, execution.CurrentStepID, approver).Updates(map[string]any{"decision": decision, "decided_by": approver, "decided_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return shared.ErrForbidden
		}
		if decision == "rejected" {
			if err := tx.Model(&WorkflowExecutionModel{}).Where("id = ?", executionID).Updates(map[string]any{"phase": string(workflowrun.PhaseFailed), "public_status": "failed", "last_error": "approval rejected", "wake_up_at": nil, "updated_at": now, "lock_version": gorm.Expr("lock_version + 1")}).Error; err != nil {
				return err
			}
			item, _ := toWorkflowExecution(execution)
			item.PublicStatus = "failed"
			if err := syncRelease(tx, item, "approval.rejected"); err != nil {
				return err
			}
			return createWorkflowEvent(tx, workflowrun.Event{ExecutionID: executionID, Type: "approval.rejected", Payload: map[string]any{"approver": approver}})
		}
		var tasks []ApprovalTaskModel
		if err := tx.Where("execution_id = ? AND step_id = ?", executionID, execution.CurrentStepID).Find(&tasks).Error; err != nil {
			return err
		}
		def, err := definitionFromMap(execution.DefinitionSnapshot)
		if err != nil {
			return err
		}
		step := def.Steps[execution.CurrentStepID]
		approved := 0
		for _, task := range tasks {
			if task.Decision == "approved" {
				approved++
			}
		}
		required := len(tasks)
		if step.Policy == "any" {
			required = 1
		}
		if step.Policy == "quorum" {
			required = step.Quorum
		}
		if approved >= required {
			return tx.Model(&WorkflowExecutionModel{}).Where("id = ?", executionID).Updates(map[string]any{"phase": string(workflowrun.PhaseRunning), "current_step_id": step.Next, "wake_up_at": nil, "updated_at": now, "lock_version": gorm.Expr("lock_version + 1")}).Error
		}
		return nil
	})
	if err != nil {
		return workflowrun.Execution{}, err
	}
	return r.Get(ctx, executionID)
}

func (r *WorkflowRunRepository) Retry(ctx context.Context, id string) (workflowrun.Execution, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row WorkflowExecutionModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.Phase == string(workflowrun.PhaseRunning) {
			return nil
		}
		if row.Phase != string(workflowrun.PhaseWaitingRetry) {
			return shared.ErrInvalidTransition
		}
		if row.Context == nil {
			row.Context = JSONMap{}
		}
		row.Context["deployment_tracking_v1"] = true
		if err := tx.Model(&row).Updates(map[string]any{"phase": "running", "public_status": "publishing", "context": row.Context, "wake_up_at": nil, "last_error": "", "updated_at": time.Now(), "lock_version": gorm.Expr("lock_version + 1")}).Error; err != nil {
			return err
		}
		item, _ := toWorkflowExecution(row)
		item.PublicStatus = "publishing"
		if err := syncRelease(tx, item, "execution.retried"); err != nil {
			return err
		}
		return createWorkflowEvent(tx, workflowrun.Event{ExecutionID: id, Type: "execution.retried", Payload: map[string]any{}})
	})
	if err != nil {
		return workflowrun.Execution{}, err
	}
	return r.Get(ctx, id)
}
func (r *WorkflowRunRepository) Cancel(ctx context.Context, id, actor string) (workflowrun.Execution, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row WorkflowExecutionModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.Phase == "cancelled" {
			return nil
		}
		if row.Phase == "completed" {
			return shared.ErrInvalidTransition
		}
		now := time.Now()
		if err := tx.Model(&row).Updates(map[string]any{"phase": "cancelled", "public_status": "cancelled", "wake_up_at": nil, "completed_at": now, "updated_at": now, "lock_version": gorm.Expr("lock_version + 1")}).Error; err != nil {
			return err
		}
		item, _ := toWorkflowExecution(row)
		item.PublicStatus = "cancelled"
		if err := syncRelease(tx, item, "execution.cancelled"); err != nil {
			return err
		}
		if err := tx.Model(&DeploymentModel{}).Where("execution_id = ? AND status = 'running'", id).Updates(map[string]any{"status": "cancelled", "message": "execution cancelled; external outcome may require reconciliation", "finished_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&ReleaseServiceModel{}).Where("release_id = ? AND status IN ?", row.ReleaseID, []string{"pending", "running"}).Updates(map[string]any{"status": "skipped", "message": "execution cancelled", "finished_at": now}).Error; err != nil {
			return err
		}
		return createWorkflowEvent(tx, workflowrun.Event{ExecutionID: id, Type: "execution.cancelled", Payload: map[string]any{"actor": actor}})
	})
	if err != nil {
		return workflowrun.Execution{}, err
	}
	return r.Get(ctx, id)
}

// Execution and public state always commit together, including legacy job projections.
func syncRelease(tx *gorm.DB, item workflowrun.Execution, eventType string) error {
	var rel ReleaseModel
	if err := tx.Where("id = ?", item.ReleaseID).First(&rel).Error; err != nil {
		return err
	}
	if rel.Status != item.PublicStatus {
		previousStatus := rel.Status
		if err := tx.Model(&rel).Updates(map[string]any{"status": item.PublicStatus, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		event := ReleaseEventModel{ID: shared.NewID("event"), ReleaseID: item.ReleaseID, Type: eventType, Title: eventType, Actor: "worker", FromStatus: previousStatus, ToStatus: item.PublicStatus, CreatedAt: time.Now()}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
	}
	status := "running"
	var finished *time.Time
	switch item.PublicStatus {
	case "success":
		status = "success"
	case "failed", "cancelled":
		status = "failed"
	}
	if status != "running" {
		now := time.Now()
		finished = &now
	}
	legacyID, _ := item.Context["legacy_job_id"].(string)
	if legacyID == "" {
		return nil
	}
	return tx.Model(&ReleaseJobModel{}).Where("id = ? AND release_id = ?", legacyID, item.ReleaseID).Updates(map[string]any{"status": status, "finished_at": finished, "message": item.LastError, "updated_at": time.Now()}).Error
}

func (r *WorkflowRunRepository) full(ctx context.Context, row WorkflowExecutionModel) (workflowrun.Execution, error) {
	item, err := toWorkflowExecution(row)
	if err != nil {
		return workflowrun.Execution{}, err
	}
	var steps []WorkflowStepExecutionModel
	if err := r.db.WithContext(ctx).Where("execution_id = ?", row.ID).Order("started_at asc").Find(&steps).Error; err != nil {
		return item, err
	}
	for _, s := range steps {
		item.Steps = append(item.Steps, toStepExecution(s))
	}
	var approvals []ApprovalTaskModel
	if err := r.db.WithContext(ctx).Where("execution_id = ?", row.ID).Order("created_at asc").Find(&approvals).Error; err != nil {
		return item, err
	}
	for _, a := range approvals {
		item.Approvals = append(item.Approvals, toApprovalTask(a))
	}
	var events []WorkflowEventModel
	if err := r.db.WithContext(ctx).Where("execution_id = ?", row.ID).Order("sequence asc").Find(&events).Error; err != nil {
		return item, err
	}
	for _, e := range events {
		item.Events = append(item.Events, workflowrun.Event{ID: e.ID, ExecutionID: e.ExecutionID, Sequence: e.Sequence, Type: e.Type, Payload: map[string]any(e.Payload), CreatedAt: e.CreatedAt})
	}
	return item, nil
}

func createWorkflowEvent(tx *gorm.DB, event workflowrun.Event) error {
	if event.ID == "" {
		event.ID = shared.NewID("wfevent")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	var max int64
	if err := tx.Model(&WorkflowEventModel{}).Where("execution_id = ?", event.ExecutionID).Select("COALESCE(MAX(sequence), 0)").Scan(&max).Error; err != nil {
		return err
	}
	row := WorkflowEventModel{ID: event.ID, ExecutionID: event.ExecutionID, Sequence: max + 1, Type: event.Type, Payload: JSONMap(event.Payload), CreatedAt: event.CreatedAt}
	return tx.Create(&row).Error
}

func saveStep(tx *gorm.DB, step workflowrun.StepExecution) error {
	row := stepExecutionModel(step)
	return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error
}
func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return shared.ErrNotFound
	}
	return err
}

func workflowExecutionModel(item workflowrun.Execution) (WorkflowExecutionModel, error) {
	b, err := json.Marshal(item.DefinitionSnapshot)
	if err != nil {
		return WorkflowExecutionModel{}, err
	}
	var def JSONMap
	if err := json.Unmarshal(b, &def); err != nil {
		return WorkflowExecutionModel{}, err
	}
	return WorkflowExecutionModel{ID: item.ID, ReleaseID: item.ReleaseID, ProjectID: item.ProjectID, DefinitionName: item.DefinitionName, DefinitionVersion: item.DefinitionVersion, DefinitionSnapshot: def, PublicStatus: item.PublicStatus, Phase: string(item.Phase), CurrentStepID: item.CurrentStepID, WakeUpAt: item.WakeUpAt, LastError: item.LastError, Context: JSONMap(item.Context), LockVersion: item.LockVersion, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, CompletedAt: item.CompletedAt}, nil
}
func definitionFromMap(value JSONMap) (workflowrun.Definition, error) {
	b, e := json.Marshal(value)
	if e != nil {
		return workflowrun.Definition{}, e
	}
	var d workflowrun.Definition
	e = json.Unmarshal(b, &d)
	return d, e
}
func toWorkflowExecution(row WorkflowExecutionModel) (workflowrun.Execution, error) {
	d, e := definitionFromMap(row.DefinitionSnapshot)
	if e != nil {
		return workflowrun.Execution{}, e
	}
	return workflowrun.Execution{ID: row.ID, ReleaseID: row.ReleaseID, ProjectID: row.ProjectID, DefinitionName: row.DefinitionName, DefinitionVersion: row.DefinitionVersion, DefinitionSnapshot: d, PublicStatus: row.PublicStatus, Phase: workflowrun.Phase(row.Phase), CurrentStepID: row.CurrentStepID, WakeUpAt: row.WakeUpAt, LastError: row.LastError, Context: map[string]any(row.Context), LockVersion: row.LockVersion, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, CompletedAt: row.CompletedAt}, nil
}
func stepExecutionModel(s workflowrun.StepExecution) WorkflowStepExecutionModel {
	return WorkflowStepExecutionModel{ID: s.ID, ExecutionID: s.ExecutionID, StepID: s.StepID, Attempt: s.Attempt, Status: string(s.Status), IdempotencyKey: s.IdempotencyKey, Input: JSONMap(s.Input), Output: JSONMap(s.Output), Error: s.Error, StartedAt: s.StartedAt, FinishedAt: s.FinishedAt}
}
func toStepExecution(s WorkflowStepExecutionModel) workflowrun.StepExecution {
	return workflowrun.StepExecution{ID: s.ID, ExecutionID: s.ExecutionID, StepID: s.StepID, Attempt: s.Attempt, Status: workflowrun.StepStatus(s.Status), IdempotencyKey: s.IdempotencyKey, Input: map[string]any(s.Input), Output: map[string]any(s.Output), Error: s.Error, StartedAt: s.StartedAt, FinishedAt: s.FinishedAt}
}
func approvalTaskModel(a workflowrun.ApprovalTask) ApprovalTaskModel {
	return ApprovalTaskModel{ID: a.ID, ExecutionID: a.ExecutionID, StepID: a.StepID, Approver: a.Approver, Decision: a.Decision, DecidedBy: a.DecidedBy, DecidedAt: a.DecidedAt, CreatedAt: a.CreatedAt}
}
func toApprovalTask(a ApprovalTaskModel) workflowrun.ApprovalTask {
	return workflowrun.ApprovalTask{ID: a.ID, ExecutionID: a.ExecutionID, StepID: a.StepID, Approver: a.Approver, Decision: a.Decision, DecidedBy: a.DecidedBy, DecidedAt: a.DecidedAt, CreatedAt: a.CreatedAt}
}
