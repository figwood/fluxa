package projectapp

import (
	"context"
	"strings"
	"time"

	"fluxa-api/internal/application/authorization"
	"fluxa-api/internal/domain/project"
	"fluxa-api/internal/shared"
)

type Service struct {
	repo   project.Repository
	access *authorization.Service
}

func New(repo project.Repository, access *authorization.Service) *Service {
	return &Service{repo: repo, access: access}
}

func (s *Service) Create(ctx context.Context, in project.CreateProjectInput) (project.Project, error) {
	name := strings.TrimSpace(in.Name)
	key := strings.ToLower(strings.TrimSpace(in.Key))
	if name == "" || key == "" {
		return project.Project{}, shared.ErrInvalidInput
	}
	now := time.Now()
	principal, err := authorization.Principal(ctx)
	if err != nil {
		return project.Project{}, err
	}
	created, err := s.repo.CreateWithOwner(ctx, project.Project{
		ID: shared.NewID("proj"), Name: name, Key: key, Description: strings.TrimSpace(in.Description),
		CreatedAt: now, UpdatedAt: now,
	}, principal.UserID)
	created.CurrentUserRole = authorization.RoleOwner
	return created, err
}

func (s *Service) List(ctx context.Context) ([]project.Project, error) {
	if authorization.IsAdmin(ctx) {
		items, err := s.repo.List(ctx)
		for i := range items {
			items[i].CurrentUserRole = authorization.RoleOwner
		}
		return items, err
	}
	principal, err := authorization.Principal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.ListForUser(ctx, principal.UserID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].CurrentUserRole, err = s.repo.GetMemberRole(ctx, items[i].ID, principal.UserID)
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Service) Get(ctx context.Context, projectID string) (project.Project, error) {
	if err := s.access.Require(ctx, projectID, authorization.RoleDeveloper); err != nil {
		return project.Project{}, err
	}
	item, err := s.repo.Get(ctx, projectID)
	if err != nil {
		return project.Project{}, err
	}
	if authorization.IsAdmin(ctx) {
		item.CurrentUserRole = authorization.RoleOwner
		return item, nil
	}
	principal, err := authorization.Principal(ctx)
	if err != nil {
		return project.Project{}, err
	}
	item.CurrentUserRole, err = s.repo.GetMemberRole(ctx, projectID, principal.UserID)
	return item, err
}

func (s *Service) ListMembers(ctx context.Context, projectID string) (project.MemberList, error) {
	if err := s.access.Require(ctx, projectID, authorization.RoleDeveloper); err != nil {
		return project.MemberList{}, err
	}
	items, err := s.repo.ListMembers(ctx, projectID)
	if err != nil {
		return project.MemberList{}, err
	}
	return project.MemberList{Items: items, CanManage: s.access.Require(ctx, projectID, authorization.RoleOwner) == nil}, nil
}

func (s *Service) ListMemberCandidates(ctx context.Context, projectID string) ([]project.MemberCandidate, error) {
	if err := s.access.Require(ctx, projectID, authorization.RoleOwner); err != nil {
		return nil, err
	}
	return s.repo.ListMemberCandidates(ctx, projectID)
}

func (s *Service) AddMember(ctx context.Context, projectID string, in project.AddMemberInput) (project.Member, error) {
	if err := s.access.Require(ctx, projectID, authorization.RoleOwner); err != nil {
		return project.Member{}, err
	}
	return s.repo.AddMember(ctx, projectID, in)
}

func (s *Service) UpdateMember(ctx context.Context, projectID string, userID int64, in project.UpdateMemberInput) (project.Member, error) {
	if err := s.access.Require(ctx, projectID, authorization.RoleOwner); err != nil {
		return project.Member{}, err
	}
	return s.repo.UpdateMember(ctx, projectID, userID, in)
}

func (s *Service) DeleteMember(ctx context.Context, projectID string, userID int64) error {
	if err := s.access.Require(ctx, projectID, authorization.RoleOwner); err != nil {
		return err
	}
	return s.repo.DeleteMember(ctx, projectID, userID)
}
