package taskapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fluxa-api/internal/application/authorization"
	"fluxa-api/internal/domain/project"
	"fluxa-api/internal/domain/task"
	"fluxa-api/internal/domain/workflow"
	"fluxa-api/internal/shared"
)

type Service struct {
	repo      task.Repository
	projects  project.Repository
	workflows workflow.Repository
	access    *authorization.Service
}

func New(repo task.Repository, projects project.Repository, workflows workflow.Repository, access *authorization.Service) *Service {
	return &Service{repo: repo, projects: projects, workflows: workflows, access: access}
}

func (s *Service) Create(ctx context.Context, in task.CreateTaskInput) (task.Task, error) {
	if strings.TrimSpace(in.ProjectID) == "" || strings.TrimSpace(in.Title) == "" {
		return task.Task{}, shared.ErrInvalidInput
	}
	if err := s.access.Require(ctx, in.ProjectID, authorization.RoleDeveloper); err != nil {
		return task.Task{}, err
	}
	principal, err := authorization.Principal(ctx)
	if err != nil {
		return task.Task{}, err
	}
	creator := principalDisplayName(principal.UserName, principal.UserEmail, principal.UserID)
	now := time.Now()
	initialStatus := task.StatusTodo
	if s.workflows != nil {
		wf, err := s.workflows.Get(ctx, in.ProjectID, workflow.KindTask)
		if err != nil {
			return task.Task{}, err
		}
		initialStatus = task.Status(wf.InitialStatus)
	}
	item := task.Task{
		ID: shared.NewID("task"), ProjectID: in.ProjectID, Title: strings.TrimSpace(in.Title),
		Description: strings.TrimSpace(in.Description), Creator: creator, Assignee: creator,
		Status: initialStatus, Details: in.Details, CreatedAt: now, UpdatedAt: now,
	}
	created, err := s.repo.CreateWithEvent(ctx, item, task.TaskEvent{
		ID: shared.NewID("event"), TaskID: item.ID, Type: task.EventTaskCreated, Title: "创建任务",
		Message: item.Title, Actor: creator, ToStatus: string(item.Status), CreatedAt: now,
	})
	if err != nil {
		return task.Task{}, err
	}
	full, err := s.repo.GetFull(ctx, created.ID)
	if err != nil {
		return task.Task{}, err
	}
	return ensureCreator(full), nil
}

func (s *Service) List(ctx context.Context, projectID string, status task.Status) ([]task.Task, error) {
	if projectID != "" {
		if err := s.access.Require(ctx, projectID, authorization.RoleDeveloper); err != nil {
			return nil, err
		}
		items, err := s.repo.List(ctx, projectID, status)
		if err != nil {
			return nil, err
		}
		return ensureCreators(items), nil
	}
	items, err := s.repo.List(ctx, "", status)
	if err != nil || authorization.IsAdmin(ctx) {
		if err != nil {
			return items, err
		}
		return ensureCreators(items), nil
	}
	out := make([]task.Task, 0, len(items))
	for _, item := range items {
		if s.access.CanRead(ctx, item.ProjectID) {
			out = append(out, item)
		}
	}
	return ensureCreators(out), nil
}

func (s *Service) Get(ctx context.Context, id string) (task.Task, error) {
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return task.Task{}, err
	}
	if err := s.access.Require(ctx, item.ProjectID, authorization.RoleDeveloper); err != nil {
		return task.Task{}, err
	}
	full, err := s.repo.GetFull(ctx, id)
	if err != nil {
		return task.Task{}, err
	}
	return ensureCreator(full), nil
}

func ensureCreators(items []task.Task) []task.Task {
	for i := range items {
		items[i] = ensureCreator(items[i])
	}
	return items
}

func principalDisplayName(userName, userEmail string, userID int64) string {
	if name := strings.TrimSpace(userName); name != "" {
		return name
	}
	if email := strings.TrimSpace(userEmail); email != "" {
		return email
	}
	return fmt.Sprintf("%d", userID)
}

