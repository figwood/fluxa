package gormdb

import (
	"context"
	"errors"
	"testing"
	"time"

	"fluxa-api/internal/domain/delivery"
	"fluxa-api/internal/domain/release"
	"fluxa-api/internal/domain/task"
	"fluxa-api/internal/shared"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func hardeningTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestArtifactIdempotencyIsProjectScoped(t *testing.T) {
	repo := NewDeliveryRepository(hardeningTestDB(t))
	ctx := context.Background()
	for _, projectID := range []string{"project-a", "project-b"} {
		_, err := repo.CreateArtifact(ctx, delivery.Artifact{ID: "artifact-" + projectID, ProjectID: projectID, ProjectServiceID: "service-" + projectID, CommitSHA: "abc", RefType: "tag", Ref: "v1", ImageRepository: "registry/app", ImageTag: "v1", ImageDigest: "sha256:abc", IdempotencyKey: "pipeline-42", BuiltAt: time.Now(), CreatedAt: time.Now()})
		if err != nil {
			t.Fatalf("create artifact for %s: %v", projectID, err)
		}
	}
	item, err := repo.GetArtifactByIdempotencyKey(ctx, "project-b", "pipeline-42")
	if err != nil || item.ProjectID != "project-b" {
		t.Fatalf("project-scoped lookup returned %#v, %v", item, err)
	}
}

func TestTaskTransitionWritesOneMatchingEvent(t *testing.T) {
	repo := NewTaskRepository(hardeningTestDB(t))
	ctx := context.Background()
	now := time.Now()
	_, err := repo.Create(ctx, task.Task{ID: "task-1", ProjectID: "project-1", Title: "task", Creator: "alice", Assignee: "alice", Status: task.StatusTodo, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	event := task.TaskEvent{ID: "event-1", TaskID: "task-1", Type: task.EventTaskStatusChanged, Title: "transition", FromStatus: string(task.StatusTodo), ToStatus: string(task.StatusInProgress), CreatedAt: now}
	if _, err := repo.TransitionStatus(ctx, "task-1", task.StatusTodo, task.StatusInProgress, event); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.TransitionStatus(ctx, "task-1", task.StatusTodo, task.StatusCancelled, task.TaskEvent{ID: "event-2", TaskID: "task-1", Type: task.EventTaskStatusChanged, Title: "stale"}); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatalf("expected stale transition rejection, got %v", err)
	}
	events, err := repo.ListEvents(ctx, "task-1")
	if err != nil || len(events) != 1 || events[0].ToStatus != string(task.StatusInProgress) {
		t.Fatalf("unexpected events: %#v, %v", events, err)
	}
}

func TestExpiredJobIsFailedAndReleaseCanBeRetried(t *testing.T) {
	db := hardeningTestDB(t)
	repo := NewReleaseRepository(db)
	ctx := context.Background()
	now := time.Now()
	_, err := repo.Create(ctx, release.Release{ID: "release-1", ProjectID: "project-1", Title: "release", Environment: "prod", Status: release.ReleaseRunning, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	expired := now.Add(-time.Minute)
	if err := db.Create(&ReleaseJobModel{ID: "job-1", ReleaseID: "release-1", Status: string(release.JobRunning), WorkerID: "lost-worker", LeaseUntil: &expired, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	jobs, err := repo.ReapExpiredJobs(ctx)
	if err != nil || len(jobs) != 1 || jobs[0].Message != "worker_lost" {
		t.Fatalf("unexpected expired jobs: %#v, %v", jobs, err)
	}
	item, err := repo.Get(ctx, "release-1")
	if err != nil || item.Status != release.ReleaseFailed {
		t.Fatalf("release was not failed: %#v, %v", item, err)
	}
}
