package authorization

import (
	"context"

	"fluxa-api/internal/domain/iam"
	"fluxa-api/internal/domain/project"
	"fluxa-api/internal/security"
	"fluxa-api/internal/shared"
)

const (
	RoleDeveloper = iam.ProjectRoleDeveloper
	RoleLead      = iam.ProjectRoleLead
	RoleOwner     = iam.ProjectRoleOwner
)

type Service struct {
	projects project.Repository
}

func New(projects project.Repository) *Service {
	return &Service{projects: projects}
}

func Principal(ctx context.Context) (security.Principal, error) {
	principal, ok := security.PrincipalFromContext(ctx)
	if !ok || principal.UserID <= 0 {
		return security.Principal{}, shared.ErrUnauthorized
	}
	return principal, nil
}

func IsAdmin(ctx context.Context) bool {
	principal, ok := security.PrincipalFromContext(ctx)
	if !ok {
		return false
	}
	for _, role := range principal.GlobalRoles {
		if role == iam.GlobalRoleAdmin {
			return true
		}
	}
	return false
}

func (s *Service) Require(ctx context.Context, projectID, minimumRole string) error {
	if projectID == "" {
		return shared.ErrInvalidInput
	}
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return err
	}
	if IsAdmin(ctx) {
		return nil
	}
	principal, err := Principal(ctx)
	if err != nil {
		return err
	}
	role, err := s.projects.GetMemberRole(ctx, projectID, principal.UserID)
	if err != nil {
		return err
	}
	if roleRank(role) < roleRank(minimumRole) {
		return shared.ErrForbidden
	}
	return nil
}

func (s *Service) CanRead(ctx context.Context, projectID string) bool {
	return s.Require(ctx, projectID, RoleDeveloper) == nil
}

func roleRank(role string) int {
	switch role {
	case RoleOwner:
		return 3
	case RoleLead:
		return 2
	case RoleDeveloper:
		return 1
	default:
		return 0
	}
}
