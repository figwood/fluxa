package gormdb

import (
	"context"
	"errors"
	"time"

	"fluxa-api/internal/domain/task"
	"fluxa-api/internal/shared"

	"gorm.io/gorm"
)

type TaskRepository struct{ db *gorm.DB }

func NewTaskRepository(db *gorm.DB) *TaskRepository { return &TaskRepository{db: db} }

func (r *TaskRepository) Create(ctx context.Context, item task.Task) (task.Task, error) {
	return r.create(ctx, item, nil)
}

func (r *TaskRepository) CreateWithEvent(ctx context.Context, item task.Task, event task.TaskEvent) (task.Task, error) {
	return r.create(ctx, item, &event)
}

func (r *TaskRepository) create(ctx context.Context, item task.Task, event *task.TaskEvent) (task.Task, error) {
	row := TaskModel{
		ID: item.ID, ProjectID: item.ProjectID, Title: item.Title, Description: item.Description,
		Creator: item.Creator, Assignee: item.Assignee, Status: string(item.Status), Details: JSONMap(item.Details),
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if event != nil {
			eventRow := taskEventModel(*event)
			return tx.Create(&eventRow).Error
		}
		return nil
	}); err != nil {
		return task.Task{}, err
	}
	return toTask(row), nil
}

func (r *TaskRepository) BackfillCreators(ctx context.Context) error {
	var rows []TaskModel
	if err := r.db.WithContext(ctx).Where("creator = '' OR creator IS NULL").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		creator := row.Assignee
		var event TaskEventModel
		if err := r.db.WithContext(ctx).Where("task_id = ? AND type = ?", row.ID, string(task.EventTaskCreated)).Order("created_at asc").First(&event).Error; err == nil && event.Actor != "" {
			creator = event.Actor
		}
		if creator == "" {
			creator = "-"
		}
		if err := r.db.WithContext(ctx).Model(&TaskModel{}).Where("id = ?", row.ID).Update("creator", creator).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *TaskRepository) List(ctx context.Context, projectID string, status task.Status) ([]task.Task, error) {
	var rows []TaskModel
	q := r.db.WithContext(ctx).Order("created_at desc")
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	if status != "" {
		q = q.Where("status = ?", string(status))
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]task.Task, 0, len(rows))
	for _, row := range rows {
		out = append(out, toTask(row))
	}
	return out, nil
}

func (r *TaskRepository) Get(ctx context.Context, id string) (task.Task, error) {
	var row TaskModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return task.Task{}, shared.ErrNotFound
		}
		return task.Task{}, err
	}
	return toTask(row), nil
}

func (r *TaskRepository) GetFull(ctx context.Context, id string) (task.Task, error) {
	item, err := r.Get(ctx, id)
	if err != nil {
		return task.Task{}, err
	}
	events, err := r.ListEvents(ctx, id)
	if err != nil {
		return task.Task{}, err
	}
	item.Events = events
	return item, nil
}

func (r *TaskRepository) UpdateStatus(ctx context.Context, id string, status task.Status) (task.Task, error) {
	if err := r.db.WithContext(ctx).Model(&TaskModel{}).Where("id = ?", id).Updates(map[string]any{
		"status": string(status), "updated_at": time.Now(),
	}).Error; err != nil {
		return task.Task{}, err
	}
	return r.Get(ctx, id)
}

func (r *TaskRepository) TransitionStatus(ctx context.Context, id string, from, to task.Status, event task.TaskEvent) (task.Task, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&TaskModel{}).Where("id = ? AND status = ?", id, string(from)).Updates(map[string]any{"status": string(to), "updated_at": time.Now()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return shared.ErrInvalidTransition
		}
		row := taskEventModel(event)
		return tx.Create(&row).Error
	})
	if err != nil {
		return task.Task{}, err
	}
	return r.GetFull(ctx, id)
}

func (r *TaskRepository) UpdateAssignee(ctx context.Context, id string, assignee string) (task.Task, error) {
	if err := r.db.WithContext(ctx).Model(&TaskModel{}).Where("id = ?", id).Updates(map[string]any{
		"assignee": assignee, "updated_at": time.Now(),
	}).Error; err != nil {
		return task.Task{}, err
	}
	return r.Get(ctx, id)
}

func (r *TaskRepository) BulkUpdateStatus(ctx context.Context, ids []string, status task.Status) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&TaskModel{}).Where("id IN ?", ids).Update("status", string(status)).Error
}

func (r *TaskRepository) BulkUpdateStatusWithEvents(ctx context.Context, ids []string, status task.Status, actor, releaseID, jobID string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []TaskModel
		if err := tx.Where("id IN ?", ids).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != len(ids) {
			return shared.ErrNotFound
		}
		now := time.Now()
		for _, row := range rows {
			if row.Status == string(status) {
				continue
			}
			result := tx.Model(&TaskModel{}).Where("id = ? AND status = ?", row.ID, row.Status).Updates(map[string]any{"status": string(status), "updated_at": now})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return shared.ErrConflict
			}
			event := taskEventModel(task.TaskEvent{ID: shared.NewID("event"), TaskID: row.ID, Type: task.EventTaskStatusChanged, Title: "发布完成", Actor: actor, FromStatus: row.Status, ToStatus: string(status), ReleaseID: releaseID, JobID: jobID, CreatedAt: now})
			if err := tx.Create(&event).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *TaskRepository) AddEvent(ctx context.Context, event task.TaskEvent) error {
	row := taskEventModel(event)
	return r.db.WithContext(ctx).Create(&row).Error
}

func taskEventModel(event task.TaskEvent) TaskEventModel {
	return TaskEventModel{ID: event.ID, TaskID: event.TaskID, Type: string(event.Type), Title: event.Title, Message: event.Message, Actor: event.Actor, FromStatus: event.FromStatus, ToStatus: event.ToStatus, ReleaseID: event.ReleaseID, JobID: event.JobID, CreatedAt: event.CreatedAt}
}

func (r *TaskRepository) ListEvents(ctx context.Context, taskID string) ([]task.TaskEvent, error) {
	var rows []TaskEventModel
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]task.TaskEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, task.TaskEvent{
			ID: row.ID, TaskID: row.TaskID, Type: task.EventType(row.Type), Title: row.Title,
			Message: row.Message, Actor: row.Actor, FromStatus: row.FromStatus,
			ToStatus: row.ToStatus, ReleaseID: row.ReleaseID, JobID: row.JobID, CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

func toTask(row TaskModel) task.Task {
	return task.Task{
		ID: row.ID, ProjectID: row.ProjectID, Title: row.Title, Description: row.Description,
		Creator: row.Creator, Assignee: row.Assignee, Status: task.Status(row.Status), Details: map[string]any(row.Details),
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
