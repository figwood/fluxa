package gormdb

import (
	"context"
	"errors"
	"time"

	"fluxa-api/internal/domain/delivery"
	"fluxa-api/internal/shared"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DeliveryRepository struct{ db *gorm.DB }

func NewDeliveryRepository(db *gorm.DB) *DeliveryRepository { return &DeliveryRepository{db: db} }

func (r *DeliveryRepository) EnsureProjectDefaults(ctx context.Context, projectID string) error {
	defaults := []EnvironmentPolicyModel{
		{ProjectID: projectID, Environment: "dev", AutoDeploy: true},
		{ProjectID: projectID, Environment: "stg"},
		{ProjectID: projectID, Environment: "prod", Approval: true, CompletesTasks: true},
	}
	for _, item := range defaults {
		if err := r.db.WithContext(ctx).Where("project_id = ? AND environment = ?", projectID, item.Environment).FirstOrCreate(&item).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *DeliveryRepository) CreateArtifact(ctx context.Context, item delivery.Artifact) (delivery.Artifact, error) {
	row := artifactModel(item)
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return delivery.Artifact{}, err
	}
	return artifact(row), nil
}

func (r *DeliveryRepository) GetArtifact(ctx context.Context, id string) (delivery.Artifact, error) {
	var row ArtifactModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return delivery.Artifact{}, shared.ErrNotFound
		}
		return delivery.Artifact{}, err
	}
	return artifact(row), nil
}

func (r *DeliveryRepository) GetArtifactByIdempotencyKey(ctx context.Context, projectID, key string) (delivery.Artifact, error) {
	var row ArtifactModel
	if err := r.db.WithContext(ctx).Where("project_id = ? AND idempotency_key = ?", projectID, key).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return delivery.Artifact{}, shared.ErrNotFound
		}
		return delivery.Artifact{}, err
	}
	return artifact(row), nil
}

func (r *DeliveryRepository) ListArtifacts(ctx context.Context, projectID, serviceID string) ([]delivery.Artifact, error) {
	q := r.db.WithContext(ctx).Order("built_at desc, created_at desc")
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	if serviceID != "" {
		q = q.Where("project_service_id = ?", serviceID)
	}
	var rows []ArtifactModel
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]delivery.Artifact, 0, len(rows))
	for _, row := range rows {
		out = append(out, artifact(row))
	}
	return out, nil
}

func (r *DeliveryRepository) CreateDeployment(ctx context.Context, item delivery.Deployment) (delivery.Deployment, error) {
	row := deploymentModel(item)
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return delivery.Deployment{}, err
	}
	return deployment(row), nil
}

func (r *DeliveryRepository) UpdateDeployment(ctx context.Context, item delivery.Deployment) error {
	return r.db.WithContext(ctx).Model(&DeploymentModel{}).Where("id = ?", item.ID).Updates(map[string]any{"status": item.Status, "external_id": item.ExternalID, "external_url": item.ExternalURL, "message": item.Message, "started_at": item.StartedAt, "finished_at": item.FinishedAt, "updated_at": item.UpdatedAt}).Error
}

func (r *DeliveryRepository) ListDeployments(ctx context.Context, projectID string) ([]delivery.Deployment, error) {
	var rows []DeploymentModel
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]delivery.Deployment, 0, len(rows))
	for _, row := range rows {
		out = append(out, deployment(row))
	}
	return out, nil
}

func (r *DeliveryRepository) GetPolicy(ctx context.Context, projectID, environment string) (delivery.EnvironmentPolicy, error) {
	if err := r.EnsureProjectDefaults(ctx, projectID); err != nil {
		return delivery.EnvironmentPolicy{}, err
	}
	var row EnvironmentPolicyModel
	if err := r.db.WithContext(ctx).Where("project_id = ? AND environment = ?", projectID, environment).First(&row).Error; err != nil {
		return delivery.EnvironmentPolicy{}, err
	}
	return policy(row), nil
}

func (r *DeliveryRepository) ListPolicies(ctx context.Context, projectID string) ([]delivery.EnvironmentPolicy, error) {
	if err := r.EnsureProjectDefaults(ctx, projectID); err != nil {
		return nil, err
	}
	var rows []EnvironmentPolicyModel
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]delivery.EnvironmentPolicy, 0, len(rows))
	for _, row := range rows {
		out = append(out, policy(row))
	}
	return out, nil
}

