package deliveryapp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"fluxa-api/internal/application/authorization"
	"fluxa-api/internal/domain/delivery"
	"fluxa-api/internal/domain/executor"
	"fluxa-api/internal/domain/servicecatalog"
	"fluxa-api/internal/shared"
)

type Service struct {
	repo      delivery.Repository
	catalog   servicecatalog.Repository
	access    *authorization.Service
	executors executor.Registry
}

func New(repo delivery.Repository, catalog servicecatalog.Repository, access *authorization.Service, executors executor.Registry) *Service {
	return &Service{repo: repo, catalog: catalog, access: access, executors: executors}
}

func (s *Service) RotateCIToken(ctx context.Context, projectID string) (string, error) {
	if err := s.access.Require(ctx, projectID, authorization.RoleOwner); err != nil {
		return "", err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := "flx_" + base64.RawURLEncoding.EncodeToString(b)
	if err := s.repo.SetCITokenHash(ctx, projectID, hash(token)); err != nil {
		return "", err
	}
	return token, nil
}
func (s *Service) Report(ctx context.Context, token string, in delivery.ReportArtifactInput) (delivery.ReportResult, error) {
	if strings.TrimSpace(token) == "" {
		return delivery.ReportResult{}, shared.ErrUnauthorized
	}
	projectID, err := s.repo.FindProjectByCITokenHash(ctx, hash(token))
	if err != nil {
		return delivery.ReportResult{}, err
	}
	svc, err := s.catalog.GetProjectService(ctx, in.ProjectServiceID)
	if err != nil {
		return delivery.ReportResult{}, err
	}
	if svc.ProjectID != projectID {
		return delivery.ReportResult{}, shared.ErrForbidden
	}
	if strings.TrimSpace(in.CommitSHA) == "" || strings.TrimSpace(in.ImageDigest) == "" || strings.TrimSpace(in.ImageRepository) == "" {
		return delivery.ReportResult{}, shared.ErrInvalidInput
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		key = in.ProjectServiceID + ":" + in.PipelineID + ":" + in.ImageDigest
	}
	existing, err := s.repo.GetArtifactByIdempotencyKey(ctx, projectID, key)
	if err == nil {
		return delivery.ReportResult{Artifact: existing}, nil
	}
	if !errors.Is(err, shared.ErrNotFound) {
		return delivery.ReportResult{}, err
	}
	now := time.Now()
	built := in.BuiltAt
	if built.IsZero() {
		built = now
	}
	item := delivery.Artifact{ID: shared.NewID("art"), ProjectID: projectID, ProjectServiceID: in.ProjectServiceID, CommitSHA: strings.TrimSpace(in.CommitSHA), RefType: strings.ToLower(strings.TrimSpace(in.RefType)), Ref: strings.TrimSpace(in.Ref), ImageRepository: strings.TrimSpace(in.ImageRepository), ImageTag: strings.TrimSpace(in.ImageTag), ImageDigest: strings.TrimSpace(in.ImageDigest), PipelineID: strings.TrimSpace(in.PipelineID), PipelineURL: strings.TrimSpace(in.PipelineURL), IdempotencyKey: key, BuiltAt: built, CreatedAt: now}
	created, err := s.repo.CreateArtifact(ctx, item)
	if err != nil {
		// A concurrent report may have won the composite unique-key race.
		if existing, lookupErr := s.repo.GetArtifactByIdempotencyKey(ctx, projectID, key); lookupErr == nil {
			return delivery.ReportResult{Artifact: existing}, nil
		}
		return delivery.ReportResult{}, err
	}
	result := delivery.ReportResult{Artifact: created}
	policy, err := s.repo.GetPolicy(ctx, projectID, "dev")
	if err == nil && policy.AutoDeploy && item.RefType == "branch" {
		dep, deployErr := s.deploy(ctx, svc, created, "dev", "ci_auto", "")
		if deployErr != nil {
			return delivery.ReportResult{Artifact: created, Deployment: &dep}, deployErr
		}
		result.Deployment = &dep
	}
	return result, nil
}
func (s *Service) deploy(ctx context.Context, svc servicecatalog.ProjectService, art delivery.Artifact, env, source, releaseID string) (delivery.Deployment, error) {
	now := time.Now()
	dep := delivery.Deployment{ID: shared.NewID("dep"), ProjectID: svc.ProjectID, ProjectServiceID: svc.ID, ArtifactID: art.ID, ReleaseID: releaseID, Environment: env, Source: source, Status: "running", StartedAt: &now, CreatedAt: now, UpdatedAt: now}
	created, err := s.repo.CreateDeployment(ctx, dep)
	if err != nil {
		return dep, err
	}
	exec, ok := s.executors.Get(string(svc.DeployTarget))
	if !ok {
		created.Status = "failed"
		created.Message = "executor not registered"
	} else {
		res, e := exec.Trigger(ctx, executor.DeployJob{IdempotencyKey: created.ID, ServiceID: svc.ID, Ref: art.ImageDigest, Environment: env, Config: svc.DeployConfig})
		finished := time.Now()
		created.FinishedAt = &finished
		if e != nil {
			created.Status = "failed"
			created.Message = e.Error()
		} else if res == nil || res.Status != "success" {
			created.Status = "failed"
			created.Message = "deployment did not report success"
			if res != nil {
				created.Message = res.Message
			}
		} else {
			created.Status = "success"
			created.ExternalID = res.ExternalID
			created.ExternalURL = res.ExternalURL
			created.Message = res.Message
		}
	}
	created.UpdatedAt = time.Now()
	if err := s.repo.UpdateDeployment(ctx, created); err != nil {
		return created, err
	}
	if created.Status == "failed" {
		return created, errors.New(created.Message)
	}
	return created, nil
}
func (s *Service) ListArtifacts(ctx context.Context, projectID, serviceID string) ([]delivery.Artifact, error) {
	if err := s.access.Require(ctx, projectID, authorization.RoleDeveloper); err != nil {
		return nil, err
	}
	return s.repo.ListArtifacts(ctx, projectID, serviceID)
}
func (s *Service) ListDeployments(ctx context.Context, projectID string) ([]delivery.Deployment, error) {
	if err := s.access.Require(ctx, projectID, authorization.RoleDeveloper); err != nil {
		return nil, err
	}
	return s.repo.ListDeployments(ctx, projectID)
}
func (s *Service) ListPolicies(ctx context.Context, projectID string) ([]delivery.EnvironmentPolicy, error) {
	if err := s.access.Require(ctx, projectID, authorization.RoleDeveloper); err != nil {
		return nil, err
	}
	return s.repo.ListPolicies(ctx, projectID)
}
func (s *Service) SetPolicies(ctx context.Context, projectID string, items []delivery.EnvironmentPolicy) error {
	if err := s.access.Require(ctx, projectID, authorization.RoleOwner); err != nil {
		return err
	}
	for i := range items {
		items[i].ProjectID = projectID
	}
	return s.repo.SetPolicies(ctx, projectID, items)
}
func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
