package releaseapp

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"fluxa-api/internal/application/authorization"
	"fluxa-api/internal/domain/delivery"
	"fluxa-api/internal/domain/release"
	"fluxa-api/internal/domain/servicecatalog"
	"fluxa-api/internal/domain/task"
	"fluxa-api/internal/domain/workflow"
	"fluxa-api/internal/domain/workflowrun"
	"fluxa-api/internal/shared"
)

type Service struct {
	releases  release.Repository
	tasks     task.Repository
	catalog   servicecatalog.Repository
	workflows workflow.Repository
	delivery  delivery.Repository
	access    *authorization.Service
	runner    WorkflowRunner
}

type WorkflowRunner interface {
	Start(context.Context, string, workflowrun.StartInput) (workflowrun.Execution, error)
	GetByRelease(context.Context, string) (workflowrun.Execution, error)
	Retry(context.Context, string) (workflowrun.Execution, error)
	Cancel(context.Context, string) (workflowrun.Execution, error)
}

func (s *Service) SetWorkflowRunner(runner WorkflowRunner) { s.runner = runner }

const systemActor = "operator"

func New(releases release.Repository, tasks task.Repository, catalog servicecatalog.Repository, workflows workflow.Repository, deliveryRepo delivery.Repository, access *authorization.Service) *Service {
	return &Service{releases: releases, tasks: tasks, catalog: catalog, workflows: workflows, delivery: deliveryRepo, access: access}
}

func (s *Service) Create(ctx context.Context, in release.CreateReleaseInput) (release.Release, error) {
	if strings.TrimSpace(in.ProjectID) == "" || strings.TrimSpace(in.Title) == "" {
		return release.Release{}, shared.ErrInvalidInput
	}
	if len(in.TaskIDs) == 0 || len(in.Services) == 0 {
		return release.Release{}, shared.ErrInvalidInput
	}
	if err := s.access.Require(ctx, in.ProjectID, authorization.RoleDeveloper); err != nil {
		return release.Release{}, err
	}
	now := time.Now()
	initialStatus := release.ReleaseDraft
	if s.workflows != nil {
		wf, err := s.workflows.Get(ctx, in.ProjectID, workflow.KindRelease)
		if err != nil {
			return release.Release{}, err
		}
		initialStatus = release.ReleaseStatus(wf.InitialStatus)
	}
	rel := release.Release{
		ID: shared.NewID("rel"), ProjectID: in.ProjectID, Title: strings.TrimSpace(in.Title),
		Description: strings.TrimSpace(in.Description), Environment: "release",
		Status: initialStatus, Details: in.Details, CreatedAt: now, UpdatedAt: now,
	}
	for _, taskID := range in.TaskIDs {
		t, err := s.tasks.Get(ctx, taskID)
		if err != nil {
			return release.Release{}, err
		}
		if t.ProjectID != in.ProjectID || t.Status != task.StatusPublishing {
			return release.Release{}, shared.ErrInvalidInput
		}
		rel.Tasks = append(rel.Tasks, release.ReleaseTask{
			ID: shared.NewID("reltask"), ReleaseID: rel.ID, TaskID: t.ID, Title: t.Title, Status: string(t.Status), CreatedAt: now,
		})
	}
	seenServices := map[string]bool{}
	for i, item := range in.Services {
		if seenServices[item.ProjectServiceID] {
			return release.Release{}, shared.ErrInvalidInput
		}
		seenServices[item.ProjectServiceID] = true
		if strings.TrimSpace(item.ArtifactID) == "" {
			return release.Release{}, shared.ErrInvalidInput
		}
		ps, err := s.catalog.GetProjectService(ctx, item.ProjectServiceID)
		if err != nil {
			return release.Release{}, err
		}
		if ps.ProjectID != in.ProjectID {
			return release.Release{}, shared.ErrInvalidInput
		}
		artifact, err := s.delivery.GetArtifact(ctx, item.ArtifactID)
		if err != nil {
			return release.Release{}, err
		}
		if artifact.ProjectID != in.ProjectID || artifact.ProjectServiceID != ps.ID {
			return release.Release{}, shared.ErrInvalidInput
		}
		order := item.OrderIndex
		if order == 0 {
			order = i + 1
		}
		rel.Services = append(rel.Services, release.ReleaseService{
			ID: shared.NewID("relsvc"), ReleaseID: rel.ID, ProjectServiceID: ps.ID, ArtifactID: artifact.ID,
			ServiceKey: ps.ServiceKey, DisplayName: ps.DisplayName, DeployTarget: string(ps.DeployTarget),
			DeployConfig: ps.DeployConfig, Ref: artifact.ImageDigest, OrderIndex: order,
			Status: release.ServicePending, CreatedAt: now, UpdatedAt: now,
		})
	}
	created, err := s.releases.CreateWithEvent(ctx, rel, newReleaseEvent(ctx, release.ReleaseEvent{
		ReleaseID: rel.ID, Type: release.EventReleaseCreated, Title: "创建发布单",
		Message: rel.Title, ToStatus: string(rel.Status),
	}))
	if err != nil {
		return release.Release{}, err
	}
	return s.releases.GetFull(ctx, created.ID)
}

