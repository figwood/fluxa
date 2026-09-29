package gormdb

import (
	"context"
	"errors"
	"testing"
	"time"

	"fluxa-api/internal/domain/project"
	"fluxa-api/internal/shared"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newProjectTestRepo(t *testing.T) (*ProjectRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	roles := []ProjectRoleModel{
		{RoleName: "owner", RoleDesc: "owner", IsBuiltin: true},
		{RoleName: "lead", RoleDesc: "lead", IsBuiltin: true},
		{RoleName: "developer", RoleDesc: "developer", IsBuiltin: true},
	}
	if err := db.Create(&roles).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&UserModel{ID: 1, UserName: "owner", UserNameCN: "Owner", UserEmail: "owner@test", UserPassword: "x"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&UserModel{ID: 2, UserName: "dev", UserNameCN: "Dev", UserEmail: "dev@test", UserPassword: "x"}).Error; err != nil {
		t.Fatal(err)
	}
	return NewProjectRepository(db), db
}

func TestCreateWithOwnerAndMemberLifecycle(t *testing.T) {
	repo, _ := newProjectTestRepo(t)
	ctx := context.Background()
	now := time.Now()
	created, err := repo.CreateWithOwner(ctx, project.Project{ID: "proj-1", Name: "One", Key: "one", CreatedAt: now, UpdatedAt: now}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "proj-1" {
		t.Fatalf("unexpected project: %#v", created)
	}
	role, err := repo.GetMemberRole(ctx, "proj-1", 1)
	if err != nil || role != "owner" {
		t.Fatalf("owner binding missing: %q %v", role, err)
	}
	if _, err := repo.AddMember(ctx, "proj-1", project.AddMemberInput{UserID: 2, RoleName: "developer"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateMember(ctx, "proj-1", 2, project.UpdateMemberInput{RoleName: "lead"}); err != nil {
		t.Fatal(err)
	}
	if role, _ := repo.GetMemberRole(ctx, "proj-1", 2); role != "lead" {
		t.Fatalf("expected lead, got %q", role)
	}
	if err := repo.DeleteMember(ctx, "proj-1", 1); !errors.Is(err, shared.ErrConflict) {
		t.Fatalf("last owner deletion should conflict, got %v", err)
	}
	if _, err := repo.UpdateMember(ctx, "proj-1", 2, project.UpdateMemberInput{RoleName: "owner"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteMember(ctx, "proj-1", 1); err != nil {
		t.Fatal(err)
	}
}

func TestCreateWithOwnerRollsBackForMissingUser(t *testing.T) {
	repo, _ := newProjectTestRepo(t)
	now := time.Now()
	_, err := repo.CreateWithOwner(context.Background(), project.Project{ID: "proj-bad", Name: "Bad", Key: "bad", CreatedAt: now, UpdatedAt: now}, 999)
	if !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if _, err := repo.Get(context.Background(), "proj-bad"); !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("project was not rolled back: %v", err)
	}
}
