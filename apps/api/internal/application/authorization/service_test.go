package authorization

import (
	"context"
	"errors"
	"testing"

	"fluxa-api/internal/domain/iam"
	"fluxa-api/internal/domain/project"
	"fluxa-api/internal/security"
	"fluxa-api/internal/shared"
)

type projectRepoStub struct {
	roles map[int64]string
}

func (r *projectRepoStub) Create(context.Context, project.Project) (project.Project, error) {
	panic("unused")
}
func (r *projectRepoStub) CreateWithOwner(context.Context, project.Project, int64) (project.Project, error) {
	panic("unused")
}
func (r *projectRepoStub) List(context.Context) ([]project.Project, error) { panic("unused") }
func (r *projectRepoStub) ListForUser(context.Context, int64) ([]project.Project, error) {
	panic("unused")
}
func (r *projectRepoStub) Get(_ context.Context, id string) (project.Project, error) {
	if id != "proj-1" {
		return project.Project{}, shared.ErrNotFound
	}
	return project.Project{ID: id}, nil
}
func (r *projectRepoStub) GetMemberRole(_ context.Context, _ string, userID int64) (string, error) {
	role, ok := r.roles[userID]
	if !ok {
		return "", shared.ErrForbidden
	}
	return role, nil
}
func (r *projectRepoStub) ListMembers(context.Context, string) ([]project.Member, error) {
	panic("unused")
}
func (r *projectRepoStub) ListMemberCandidates(context.Context, string) ([]project.MemberCandidate, error) {
	panic("unused")
}
func (r *projectRepoStub) AddMember(context.Context, string, project.AddMemberInput) (project.Member, error) {
	panic("unused")
}
func (r *projectRepoStub) UpdateMember(context.Context, string, int64, project.UpdateMemberInput) (project.Member, error) {
	panic("unused")
}
func (r *projectRepoStub) DeleteMember(context.Context, string, int64) error { panic("unused") }

func principalContext(userID int64, globalRoles ...string) context.Context {
	return security.WithPrincipal(context.Background(), security.Principal{UserID: userID, GlobalRoles: globalRoles})
}

func TestRequireRoleMatrix(t *testing.T) {
	svc := New(&projectRepoStub{roles: map[int64]string{1: RoleDeveloper, 2: RoleLead, 3: RoleOwner}})
	tests := []struct {
		name    string
		userID  int64
		minimum string
		allowed bool
	}{
		{"developer reads", 1, RoleDeveloper, true},
		{"developer cannot lead", 1, RoleLead, false},
		{"lead can develop", 2, RoleDeveloper, true},
		{"lead cannot own", 2, RoleOwner, false},
		{"owner can lead", 3, RoleLead, true},
		{"non member denied", 4, RoleDeveloper, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Require(principalContext(tt.userID), "proj-1", tt.minimum)
			if tt.allowed && err != nil {
				t.Fatalf("expected access, got %v", err)
			}
			if !tt.allowed && !errors.Is(err, shared.ErrForbidden) {
				t.Fatalf("expected forbidden, got %v", err)
			}
		})
	}
}

func TestRequireAdminBypassStillChecksProject(t *testing.T) {
	svc := New(&projectRepoStub{})
	ctx := principalContext(99, iam.GlobalRoleAdmin)
	if err := svc.Require(ctx, "proj-1", RoleOwner); err != nil {
		t.Fatalf("admin denied: %v", err)
	}
	if err := svc.Require(ctx, "missing", RoleOwner); !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}
