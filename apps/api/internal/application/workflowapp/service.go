package workflowapp

import (
	"context"
	"strings"
	"time"

	"fluxa-api/internal/application/authorization"
	"fluxa-api/internal/domain/workflow"
	"fluxa-api/internal/shared"
)

type Service struct {
	repo   workflow.Repository
	access *authorization.Service
}

func New(repo workflow.Repository, access *authorization.Service) *Service {
	return &Service{repo: repo, access: access}
}

func (s *Service) EnsureDefaults(ctx context.Context) error {
	return s.repo.EnsureDefaults(ctx)
}

func (s *Service) List(ctx context.Context, projectID string) ([]workflow.Workflow, error) {
	if projectID == "" {
		if !authorization.IsAdmin(ctx) {
			return nil, shared.ErrForbidden
		}
		return s.repo.List(ctx, "")
	}
	if s.access != nil {
		if err := s.access.Require(ctx, projectID, authorization.RoleDeveloper); err != nil {
			return nil, err
		}
	}
	return s.repo.List(ctx, projectID)
}

func (s *Service) Get(ctx context.Context, projectID string, kind workflow.Kind) (workflow.Workflow, error) {
	if !kind.Valid() {
		return workflow.Workflow{}, shared.ErrInvalidInput
	}
	if s.access != nil {
		if err := s.access.Require(ctx, projectID, authorization.RoleDeveloper); err != nil {
			return workflow.Workflow{}, err
		}
	}
	return s.repo.Get(ctx, projectID, kind)
}

func (s *Service) Update(ctx context.Context, projectID string, kind workflow.Kind, in workflow.UpdateWorkflowInput) (workflow.Workflow, error) {
	if !kind.Valid() {
		return workflow.Workflow{}, shared.ErrInvalidInput
	}
	if s.access != nil {
		if err := s.access.Require(ctx, projectID, authorization.RoleLead); err != nil {
			return workflow.Workflow{}, err
		}
	}
	item := workflow.Workflow{
		ProjectID:     projectID,
		Kind:          kind,
		Name:          strings.TrimSpace(in.Name),
		Description:   strings.TrimSpace(in.Description),
		InitialStatus: workflow.NormalizeStatusID(in.InitialStatus),
		Stages:        normalizeStages(in.Stages),
		Statuses:      normalizeStatuses(in.Statuses),
		Transitions:   normalizeTransitions(in.Transitions),
		UpdatedAt:     time.Now(),
	}
	if err := validateWorkflow(item); err != nil {
		return workflow.Workflow{}, err
	}
	return s.repo.Save(ctx, item)
}

func (s *Service) AllowedTransitions(ctx context.Context, projectID string, kind workflow.Kind, fromStatus string) ([]workflow.Transition, error) {
	if !kind.Valid() || strings.TrimSpace(fromStatus) == "" {
		return nil, shared.ErrInvalidInput
	}
	if s.access != nil {
		if err := s.access.Require(ctx, projectID, authorization.RoleDeveloper); err != nil {
			return nil, err
		}
	}
	return s.repo.AllowedTransitions(ctx, projectID, kind, workflow.NormalizeStatusID(fromStatus))
}

func normalizeStages(items []workflow.Stage) []workflow.Stage {
	out := make([]workflow.Stage, 0, len(items))
	for i, item := range items {
		stage := workflow.Stage{
			ID:         strings.TrimSpace(item.ID),
			Name:       strings.TrimSpace(item.Name),
			OrderIndex: item.OrderIndex,
		}
		if stage.OrderIndex == 0 {
			stage.OrderIndex = i + 1
		}
		out = append(out, stage)
	}
	return out
}

func normalizeStatuses(items []workflow.Status) []workflow.Status {
	out := make([]workflow.Status, 0, len(items))
	for i, item := range items {
		status := workflow.Status{
			ID:         workflow.NormalizeStatusID(item.ID),
			Name:       strings.TrimSpace(item.Name),
			StageID:    strings.TrimSpace(item.StageID),
			Category:   strings.TrimSpace(item.Category),
			OrderIndex: item.OrderIndex,
		}
		if status.OrderIndex == 0 {
			status.OrderIndex = i + 1
		}
		out = append(out, status)
	}
	return out
}

func normalizeTransitions(items []workflow.Transition) []workflow.Transition {
	out := make([]workflow.Transition, 0, len(items))
	for i, item := range items {
		transition := workflow.Transition{
			ID:         strings.TrimSpace(item.ID),
			Name:       strings.TrimSpace(item.Name),
			FromStatus: workflow.NormalizeStatusID(item.FromStatus),
			ToStatus:   workflow.NormalizeStatusID(item.ToStatus),
			Action:     strings.TrimSpace(item.Action),
			OrderIndex: item.OrderIndex,
		}
		if transition.OrderIndex == 0 {
			transition.OrderIndex = i + 1
		}
		out = append(out, transition)
	}
	return out
}

func validateWorkflow(item workflow.Workflow) error {
	if strings.TrimSpace(item.Name) == "" || item.InitialStatus == "" || len(item.Stages) == 0 || len(item.Statuses) == 0 {
		return shared.ErrInvalidInput
	}
	stages := map[string]bool{}
	for _, stage := range item.Stages {
		if stage.ID == "" || stage.Name == "" || stages[stage.ID] {
			return shared.ErrInvalidInput
		}
		stages[stage.ID] = true
	}
	statuses := map[string]bool{}
	for _, status := range item.Statuses {
		if status.ID == "" || status.Name == "" || status.StageID == "" || statuses[status.ID] || !stages[status.StageID] {
			return shared.ErrInvalidInput
		}
		statuses[status.ID] = true
	}
	if !statuses[item.InitialStatus] {
		return shared.ErrInvalidInput
	}
	transitions := map[string]bool{}
	for _, transition := range item.Transitions {
		if transition.ID == "" || transition.Name == "" || transition.FromStatus == "" || transition.ToStatus == "" {
			return shared.ErrInvalidInput
		}
		if transitions[transition.ID] || !statuses[transition.FromStatus] || !statuses[transition.ToStatus] {
			return shared.ErrInvalidInput
		}
		transitions[transition.ID] = true
	}
	return nil
}
