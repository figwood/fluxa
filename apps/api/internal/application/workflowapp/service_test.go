package workflowapp

import (
	"context"
	"errors"
	"testing"

	"fluxa-api/internal/domain/workflow"
	"fluxa-api/internal/shared"
)

type fakeWorkflowRepo struct {
	saved workflow.Workflow
}

func (r *fakeWorkflowRepo) EnsureDefaults(context.Context) error { return nil }
func (r *fakeWorkflowRepo) List(context.Context, string) ([]workflow.Workflow, error) {
	return []workflow.Workflow{r.saved}, nil
}
func (r *fakeWorkflowRepo) Get(context.Context, string, workflow.Kind) (workflow.Workflow, error) {
	if r.saved.Kind == "" {
		return workflow.Workflow{}, shared.ErrNotFound
	}
	return r.saved, nil
}
func (r *fakeWorkflowRepo) Save(_ context.Context, item workflow.Workflow) (workflow.Workflow, error) {
	r.saved = item
	return item, nil
}
func (r *fakeWorkflowRepo) AllowedTransitions(context.Context, string, workflow.Kind, string) ([]workflow.Transition, error) {
	return nil, nil
}

func TestUpdateRejectsTransitionWithMissingStatus(t *testing.T) {
	svc := New(&fakeWorkflowRepo{}, nil)
	_, err := svc.Update(context.Background(), "project", workflow.KindTask, workflow.UpdateWorkflowInput{
		Name:          "任务工作流",
		InitialStatus: "todo",
		Stages: []workflow.Stage{
			{ID: "backlog", Name: "待处理"},
		},
		Statuses: []workflow.Status{
			{ID: "todo", Name: "待办", StageID: "backlog"},
		},
		Transitions: []workflow.Transition{
			{ID: "start", Name: "开始", FromStatus: "todo", ToStatus: "missing"},
		},
	})
	if !errors.Is(err, shared.ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestUpdateSavesValidWorkflow(t *testing.T) {
	repo := &fakeWorkflowRepo{}
	svc := New(repo, nil)
	item, err := svc.Update(context.Background(), "project", workflow.KindTask, workflow.UpdateWorkflowInput{
		Name:          "任务工作流",
		InitialStatus: "todo",
		Stages: []workflow.Stage{
			{ID: "backlog", Name: "待处理"},
			{ID: "closed", Name: "结束"},
		},
		Statuses: []workflow.Status{
			{ID: "todo", Name: "待办", StageID: "backlog"},
			{ID: "done", Name: "完成", StageID: "closed"},
		},
		Transitions: []workflow.Transition{
			{ID: "finish", Name: "完成", FromStatus: "todo", ToStatus: "done"},
		},
	})
	if err != nil {
		t.Fatalf("expected valid workflow, got %v", err)
	}
	if item.Kind != workflow.KindTask || repo.saved.InitialStatus != "todo" {
		t.Fatalf("workflow was not saved correctly: %#v", repo.saved)
	}
}
