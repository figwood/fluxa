package gormdb

import (
	"context"
	"errors"
	"time"

	"fluxa-api/internal/domain/workflow"
	"fluxa-api/internal/shared"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorkflowRepository struct{ db *gorm.DB }

func NewWorkflowRepository(db *gorm.DB) *WorkflowRepository { return &WorkflowRepository{db: db} }

// EnsureDefaults copies the legacy global configuration into every existing
// project. Fresh databases use the built-in defaults. The legacy tables remain
// read-only migration sources so both SQLite and PostgreSQL upgrades are safe.
func (r *WorkflowRepository) EnsureDefaults(ctx context.Context) error {
	var projects []ProjectModel
	if err := r.db.WithContext(ctx).Find(&projects).Error; err != nil {
		return err
	}
	sources := r.legacySources(ctx)
	for _, project := range projects {
		for _, source := range sources {
			if err := r.ensureProjectWorkflow(ctx, project.ID, source); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *WorkflowRepository) ensureProjectWorkflow(ctx context.Context, projectID string, source workflow.Workflow) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&WorkflowModel{}).
		Where("project_id = ? AND kind = ?", projectID, string(source.Kind)).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		if source.Kind == workflow.KindRelease {
			current, err := r.Get(ctx, projectID, source.Kind)
			if err != nil {
				return err
			}
			existing := map[string]bool{}
			for _, status := range current.Statuses {
				existing[status.ID] = true
			}
			changed := false
			if !existing["queued"] {
				current.Statuses = append(current.Statuses, workflow.Status{ID: "queued", Name: "已排队", StageID: "execution", Category: "active", OrderIndex: 40})
				changed = true
			}
			if !existing["running"] {
				current.Statuses = append(current.Statuses, workflow.Status{ID: "running", Name: "发布中", StageID: "execution", Category: "active", OrderIndex: 41})
				changed = true
			}
			if changed {
				_, err = r.Save(ctx, current)
				return err
			}
		}
		return nil
	}
	if source.Kind == workflow.KindRelease {
		ensureReleaseRuntimeStatuses(&source)
	}
	source.ProjectID = projectID
	_, err := r.Save(ctx, source)
	return err
}

func ensureReleaseRuntimeStatuses(item *workflow.Workflow) bool {
	existing := map[string]bool{}
	for _, status := range item.Statuses {
		existing[status.ID] = true
	}
	changed := false
	if !existing["queued"] {
		item.Statuses = append(item.Statuses, workflow.Status{ID: "queued", Name: "已排队", StageID: "execution", Category: "active", OrderIndex: 40})
		changed = true
	}
	if !existing["running"] {
		item.Statuses = append(item.Statuses, workflow.Status{ID: "running", Name: "发布中", StageID: "execution", Category: "active", OrderIndex: 41})
		changed = true
	}
	return changed
}

func (r *WorkflowRepository) legacySources(ctx context.Context) []workflow.Workflow {
	defaults := workflow.DefaultWorkflows()
	if !r.db.Migrator().HasTable("workflows") {
		return defaults
	}
	for i := range defaults {
		kind := string(defaults[i].Kind)
		var header struct {
			Kind, Name, Description, InitialStatus string
			CreatedAt, UpdatedAt                   time.Time
		}
		if err := r.db.WithContext(ctx).Table("workflows").Where("kind = ?", kind).First(&header).Error; err != nil {
			continue
		}
		defaults[i].Name, defaults[i].Description, defaults[i].InitialStatus = header.Name, header.Description, header.InitialStatus
		defaults[i].CreatedAt, defaults[i].UpdatedAt = header.CreatedAt, header.UpdatedAt
		var stages []struct {
			StageID, Name string
			OrderIndex    int
		}
		var statuses []struct {
			StatusID, Name, StageID, Category string
			OrderIndex                        int
		}
		var transitions []struct {
			TransitionID, Name, FromStatus, ToStatus, Action string
			OrderIndex                                       int
		}
		if r.db.Migrator().HasTable("workflow_stages") {
			_ = r.db.WithContext(ctx).Table("workflow_stages").Where("workflow_kind = ?", kind).Order("order_index").Scan(&stages).Error
		}
		_ = r.db.WithContext(ctx).Table("workflow_statuses").Where("workflow_kind = ?", kind).Order("order_index").Scan(&statuses).Error
		_ = r.db.WithContext(ctx).Table("workflow_transitions").Where("workflow_kind = ?", kind).Order("order_index").Scan(&transitions).Error
		if len(stages) > 0 {
			defaults[i].Stages = nil
			for _, row := range stages {
				defaults[i].Stages = append(defaults[i].Stages, workflow.Stage{ID: row.StageID, Name: row.Name, OrderIndex: row.OrderIndex})
			}
		}
		if len(statuses) > 0 {
			defaults[i].Statuses = nil
			for _, row := range statuses {
				defaults[i].Statuses = append(defaults[i].Statuses, workflow.Status{ID: row.StatusID, Name: row.Name, StageID: row.StageID, Category: row.Category, OrderIndex: row.OrderIndex})
			}
		}
		if len(transitions) > 0 {
			defaults[i].Transitions = nil
			for _, row := range transitions {
				defaults[i].Transitions = append(defaults[i].Transitions, workflow.Transition{ID: row.TransitionID, Name: row.Name, FromStatus: row.FromStatus, ToStatus: row.ToStatus, Action: row.Action, OrderIndex: row.OrderIndex})
			}
		}
	}
	return defaults
}

