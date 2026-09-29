package gormdb

import (
	"context"
	"errors"

	"fluxa-api/internal/domain/servicecatalog"
	"fluxa-api/internal/shared"

	"gorm.io/gorm"
)

type ServiceCatalogRepository struct{ db *gorm.DB }

func NewServiceCatalogRepository(db *gorm.DB) *ServiceCatalogRepository {
	return &ServiceCatalogRepository{db: db}
}

func (r *ServiceCatalogRepository) CreateGitlabRepository(ctx context.Context, item servicecatalog.GitlabRepository) (servicecatalog.GitlabRepository, error) {
	row := GitlabRepositoryModel{
		ID: item.ID, GitlabProjectID: item.GitlabProjectID, Name: item.Name, Path: item.Path,
		URL: item.URL, DefaultBranch: item.DefaultBranch, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return servicecatalog.GitlabRepository{}, err
	}
	return toGitlabRepository(row), nil
}

func (r *ServiceCatalogRepository) UpsertGitlabRepository(ctx context.Context, item servicecatalog.GitlabRepository) (servicecatalog.GitlabRepository, error) {
	var row GitlabRepositoryModel
	err := r.db.WithContext(ctx).Where("gitlab_project_id = ?", item.GitlabProjectID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.CreateGitlabRepository(ctx, item)
	}
	if err != nil {
		return servicecatalog.GitlabRepository{}, err
	}
	updates := map[string]any{
		"name": item.Name, "path": item.Path, "url": item.URL,
		"default_branch": item.DefaultBranch, "updated_at": item.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Model(&row).Updates(updates).Error; err != nil {
		return servicecatalog.GitlabRepository{}, err
	}
	return r.GetGitlabRepositoryByProjectID(ctx, item.GitlabProjectID)
}

func (r *ServiceCatalogRepository) ListGitlabRepositories(ctx context.Context) ([]servicecatalog.GitlabRepository, error) {
	var rows []GitlabRepositoryModel
	if err := r.db.WithContext(ctx).Order("created_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]servicecatalog.GitlabRepository, 0, len(rows))
	for _, row := range rows {
		out = append(out, toGitlabRepository(row))
	}
	return out, nil
}

func (r *ServiceCatalogRepository) GetGitlabRepositoryByProjectID(ctx context.Context, gitlabProjectID int64) (servicecatalog.GitlabRepository, error) {
	var row GitlabRepositoryModel
	if err := r.db.WithContext(ctx).Where("gitlab_project_id = ?", gitlabProjectID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return servicecatalog.GitlabRepository{}, shared.ErrNotFound
		}
		return servicecatalog.GitlabRepository{}, err
	}
	return toGitlabRepository(row), nil
}

func (r *ServiceCatalogRepository) ListProjectGitlabRepositories(ctx context.Context, projectID string) ([]servicecatalog.GitlabRepository, error) {
	var rows []GitlabRepositoryModel
	err := r.db.WithContext(ctx).Table("gitlab_repositories gr").Select("gr.*").
		Joins("JOIN project_gitlab_repositories pgr ON pgr.gitlab_project_id = gr.gitlab_project_id").
		Where("pgr.project_id = ?", projectID).Order("gr.name, gr.gitlab_project_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]servicecatalog.GitlabRepository, 0, len(rows))
	for _, row := range rows {
		out = append(out, toGitlabRepository(row))
	}
	return out, nil
}

func (r *ServiceCatalogRepository) AttachGitlabRepository(ctx context.Context, projectID string, gitlabProjectID int64) error {
	row := ProjectGitlabRepositoryModel{ProjectID: projectID, GitlabProjectID: gitlabProjectID}
	return r.db.WithContext(ctx).Where("project_id = ? AND gitlab_project_id = ?", projectID, gitlabProjectID).
		FirstOrCreate(&row).Error
}

func (r *ServiceCatalogRepository) DetachGitlabRepository(ctx context.Context, projectID string, gitlabProjectID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var dependencies int64
		if err := tx.Model(&ProjectServiceModel{}).Where("project_id = ? AND gitlab_project_id = ?", projectID, gitlabProjectID).
			Count(&dependencies).Error; err != nil {
			return err
		}
		if dependencies > 0 {
			return shared.ErrConflict
		}
		res := tx.Where("project_id = ? AND gitlab_project_id = ?", projectID, gitlabProjectID).
			Delete(&ProjectGitlabRepositoryModel{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return shared.ErrNotFound
		}
		return nil
	})
}

func (r *ServiceCatalogRepository) IsGitlabRepositoryAttached(ctx context.Context, projectID string, gitlabProjectID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&ProjectGitlabRepositoryModel{}).
		Where("project_id = ? AND gitlab_project_id = ?", projectID, gitlabProjectID).Count(&count).Error
	return count > 0, err
}

func (r *ServiceCatalogRepository) BackfillProjectGitlabRepositories(ctx context.Context) error {
	var services []ProjectServiceModel
	if err := r.db.WithContext(ctx).Select("project_id, gitlab_project_id").
		Group("project_id, gitlab_project_id").Find(&services).Error; err != nil {
		return err
	}
	for _, service := range services {
		if err := r.AttachGitlabRepository(ctx, service.ProjectID, service.GitlabProjectID); err != nil {
			return err
		}
	}
	return nil
}

func (r *ServiceCatalogRepository) CreateProjectService(ctx context.Context, item servicecatalog.ProjectService) (servicecatalog.ProjectService, error) {
	row := ReleaseProjectServiceModel(item)
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return servicecatalog.ProjectService{}, err
	}
	return toProjectService(row), nil
}

func (r *ServiceCatalogRepository) ListProjectServices(ctx context.Context, projectID string) ([]servicecatalog.ProjectService, error) {
	var rows []ProjectServiceModel
	q := r.db.WithContext(ctx).Order("created_at desc")
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]servicecatalog.ProjectService, 0, len(rows))
	for _, row := range rows {
		out = append(out, toProjectService(row))
	}
	return out, nil
}

func (r *ServiceCatalogRepository) GetProjectService(ctx context.Context, id string) (servicecatalog.ProjectService, error) {
	var row ProjectServiceModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return servicecatalog.ProjectService{}, shared.ErrNotFound
		}
		return servicecatalog.ProjectService{}, err
	}
	return toProjectService(row), nil
}

func ReleaseProjectServiceModel(item servicecatalog.ProjectService) ProjectServiceModel {
	return ProjectServiceModel{
		ID: item.ID, ProjectID: item.ProjectID, GitlabProjectID: item.GitlabProjectID,
		ServiceKey: item.ServiceKey, DisplayName: item.DisplayName, ModulePath: item.ModulePath,
		ImageName: item.ImageName, DeployTarget: string(item.DeployTarget), DeployConfig: JSONMap(item.DeployConfig),
		Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func toGitlabRepository(row GitlabRepositoryModel) servicecatalog.GitlabRepository {
	return servicecatalog.GitlabRepository{
		ID: row.ID, GitlabProjectID: row.GitlabProjectID, Name: row.Name, Path: row.Path,
		URL: row.URL, DefaultBranch: row.DefaultBranch, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func toProjectService(row ProjectServiceModel) servicecatalog.ProjectService {
	return servicecatalog.ProjectService{
		ID: row.ID, ProjectID: row.ProjectID, GitlabProjectID: row.GitlabProjectID,
		ServiceKey: row.ServiceKey, DisplayName: row.DisplayName, ModulePath: row.ModulePath,
		ImageName: row.ImageName, DeployTarget: servicecatalog.DeployTargetType(row.DeployTarget),
		DeployConfig: map[string]any(row.DeployConfig), Status: row.Status,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
