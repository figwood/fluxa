package servicecatalog

import "context"

type Repository interface {
	CreateGitlabRepository(ctx context.Context, item GitlabRepository) (GitlabRepository, error)
	UpsertGitlabRepository(ctx context.Context, item GitlabRepository) (GitlabRepository, error)
	ListGitlabRepositories(ctx context.Context) ([]GitlabRepository, error)
	GetGitlabRepositoryByProjectID(ctx context.Context, gitlabProjectID int64) (GitlabRepository, error)
	ListProjectGitlabRepositories(ctx context.Context, projectID string) ([]GitlabRepository, error)
	AttachGitlabRepository(ctx context.Context, projectID string, gitlabProjectID int64) error
	DetachGitlabRepository(ctx context.Context, projectID string, gitlabProjectID int64) error
	IsGitlabRepositoryAttached(ctx context.Context, projectID string, gitlabProjectID int64) (bool, error)
	BackfillProjectGitlabRepositories(ctx context.Context) error
	CreateProjectService(ctx context.Context, item ProjectService) (ProjectService, error)
	ListProjectServices(ctx context.Context, projectID string) ([]ProjectService, error)
	GetProjectService(ctx context.Context, id string) (ProjectService, error)
}

type GitlabProvider interface {
	SearchProjects(ctx context.Context, keyword string) ([]RemoteGitlabProject, error)
	GetProject(ctx context.Context, projectID int64) (RemoteGitlabProject, error)
}