func (r *WorkflowRepository) List(ctx context.Context, projectID string) ([]workflow.Workflow, error) {
	query := r.db.WithContext(ctx).Order("project_id asc, kind asc")
	if projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	var rows []WorkflowModel
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]workflow.Workflow, 0, len(rows))
	for _, row := range rows {
		item, err := r.Get(ctx, row.ProjectID, workflow.Kind(row.Kind))
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *WorkflowRepository) Get(ctx context.Context, projectID string, kind workflow.Kind) (workflow.Workflow, error) {
	var row WorkflowModel
	if err := r.db.WithContext(ctx).Where("project_id = ? AND kind = ?", projectID, string(kind)).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			for _, item := range r.legacySources(ctx) {
				if item.Kind == kind {
					if err := r.ensureProjectWorkflow(ctx, projectID, item); err != nil {
						return workflow.Workflow{}, err
					}
					return r.Get(ctx, projectID, kind)
				}
			}
			return workflow.Workflow{}, shared.ErrNotFound
		}
		return workflow.Workflow{}, err
	}
	var stages []WorkflowStageModel
	var statuses []WorkflowStatusModel
	var transitions []WorkflowTransitionModel
	base := r.db.WithContext(ctx).Where("project_id = ? AND workflow_kind = ?", projectID, string(kind))
	if err := base.Order("order_index asc, stage_id asc").Find(&stages).Error; err != nil {
		return workflow.Workflow{}, err
	}
	if err := r.db.WithContext(ctx).Where("project_id = ? AND workflow_kind = ?", projectID, string(kind)).Order("order_index asc, status_id asc").Find(&statuses).Error; err != nil {
		return workflow.Workflow{}, err
	}
	if err := r.db.WithContext(ctx).Where("project_id = ? AND workflow_kind = ?", projectID, string(kind)).Order("order_index asc, transition_id asc").Find(&transitions).Error; err != nil {
		return workflow.Workflow{}, err
	}
	item := workflow.Workflow{ProjectID: projectID, Kind: kind, Name: row.Name, Description: row.Description, InitialStatus: row.InitialStatus, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Stages: []workflow.Stage{}, Statuses: []workflow.Status{}, Transitions: []workflow.Transition{}}
	for _, x := range stages {
		item.Stages = append(item.Stages, workflow.Stage{ID: x.StageID, Name: x.Name, OrderIndex: x.OrderIndex})
	}
	for _, x := range statuses {
		item.Statuses = append(item.Statuses, workflow.Status{ID: x.StatusID, Name: x.Name, StageID: x.StageID, Category: x.Category, OrderIndex: x.OrderIndex})
	}
	for _, x := range transitions {
		item.Transitions = append(item.Transitions, workflow.Transition{ID: x.TransitionID, Name: x.Name, FromStatus: x.FromStatus, ToStatus: x.ToStatus, Action: x.Action, OrderIndex: x.OrderIndex})
	}
	return item, nil
}