func (r *DeliveryRepository) SetPolicies(ctx context.Context, projectID string, items []delivery.EnvironmentPolicy) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			row := EnvironmentPolicyModel{ProjectID: projectID, Environment: item.Environment, AutoDeploy: item.AutoDeploy, Approval: item.Approval, CompletesTasks: item.CompletesTasks}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "project_id"}, {Name: "environment"}}, DoUpdates: clause.AssignmentColumns([]string{"auto_deploy", "approval", "completes_tasks"})}).Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *DeliveryRepository) SetCITokenHash(ctx context.Context, projectID, tokenHash string) error {
	row := ProjectCITokenModel{ProjectID: projectID, TokenHash: tokenHash, UpdatedAt: time.Now()}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "project_id"}}, DoUpdates: clause.AssignmentColumns([]string{"token_hash", "updated_at"})}).Create(&row).Error
}
func (r *DeliveryRepository) FindProjectByCITokenHash(ctx context.Context, tokenHash string) (string, error) {
	var row ProjectCITokenModel
	if err := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", shared.ErrUnauthorized
		}
		return "", err
	}
	return row.ProjectID, nil
}

func artifactModel(x delivery.Artifact) ArtifactModel {
	return ArtifactModel{ID: x.ID, ProjectID: x.ProjectID, ProjectServiceID: x.ProjectServiceID, CommitSHA: x.CommitSHA, RefType: x.RefType, Ref: x.Ref, ImageRepository: x.ImageRepository, ImageTag: x.ImageTag, ImageDigest: x.ImageDigest, PipelineID: x.PipelineID, PipelineURL: x.PipelineURL, IdempotencyKey: x.IdempotencyKey, BuiltAt: x.BuiltAt, CreatedAt: x.CreatedAt}
}
func artifact(x ArtifactModel) delivery.Artifact {
	return delivery.Artifact{ID: x.ID, ProjectID: x.ProjectID, ProjectServiceID: x.ProjectServiceID, CommitSHA: x.CommitSHA, RefType: x.RefType, Ref: x.Ref, ImageRepository: x.ImageRepository, ImageTag: x.ImageTag, ImageDigest: x.ImageDigest, PipelineID: x.PipelineID, PipelineURL: x.PipelineURL, IdempotencyKey: x.IdempotencyKey, BuiltAt: x.BuiltAt, CreatedAt: x.CreatedAt}
}
func deploymentModel(x delivery.Deployment) DeploymentModel {
	return DeploymentModel{ExecutionID: x.ExecutionID, StepID: x.StepID, Attempt: x.Attempt, ID: x.ID, ProjectID: x.ProjectID, ProjectServiceID: x.ProjectServiceID, ArtifactID: x.ArtifactID, ReleaseID: x.ReleaseID, Environment: x.Environment, Source: x.Source, Status: x.Status, ExternalID: x.ExternalID, ExternalURL: x.ExternalURL, Message: x.Message, StartedAt: x.StartedAt, FinishedAt: x.FinishedAt, CreatedAt: x.CreatedAt, UpdatedAt: x.UpdatedAt}
}
func deployment(x DeploymentModel) delivery.Deployment {
	return delivery.Deployment{ExecutionID: x.ExecutionID, StepID: x.StepID, Attempt: x.Attempt, ID: x.ID, ProjectID: x.ProjectID, ProjectServiceID: x.ProjectServiceID, ArtifactID: x.ArtifactID, ReleaseID: x.ReleaseID, Environment: x.Environment, Source: x.Source, Status: x.Status, ExternalID: x.ExternalID, ExternalURL: x.ExternalURL, Message: x.Message, StartedAt: x.StartedAt, FinishedAt: x.FinishedAt, CreatedAt: x.CreatedAt, UpdatedAt: x.UpdatedAt}
}
func policy(x EnvironmentPolicyModel) delivery.EnvironmentPolicy {
	return delivery.EnvironmentPolicy{ProjectID: x.ProjectID, Environment: x.Environment, AutoDeploy: x.AutoDeploy, Approval: x.Approval, CompletesTasks: x.CompletesTasks}
}