func (s *Service) List(ctx context.Context, projectID string) ([]release.Release, error) {
	items, err := s.releases.List(ctx)
	if err != nil {
		return items, err
	}
	if projectID != "" {
		if err := s.access.Require(ctx, projectID, authorization.RoleDeveloper); err != nil {
			return nil, err
		}
		out := make([]release.Release, 0)
		for _, item := range items {
			if item.ProjectID == projectID {
				out = append(out, item)
			}
		}
		return out, nil
	}
	if authorization.IsAdmin(ctx) {
		return items, err
	}
	out := make([]release.Release, 0, len(items))
	for _, item := range items {
		if s.access.CanRead(ctx, item.ProjectID) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id string) (release.Release, error) {
	item, err := s.releases.Get(ctx, id)
	if err != nil {
		return release.Release{}, err
	}
	if err := s.access.Require(ctx, item.ProjectID, authorization.RoleDeveloper); err != nil {
		return release.Release{}, err
	}
	return s.releases.GetFull(ctx, id)
}

func (s *Service) Submit(ctx context.Context, id string) (release.Release, error) {
	rel, err := s.releases.Get(ctx, id)
	if err != nil {
		return release.Release{}, err
	}
	policy, err := s.delivery.GetPolicy(ctx, rel.ProjectID, "prod")
	if err != nil {
		return release.Release{}, err
	}
	if policy.Approval {
		return s.transition(ctx, id, release.ReleasePendingApproval, release.EventReleaseSubmitted, "提交审批")
	}
	if _, err := s.runner.Start(ctx, id, workflowrun.StartInput{}); err != nil {
		return release.Release{}, err
	}
	return s.releases.GetFull(ctx, id)
}

func (s *Service) Approve(ctx context.Context, id string) (release.Release, error) {
	rel, err := s.releases.Get(ctx, id)
	if err != nil {
		return release.Release{}, err
	}
	if err := s.access.Require(ctx, rel.ProjectID, authorization.RoleLead); err != nil {
		return release.Release{}, err
	}
	if rel.Status == release.ReleasePendingApproval {
		if _, err := s.transition(ctx, id, release.ReleaseApproved, release.EventReleaseApproved, "审批通过"); err != nil {
			return release.Release{}, err
		}
	} else if rel.Status != release.ReleaseApproved {
		if _, err := s.runner.GetByRelease(ctx, id); err != nil {
			return release.Release{}, shared.ErrInvalidTransition
		}
	}
	if _, err := s.Publish(ctx, id); err != nil {
		return release.Release{}, err
	}
	return s.releases.GetFull(ctx, id)
}
func (s *Service) Publish(ctx context.Context, id string) (workflowrun.Execution, error) {
	return s.runner.Start(ctx, id, workflowrun.StartInput{})
}
func (s *Service) Retry(ctx context.Context, id string) (workflowrun.Execution, error) {
	item, err := s.runner.GetByRelease(ctx, id)
	if err != nil {
		return workflowrun.Execution{}, err
	}
	return s.runner.Retry(ctx, item.ID)
}

func (s *Service) GetJob(ctx context.Context, id string) (release.ReleaseJob, error) {
	job, err := s.releases.GetJob(ctx, id)
	if err != nil {
		return release.ReleaseJob{}, err
	}
	rel, err := s.releases.Get(ctx, job.ReleaseID)
	if err != nil {
		return release.ReleaseJob{}, err
	}
	if err := s.access.Require(ctx, rel.ProjectID, authorization.RoleDeveloper); err != nil {
		return release.ReleaseJob{}, err
	}
	return job, nil
}

func (s *Service) SetStatus(ctx context.Context, id string, nextStatus release.ReleaseStatus) (release.Release, *workflowrun.Execution, error) {
	if nextStatus == release.ReleaseQueued || nextStatus == release.ReleasePublishing {
		item, err := s.Publish(ctx, id)
		if err != nil {
			return release.Release{}, nil, err
		}
		rel, err := s.releases.GetFull(ctx, id)
		return rel, &item, err
	}
	if nextStatus == release.ReleaseCancelled {
		item, err := s.runner.GetByRelease(ctx, id)
		if err == nil {
			cancelled, err := s.runner.Cancel(ctx, item.ID)
			if err != nil {
				return release.Release{}, nil, err
			}
			rel, err := s.releases.GetFull(ctx, id)
			return rel, &cancelled, err
		}
		if !errors.Is(err, shared.ErrNotFound) {
			return release.Release{}, nil, err
		}
	}
	rel, err := s.transition(ctx, id, nextStatus, eventTypeForStatus(nextStatus), eventTitleForStatus(nextStatus))
	return rel, nil, err
}

func (s *Service) transition(ctx context.Context, id string, nextStatus release.ReleaseStatus, eventType release.ReleaseEventType, title string) (release.Release, error) {
	rel, err := s.releases.Get(ctx, id)
	if err != nil {
		return release.Release{}, err
	}
	if s.runner != nil {
		if _, err := s.runner.GetByRelease(ctx, id); err == nil {
			return release.Release{}, shared.ErrInvalidTransition
		} else if !errors.Is(err, shared.ErrNotFound) {
			return release.Release{}, err
		}
	}
	minimumRole := authorization.RoleDeveloper
	if nextStatus == release.ReleaseApproved || nextStatus == release.ReleasePublishing {
		minimumRole = authorization.RoleLead
	}
	if nextStatus == release.ReleaseSuccess || nextStatus == release.ReleaseFailed {
		return release.Release{}, shared.ErrForbidden
	}
	if err := s.access.Require(ctx, rel.ProjectID, minimumRole); err != nil {
		return release.Release{}, err
	}
	if err := s.validateTransition(ctx, rel.ProjectID, rel.Status, nextStatus); err != nil {
		return release.Release{}, err
	}
	updated, err := s.releases.TransitionStatus(ctx, id, rel.Status, nextStatus, newReleaseEvent(ctx, release.ReleaseEvent{
		ReleaseID: id, Type: eventType, Title: title,
		FromStatus: string(rel.Status), ToStatus: string(nextStatus),
	}))
	if err != nil {
		return release.Release{}, err
	}
	return s.releases.GetFull(ctx, updated.ID)
}

func (s *Service) validateTransition(ctx context.Context, projectID string, current, next release.ReleaseStatus) error {
	if s.workflows == nil {
		rel := release.Release{Status: current}
		_, err := rel.TransitionTo(next)
		return err
	}
	allowed, err := s.workflows.AllowedTransitions(ctx, projectID, workflow.KindRelease, string(current))
	if err != nil {
		return err
	}
	for _, transition := range allowed {
		if transition.ToStatus == string(next) {
			return nil
		}
	}
	return shared.ErrInvalidTransition
}

func (s *Service) addEvent(ctx context.Context, event release.ReleaseEvent) {
	event = newReleaseEvent(ctx, event)
	_ = s.releases.AddEvent(ctx, event)
}

func newReleaseEvent(ctx context.Context, event release.ReleaseEvent) release.ReleaseEvent {
	if event.ID == "" {
		event.ID = shared.NewID("event")
	}
	if event.Actor == "" {
		event.Actor = currentActor(ctx)
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	return event
}

func currentActor(ctx context.Context) string {
	principal, err := authorization.Principal(ctx)
	if err != nil {
		return systemActor
	}
	if name := strings.TrimSpace(principal.UserName); name != "" {
		return name
	}
	if email := strings.TrimSpace(principal.UserEmail); email != "" {
		return email
	}
	if principal.UserID != 0 {
		return strconv.FormatInt(principal.UserID, 10)
	}
	return systemActor
}

func eventTypeForStatus(status release.ReleaseStatus) release.ReleaseEventType {
	switch status {
	case release.ReleasePendingApproval:
		return release.EventReleaseSubmitted
	case release.ReleaseApproved:
		return release.EventReleaseApproved
	default:
		return release.EventReleaseStatusChanged
	}
}

func eventTitleForStatus(status release.ReleaseStatus) string {
	switch status {
	case release.ReleasePendingApproval:
		return "提交审批"
	case release.ReleaseApproved:
		return "审批通过"
	case release.ReleaseCancelled:
		return "取消发布"
	default:
		return "状态流转"
	}
}
