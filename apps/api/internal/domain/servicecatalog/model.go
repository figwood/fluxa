package servicecatalog

import "time"

type DeployTargetType string

const (
	DeployTargetJenkins   DeployTargetType = "jenkins"
	DeployTargetPortainer DeployTargetType = "portainer"
)

type GitlabRepository struct {
	ID              string    `json:"id"`
	GitlabProjectID int64     `json:"gitlab_project_id"`
	Name            string    `json:"name"`
	Path            string    `json:"path"`
	URL             string    `json:"url"`
	DefaultBranch   string    `json:"default_branch"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ProjectService struct {
	ID              string           `json:"id"`
	ProjectID       string           `json:"project_id"`
	GitlabProjectID int64            `json:"gitlab_project_id"`
	ServiceKey      string           `json:"service_key"`
	DisplayName     string           `json:"display_name"`
	ModulePath      string           `json:"module_path"`
	ImageName       string           `json:"image_name"`
	DeployTarget    DeployTargetType `json:"deploy_target"`
	DeployConfig    map[string]any   `json:"deploy_config"`
	Status          string           `json:"status"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

type CreateGitlabRepositoryInput struct {
	GitlabProjectID int64  `json:"gitlab_project_id"`
	Name            string `json:"name"`
	Path            string `json:"path"`
	URL             string `json:"url"`
	DefaultBranch   string `json:"default_branch"`
}

type AttachGitlabRepositoryInput struct {
	GitlabProjectID int64 `json:"gitlab_project_id"`
}

type RemoteGitlabProject struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	PathWithNamespace string `json:"path_with_namespace"`
	WebURL            string `json:"web_url"`
	DefaultBranch     string `json:"default_branch"`
}

type CreateProjectServiceInput struct {
	ProjectID       string         `json:"project_id"`
	GitlabProjectID int64          `json:"gitlab_project_id"`
	ServiceKey      string         `json:"service_key"`
	DisplayName     string         `json:"display_name"`
	ModulePath      string         `json:"module_path"`
	ImageName       string         `json:"image_name"`
	DeployTarget    string         `json:"deploy_target"`
	DeployConfig    map[string]any `json:"deploy_config"`
}
