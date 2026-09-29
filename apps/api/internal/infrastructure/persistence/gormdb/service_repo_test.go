package gormdb

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBackfillProjectGitlabRepositories(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := db.Create(&GitlabRepositoryModel{ID: "repo-1", GitlabProjectID: 42, Name: "checkout", Path: "shop/checkout", DefaultBranch: "main", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProjectServiceModel{ID: "svc-1", ProjectID: "project-1", GitlabProjectID: 42, ServiceKey: "checkout", DisplayName: "Checkout", DeployTarget: "portainer", DeployConfig: JSONMap{}, Status: "active", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}

	repo := NewServiceCatalogRepository(db)
	if err := repo.BackfillProjectGitlabRepositories(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := repo.BackfillProjectGitlabRepositories(context.Background()); err != nil {
		t.Fatalf("backfill should be idempotent: %v", err)
	}
	items, err := repo.ListProjectGitlabRepositories(context.Background(), "project-1")
	if err != nil || len(items) != 1 || items[0].GitlabProjectID != 42 {
		t.Fatalf("unexpected backfill result: %#v, %v", items, err)
	}
	attached, err := repo.IsGitlabRepositoryAttached(context.Background(), "project-1", 42)
	if err != nil || !attached {
		t.Fatalf("expected attached repository: %v, %v", attached, err)
	}
}
