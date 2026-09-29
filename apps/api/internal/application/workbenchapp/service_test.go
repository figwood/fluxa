package workbenchapp

import (
	"context"
	"testing"
	"time"

	"fluxa-api/internal/domain/project"
	"fluxa-api/internal/domain/release"
	"fluxa-api/internal/domain/task"
	"fluxa-api/internal/security"
)

type projectStub []project.Project

func (s projectStub) List(context.Context) ([]project.Project, error) { return s, nil }

type taskStub []task.Task

func (s taskStub) List(context.Context, string, task.Status) ([]task.Task, error) { return s, nil }

type releaseStub []release.Release

func (s releaseStub) List(context.Context, string) ([]release.Release, error) { return s, nil }

func TestWorkbenchReturnsOnlyActionableItems(t *testing.T) {
	now := time.Now()
	ctx := security.WithPrincipal(context.Background(), security.Principal{UserID: 7, UserName: "alice"})
	svc := New(
		projectStub{{ID: "lead", CurrentUserRole: "lead"}, {ID: "dev", CurrentUserRole: "developer"}},
		taskStub{
			{ID: "mine", ProjectID: "lead", Assignee: "alice", Status: task.StatusInProgress, UpdatedAt: now},
			{ID: "other", ProjectID: "lead", Assignee: "bob", Status: task.StatusTodo, UpdatedAt: now},
			{ID: "done", ProjectID: "lead", Assignee: "alice", Status: task.StatusDone, UpdatedAt: now},
		},
		releaseStub{
			{ID: "approve", ProjectID: "lead", Status: release.ReleasePendingApproval, UpdatedAt: now},
			{ID: "hidden-approve", ProjectID: "dev", Status: release.ReleasePendingApproval, UpdatedAt: now},
			{ID: "running", ProjectID: "dev", Status: release.ReleaseRunning, UpdatedAt: now},
			{ID: "failed", ProjectID: "lead", Status: release.ReleaseFailed, UpdatedAt: now},
		},
	)

	result, err := svc.Get(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Counts.MyOpenTasks != 1 || result.MyTasks[0].ID != "mine" {
		t.Fatalf("unexpected tasks: %#v", result.MyTasks)
	}
	if result.Counts.PendingApprovals != 1 || result.PendingApprovals[0].ID != "approve" {
		t.Fatalf("unexpected approvals: %#v", result.PendingApprovals)
	}
	if result.Counts.ActiveReleases != 1 || result.Counts.FailedReleases != 1 {
		t.Fatalf("unexpected counts: %#v", result.Counts)
	}
}
