package projectapp

import (
	"context"
	"strings"
	"time"

	"fluxa-api/internal/application/authorization"
	"fluxa-api/internal/domain/servicecatalog"
	"fluxa-api/internal/shared"
)

type CatalogService struct {
	repo   servicecatalog.Repository
	access *authorization.Service
	gitlab servicecatalog.GitlabProvider
}

func NewCatalog(repo servicecatalog.Repository, access *authorization.Service, gitlab servicecatalog.GitlabProvider) *CatalogService {
	return &CatalogService{repo: repo, access: access, gitlab: gitlab}
}

func (s *CatalogService) CreateGitlabRepository(ctx context.Context, in servicecatalog.CreateGitlabRepositoryInput) (servicecatalog.GitlabRepository, error) {
	if !authorization.IsAdmin(ctx) {
		return servicecatalog.GitlabRepository{}, shared.ErrForbidden
	}
	if in.GitlabProjectID <= 0 || strings.TrimSpace(in.Name) == "" {
		return servicecatalog.GitlabRepository{}, shared.ErrInvalidInput
	}
	branch := strings.TrimSpace(in.DefaultBranch)
	if branch == "" {
		branch = "main"
	}
	now := time.Now()
	return s.repo.CreateGitlabRepository(ctx, servicecatalog.GitlabRepository{
		ID: shared.NewID("glrepo"), GitlabProjectID: in.GitlabProjectID, Name: strings.TrimSpace(in.Name),
		Path: strings.TrimSpace(in.Path), URL: strings.TrimSpace(in.URL), DefaultBranch: branch,
		CreatedAt: now, UpdatedAt: now,
	})
}

func (s *CatalogService) ListGitlabRepositories(ctx context.Context) ([]servicecatalog.GitlabRepository, error) {
	return s.repo.ListGitlabRepositories(ctx)
}

func (s *CatalogService) SearchGitlabProjects(ctx context.Context, keyword string) ([]servicecatalog.RemoteGitlabProject, error) {
	return s.gitlab.SearchProjects(ctx, keyword)
}

func (s *CatalogService) ListProjectGitlabRepositories(ctx context.Context, projectID string) ([]servicecatalog.GitlabRepository, error) {
	if err := s.access.Require(ctx, projectID, authorization.RoleDeveloper); err != nil {
		return nil, err
	}
	return s.repo.ListProjectGitlabRepositories(ctx, projectID)
}

func (s *CatalogService) AttachProjectGitlabRepository(ctx context.Context, projectID string, in servicecatalog.AttachGitlabRepositoryInput) (servicecatalog.GitlabRepository, error) {
	if err := s.access.Require(ctx, projectID, authorization.RoleLead); err != nil {
		return servicecatalog.GitlabRepository{}, err
	}
	if in.GitlabProjectID <= 0 {
		return servicecatalog.GitlabRepository{}, shared.ErrInvalidInput
	}
	remote, err := s.gitlab.GetProject(ctx, in.GitlabProjectID)
	if err != nil {
		return servicecatalog.GitlabRepository{}, err
	}
	if remote.ID <= 0 || strings.TrimSpace(remote.Name) == "" {
		return servicecatalog.GitlabRepository{}, shared.ErrInvalidInput
	}
	branch := strings.TrimSpace(remote.DefaultBranch)
	if branch == "" {
		branch = "main"
	}
	now := time.Now()
	item, err := s.repo.UpsertGitlabRepository(ctx, servicecatalog.GitlabRepository{
		ID: shared.NewID("glrepo"), GitlabProjectID: remote.ID, Name: strings.TrimSpace(remote.Name),
		Path: strings.TrimSpace(remote.PathWithNamespace), URL: strings.TrimSpace(remote.WebURL),
		DefaultBranch: branch, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return servicecatalog.GitlabRepository{}, err
	}
	if err := s.repo.AttachGitlabRepository(ctx, projectID, item.GitlabProjectID); err != nil {
		return servicecatalog.GitlabRepository{}, err
	}
	return item, nil
}

func (s *CatalogService) DetachProjectGitlabRepository(ctx context.Context, projectID string, gitlabProjectID int64) error {
	if err := s.access.Require(ctx, projectID, authorization.RoleLead); err != nil {
		return err
	}
	if gitlabProjectID <= 0 {
		return shared.ErrInvalidInput
	}
	return s.repo.DetachGitlabRepository(ctx, projectID, gitlabProjectID)
}

func (s *CatalogService) CreateProjectService(ctx context.Context, in servicecatalog.CreateProjectServiceInput) (servicecatalog.ProjectService, error) {
	target := servicecatalog.DeployTargetType(strings.ToLower(strings.TrimSpace(in.DeployTarget)))
	if target != servicecatalog.DeployTargetJenkins && target != servicecatalog.DeployTargetPortainer {
		return servicecatalog.ProjectService{}, shared.ErrInvalidInput
	}
	if strings.TrimSpace(in.ProjectID) == "" || in.GitlabProjectID <= 0 || strings.TrimSpace(in.ServiceKey) == "" {
		return servicecatalog.ProjectService{}, shared.ErrInvalidInput
	}
	if err := s.access.Require(ctx, in.ProjectID, authorization.RoleLead); err != nil {
		return servicecatalog.ProjectService{}, err
	}
	if _, err := s.repo.GetGitlabRepositoryByProjectID(ctx, in.GitlabProjectID); err != nil {
		return servicecatalog.ProjectService{}, err
	}
	attached, err := s.repo.IsGitlabRepositoryAttached(ctx, in.ProjectID, in.GitlabProjectID)
	if err != nil {
		return servicecatalog.ProjectService{}, err
	}
	if !attached {
		return servicecatalog.ProjectService{}, shared.ErrConflict
	}
	now := time.Now()
	config := in.DeployConfig
	if config == nil {
		config = map[string]any{}
	}
	return s.repo.CreateProjectService(ctx, servicecatalog.ProjectService{
		ID: shared.NewID("svc"), ProjectID: in.ProjectID, GitlabProjectID: in.GitlabProjectID,
		ServiceKey: strings.TrimSpace(in.ServiceKey), DisplayName: strings.TrimSpace(in.DisplayName),
		ModulePath: strings.TrimSpace(in.ModulePath), ImageName: strings.TrimSpace(in.ImageName),
		DeployTarget: target, DeployConfig: config, Status: "active", CreatedAt: now, UpdatedAt: now,
	})
}

func (s *CatalogService) ListProjectServices(ctx context.Context, projectID string) ([]servicecatalog.ProjectService, error) {
	if projectID != "" {
		if err := s.access.Require(ctx, projectID, authorization.RoleDeveloper); err != nil {
			return nil, err
		}
		return s.repo.ListProjectServices(ctx, projectID)
	}
	items, err := s.repo.ListProjectServices(ctx, "")
	if err != nil || authorization.IsAdmin(ctx) {
		return items, err
	}
	out := make([]servicecatalog.ProjectService, 0, len(items))
	for _, item := range items {
		if s.access.CanRead(ctx, item.ProjectID) {
			out = append(out, item)
		}
	}
	return out, nil
}
