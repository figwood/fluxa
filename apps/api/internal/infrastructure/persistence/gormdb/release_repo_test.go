package gormdb

import (
	"context"
	"errors"
	"testing"
	"time"

	"fluxa-api/internal/domain/release"
	"fluxa-api/internal/shared"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestReleaseRepositoryStoresEvents(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	repo := NewReleaseRepository(db)
	ctx := context.Background()
	now := time.Now()
	rel, err := repo.Create(ctx, release.Release{
		ID: "rel_1", ProjectID: "proj_1", Title: "发布单", Environment: "test",
		Status: release.ReleaseDraft, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create release: %v", err)
	}
	events := []release.ReleaseEvent{
		{ID: "event_1", ReleaseID: rel.ID, Type: release.EventReleaseCreated, Title: "创建发布单", CreatedAt: now.Add(time.Second)},
		{ID: "event_2", ReleaseID: rel.ID, Type: release.EventReleaseSubmitted, Title: "提交审批", FromStatus: "draft", ToStatus: "pending_approval", CreatedAt: now.Add(2 * time.Second)},
	}
	for _, event := range events {
		if err := repo.AddEvent(ctx, event); err != nil {
			t.Fatalf("add event: %v", err)
		}
	}
	full, err := repo.GetFull(ctx, rel.ID)
	if err != nil {
		t.Fatalf("get full release: %v", err)
	}
	if len(full.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(full.Events))
	}
	if full.Events[0].ID != "event_1" || full.Events[1].ID != "event_2" {
		t.Fatalf("events were not returned in created order: %#v", full.Events)
	}
	if full.Events[1].FromStatus != "draft" || full.Events[1].ToStatus != "pending_approval" {
		t.Fatalf("status change metadata was not persisted: %#v", full.Events[1])
	}
}

func TestCreateWithEventRollsBackWhenEventFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	repo := NewReleaseRepository(db)
	ctx := context.Background()
	now := time.Now()
	if err := repo.AddEvent(ctx, release.ReleaseEvent{
		ID: "event_duplicate", ReleaseID: "rel_existing", Type: release.EventReleaseCreated, Title: "existing", CreatedAt: now,
	}); err != nil {
		t.Fatalf("seed event: %v", err)
	}
	_, err = repo.CreateWithEvent(ctx, release.Release{
		ID: "rel_rollback", ProjectID: "proj_1", Title: "发布单", Environment: "test",
		Status: release.ReleaseDraft, CreatedAt: now, UpdatedAt: now,
	}, release.ReleaseEvent{
		ID: "event_duplicate", ReleaseID: "rel_rollback", Type: release.EventReleaseCreated, Title: "创建发布单", CreatedAt: now,
	})
	if err == nil {
		t.Fatal("expected duplicate event error")
	}
	if _, err := repo.Get(ctx, "rel_rollback"); !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("release should have been rolled back, got %v", err)
	}
}

func TestTransitionStatusRequiresExpectedCurrentStatus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	repo := NewReleaseRepository(db)
	ctx := context.Background()
	now := time.Now()
	if _, err := repo.Create(ctx, release.Release{
		ID: "rel_transition", ProjectID: "proj_1", Title: "发布单", Environment: "test",
		Status: release.ReleaseDraft, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create release: %v", err)
	}
	if _, err := repo.TransitionStatus(ctx, "rel_transition", release.ReleaseApproved, release.ReleaseQueued, release.ReleaseEvent{
		ID: "event_bad_transition", ReleaseID: "rel_transition", Type: release.EventReleaseStatusChanged, Title: "状态流转", CreatedAt: now,
	}); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
	full, err := repo.TransitionStatus(ctx, "rel_transition", release.ReleaseDraft, release.ReleasePendingApproval, release.ReleaseEvent{
		ID: "event_transition", ReleaseID: "rel_transition", Type: release.EventReleaseSubmitted, Title: "提交审批", CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("transition status: %v", err)
	}
	if full.Status != release.ReleasePendingApproval || len(full.Events) != 1 {
		t.Fatalf("transition was not persisted atomically: %#v", full)
	}
}

func TestEnqueueIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	repo := NewReleaseRepository(db)
	ctx := context.Background()
	now := time.Now()
	if _, err := repo.Create(ctx, release.Release{
		ID: "rel_enqueue", ProjectID: "proj_1", Title: "发布单", Environment: "test",
		Status: release.ReleaseApproved, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create release: %v", err)
	}
	first, created, err := repo.Enqueue(ctx, "rel_enqueue", []release.ReleaseStatus{release.ReleaseApproved}, release.ReleaseJob{
		ID: "job_first", ReleaseID: "rel_enqueue", Status: release.JobPending, CreatedAt: now, UpdatedAt: now,
	}, release.ReleaseEvent{
		ID: "event_enqueue", ReleaseID: "rel_enqueue", Type: release.EventReleasePublishQueued, Title: "发布入队", CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if !created || first.ID != "job_first" {
		t.Fatalf("expected first enqueue to create job, got created=%v job=%#v", created, first)
	}
	second, created, err := repo.Enqueue(ctx, "rel_enqueue", []release.ReleaseStatus{release.ReleaseApproved}, release.ReleaseJob{
		ID: "job_second", ReleaseID: "rel_enqueue", Status: release.JobPending, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
	}, release.ReleaseEvent{
		ID: "event_enqueue_second", ReleaseID: "rel_enqueue", Type: release.EventReleasePublishQueued, Title: "发布入队", CreatedAt: now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("second enqueue: %v", err)
	}
	if created || second.ID != first.ID {
		t.Fatalf("expected second enqueue to return existing job, got created=%v job=%#v", created, second)
	}
	var count int64
	if err := db.Model(&ReleaseJobModel{}).Where("release_id = ?", "rel_enqueue").Count(&count).Error; err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one job, got %d", count)
	}
	full, err := repo.GetFull(ctx, "rel_enqueue")
	if err != nil {
		t.Fatalf("get full: %v", err)
	}
	if full.Status != release.ReleaseQueued || len(full.Events) != 1 {
		t.Fatalf("release enqueue state not persisted correctly: %#v", full)
	}
}
