package delivery

import "context"

type Repository interface {
	EnsureProjectDefaults(ctx context.Context, projectID string) error
	CreateArtifact(ctx context.Context, item Artifact) (Artifact, error)
	GetArtifact(ctx context.Context, id string) (Artifact, error)
	GetArtifactByIdempotencyKey(ctx context.Context, projectID, key string) (Artifact, error)
	ListArtifacts(ctx context.Context, projectID, serviceID string) ([]Artifact, error)
	CreateDeployment(ctx context.Context, item Deployment) (Deployment, error)
	UpdateDeployment(ctx context.Context, item Deployment) error
	ListDeployments(ctx context.Context, projectID string) ([]Deployment, error)
	GetPolicy(ctx context.Context, projectID, environment string) (EnvironmentPolicy, error)
	ListPolicies(ctx context.Context, projectID string) ([]EnvironmentPolicy, error)
	SetPolicies(ctx context.Context, projectID string, items []EnvironmentPolicy) error
	SetCITokenHash(ctx context.Context, projectID, tokenHash string) error
	FindProjectByCITokenHash(ctx context.Context, tokenHash string) (string, error)
}