func (r *WorkflowRepository) Save(ctx context.Context, item workflow.Workflow) (workflow.Workflow, error) {
	// Existing work items must never be stranded by removing an in-use status.
	allowed := make(map[string]bool, len(item.Statuses))
	for _, status := range item.Statuses {
		allowed[status.ID] = true
	}
	var used []string
	query := r.db.WithContext(ctx)
	if item.Kind == workflow.KindTask {
		if err := query.Model(&TaskModel{}).Where("project_id = ?", item.ProjectID).Distinct().Pluck("status", &used).Error; err != nil {
			return workflow.Workflow{}, err
		}
	} else {
		if err := query.Model(&ReleaseModel{}).Where("project_id = ?", item.ProjectID).Distinct().Pluck("status", &used).Error; err != nil {
			return workflow.Workflow{}, err
		}
	}
	for _, status := range used {
		if !allowed[status] {
			return workflow.Workflow{}, shared.ErrConflict
		}
	}
	now := time.Now()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		id := workflowRowID(item.ProjectID, item.Kind, "config")
		row := WorkflowModel{ID: id, ProjectID: item.ProjectID, Kind: string(item.Kind), Name: item.Name, Description: item.Description, InitialStatus: item.InitialStatus, CreatedAt: now, UpdatedAt: now}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "project_id"}, {Name: "kind"}}, DoUpdates: clause.AssignmentColumns([]string{"name", "description", "initial_status", "updated_at"})}).Create(&row).Error; err != nil {
			return err
		}
		where := tx.Where("project_id = ? AND workflow_kind = ?", item.ProjectID, string(item.Kind))
		if err := where.Delete(&WorkflowTransitionModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ? AND workflow_kind = ?", item.ProjectID, string(item.Kind)).Delete(&WorkflowStatusModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ? AND workflow_kind = ?", item.ProjectID, string(item.Kind)).Delete(&WorkflowStageModel{}).Error; err != nil {
			return err
		}
		for _, x := range item.Stages {
			if err := tx.Create(&WorkflowStageModel{ID: workflowRowID(item.ProjectID, item.Kind, x.ID), ProjectID: item.ProjectID, WorkflowKind: string(item.Kind), StageID: x.ID, Name: x.Name, OrderIndex: x.OrderIndex, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
				return err
			}
		}
		for _, x := range item.Statuses {
			if err := tx.Create(&WorkflowStatusModel{ID: workflowRowID(item.ProjectID, item.Kind, x.ID), ProjectID: item.ProjectID, WorkflowKind: string(item.Kind), StatusID: x.ID, Name: x.Name, StageID: x.StageID, Category: x.Category, OrderIndex: x.OrderIndex, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
				return err
			}
		}
		for _, x := range item.Transitions {
			if err := tx.Create(&WorkflowTransitionModel{ID: workflowRowID(item.ProjectID, item.Kind, x.ID), ProjectID: item.ProjectID, WorkflowKind: string(item.Kind), TransitionID: x.ID, Name: x.Name, FromStatus: x.FromStatus, ToStatus: x.ToStatus, Action: x.Action, OrderIndex: x.OrderIndex, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return workflow.Workflow{}, err
	}
	return r.Get(ctx, item.ProjectID, item.Kind)
}

func (r *WorkflowRepository) AllowedTransitions(ctx context.Context, projectID string, kind workflow.Kind, fromStatus string) ([]workflow.Transition, error) {
	var rows []WorkflowTransitionModel
	if err := r.db.WithContext(ctx).Where("project_id = ? AND workflow_kind = ? AND from_status = ?", projectID, string(kind), fromStatus).Order("order_index asc, transition_id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]workflow.Transition, 0, len(rows))
	for _, row := range rows {
		out = append(out, workflow.Transition{ID: row.TransitionID, Name: row.Name, FromStatus: row.FromStatus, ToStatus: row.ToStatus, Action: row.Action, OrderIndex: row.OrderIndex})
	}
	return out, nil
}

func workflowRowID(projectID string, kind workflow.Kind, id string) string {
	return projectID + ":" + string(kind) + ":" + id
}
