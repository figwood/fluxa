package gormdb

import (
	"context"
	"errors"
	"testing"
	"time"

	"fluxa-api/internal/domain/workflow"
	"fluxa-api/internal/shared"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestProjectWorkflowsAreIndependent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := db.Create([]ProjectModel{{ID: "one", Name: "One", Key: "one", CreatedAt: now, UpdatedAt: now}, {ID: "two", Name: "Two", Key: "two", CreatedAt: now, UpdatedAt: now}}).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewWorkflowRepository(db)
	ctx := context.Background()
	if err := repo.EnsureDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	one, err := repo.Get(ctx, "one", workflow.KindTask)
	if err != nil {
		t.Fatal(err)
	}
	one.Name = "One task flow"
	if _, err := repo.Save(ctx, one); err != nil {
		t.Fatal(err)
	}
	two, err := repo.Get(ctx, "two", workflow.KindTask)
	if err != nil {
		t.Fatal(err)
	}
	if two.Name == one.Name {
		t.Fatalf("workflow update leaked between projects: %#v", two)
	}
}

func TestWorkflowCannotRemoveStatusInUse(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := db.Create(&ProjectModel{ID: "one", Name: "One", Key: "one", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewWorkflowRepository(db)
	ctx := context.Background()
	if err := repo.EnsureDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	item, _ := repo.Get(ctx, "one", workflow.KindTask)
	if err := db.Create(&TaskModel{ID: "task-1", ProjectID: "one", Title: "Task", Status: "todo", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	item.Statuses = item.Statuses[1:]
	if _, err := repo.Save(ctx, item); !errors.Is(err, shared.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}