func ensureCreator(item task.Task) task.Task {
	item.Creator = strings.TrimSpace(item.Creator)
	if item.Creator == "" {
		for _, event := range item.Events {
			if event.Type == task.EventTaskCreated && strings.TrimSpace(event.Actor) != "" {
				item.Creator = strings.TrimSpace(event.Actor)
				break
			}
		}
	}
	if item.Creator == "" {
		item.Creator = strings.TrimSpace(item.Assignee)
	}
	if item.Creator == "" {
		item.Creator = "-"
	}
	if len(item.Events) == 0 {
		item.Events = []task.TaskEvent{{
			ID: "synthetic_created", TaskID: item.ID, Type: task.EventTaskCreated,
			Title: "创建任务", Message: item.Title, Actor: item.Creator,
			ToStatus: string(item.Status), CreatedAt: item.CreatedAt,
		}}
	}
	return item
}

func (s *Service) Transition(ctx context.Context, id string, next task.Status) (task.Task, error) {
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return task.Task{}, err
	}
	if err := s.access.Require(ctx, item.ProjectID, authorization.RoleDeveloper); err != nil {
		return task.Task{}, err
	}
	principal, err := authorization.Principal(ctx)
	if err != nil {
		return task.Task{}, err
	}
	if s.workflows != nil {
		allowed, err := s.workflows.AllowedTransitions(ctx, item.ProjectID, workflow.KindTask, string(item.Status))
		if err != nil {
			return task.Task{}, err
		}
		ok := false
		for _, transition := range allowed {
			if transition.ToStatus == string(next) {
				ok = true
				break
			}
		}
		if !ok {
			return task.Task{}, shared.ErrInvalidTransition
		}
		return s.repo.TransitionStatus(ctx, id, item.Status, next, task.TaskEvent{
			ID: shared.NewID("event"), TaskID: id, Type: task.EventTaskStatusChanged, Title: "状态流转",
			Actor: principalDisplayName(principal.UserName, principal.UserEmail, principal.UserID), FromStatus: string(item.Status), ToStatus: string(next), CreatedAt: time.Now(),
		})
	}
	updated, err := item.TransitionTo(next)
	if err != nil {
		return task.Task{}, err
	}
	return s.repo.TransitionStatus(ctx, id, item.Status, updated.Status, task.TaskEvent{
		ID: shared.NewID("event"), TaskID: id, Type: task.EventTaskStatusChanged, Title: "状态流转",
		Actor: principalDisplayName(principal.UserName, principal.UserEmail, principal.UserID), FromStatus: string(item.Status), ToStatus: string(updated.Status), CreatedAt: time.Now(),
	})
}

func (s *Service) UpdateAssignee(ctx context.Context, id string, assignee string) (task.Task, error) {
	assignee = strings.TrimSpace(assignee)
	if assignee == "" {
		return task.Task{}, shared.ErrInvalidInput
	}
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return task.Task{}, err
	}
	if err := s.access.Require(ctx, item.ProjectID, authorization.RoleDeveloper); err != nil {
		return task.Task{}, err
	}
	if s.projects != nil {
		members, err := s.projects.ListMembers(ctx, item.ProjectID)
		if err != nil {
			return task.Task{}, err
		}
		found := false
		for _, member := range members {
			if member.UserName == assignee {
				found = true
				break
			}
		}
		if !found {
			return task.Task{}, shared.ErrInvalidInput
		}
	}
	updated, err := s.repo.UpdateAssignee(ctx, id, assignee)
	if err != nil {
		return task.Task{}, err
	}
	full, err := s.repo.GetFull(ctx, updated.ID)
	if err != nil {
		return task.Task{}, err
	}
	return ensureCreator(full), nil
}

func (s *Service) addEvent(ctx context.Context, event task.TaskEvent) {
	event.ID = shared.NewID("event")
	event.CreatedAt = time.Now()
	_ = s.repo.AddEvent(ctx, event)
}
