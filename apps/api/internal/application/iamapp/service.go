package iamapp

import (
	"context"
	"strings"
	"time"

	"fluxa-api/internal/domain/iam"
	"fluxa-api/internal/security"
	"fluxa-api/internal/shared"
)

const minimumPasswordLength = 12

type Service struct {
	repo      iam.Repository
	jwtSecret string
	tokenTTL  time.Duration
}

func New(repo iam.Repository, jwtSecret string, tokenTTL time.Duration) *Service {
	if tokenTTL <= 0 {
		tokenTTL = 8 * time.Hour
	}
	return &Service{repo: repo, jwtSecret: jwtSecret, tokenTTL: tokenTTL}
}

func (s *Service) EnsureDefaults(ctx context.Context) error {
	return s.repo.EnsureBuiltinAccess(ctx)
}

func (s *Service) Login(ctx context.Context, in iam.LoginInput) (iam.LoginResult, error) {
	login := strings.TrimSpace(in.UserName)
	password := strings.TrimSpace(in.Password)
	if login == "" || password == "" {
		return iam.LoginResult{}, shared.ErrInvalidInput
	}
	user, hash, err := s.repo.FindUserForLogin(ctx, login)
	if err != nil {
		return iam.LoginResult{}, shared.ErrUnauthorized
	}
	if !security.VerifyPassword(hash, password) {
		return iam.LoginResult{}, shared.ErrUnauthorized
	}
	access, err := s.GetUserAccess(ctx, user.UserEmail)
	if err != nil {
		return iam.LoginResult{}, err
	}
	token, err := security.GenerateAccessToken(s.jwtSecret, s.tokenTTL, security.Claims{
		UserID: user.ID, UserName: user.UserName, UserEmail: user.UserEmail, GlobalRoles: access.GlobalRoles, AuthVersion: user.AuthVersion,
	})
	if err != nil {
		return iam.LoginResult{}, err
	}
	return iam.LoginResult{AccessToken: token, User: user, Auth: access}, nil
}

func (s *Service) ParseToken(token string) (security.Claims, error) {
	return security.ParseAccessToken(s.jwtSecret, token)
}

func (s *Service) ListUsers(ctx context.Context, limit, offset int) ([]iam.User, error) {
	return s.repo.ListUsers(ctx, limit, offset)
}

func (s *Service) GetUser(ctx context.Context, id int64) (iam.User, error) {
	if id <= 0 {
		return iam.User{}, shared.ErrInvalidInput
	}
	return s.repo.GetUser(ctx, id)
}

func (s *Service) CreateUser(ctx context.Context, in iam.CreateUserInput) (iam.User, error) {
	user := iam.User{
		UserName:   strings.TrimSpace(in.UserName),
		UserNameCN: strings.TrimSpace(in.UserNameCN),
		UserEmail:  strings.ToLower(strings.TrimSpace(in.UserEmail)),
		UserType:   in.UserType,
	}
	password := strings.TrimSpace(in.InitialPassword)
	if user.UserName == "" || user.UserNameCN == "" || user.UserEmail == "" || len(password) < minimumPasswordLength {
		return iam.User{}, shared.ErrInvalidInput
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return iam.User{}, err
	}
	return s.repo.CreateUser(ctx, user, hash)
}

func (s *Service) UpdateUser(ctx context.Context, id int64, in iam.UpdateUserInput) (iam.User, error) {
	if id <= 0 {
		return iam.User{}, shared.ErrInvalidInput
	}
	if in.UserEmail != nil {
		email := strings.ToLower(strings.TrimSpace(*in.UserEmail))
		in.UserEmail = &email
	}
	if in.UserName != nil {
		v := strings.TrimSpace(*in.UserName)
		in.UserName = &v
	}
	if in.UserNameCN != nil {
		v := strings.TrimSpace(*in.UserNameCN)
		in.UserNameCN = &v
	}
	return s.repo.UpdateUser(ctx, id, in)
}

func (s *Service) DeleteUser(ctx context.Context, id int64) error {
	if id <= 0 {
		return shared.ErrInvalidInput
	}
	return s.repo.DeleteUser(ctx, id)
}

func (s *Service) ResetUserPassword(ctx context.Context, id int64, password string) error {
	password = strings.TrimSpace(password)
	if id <= 0 || len(password) < minimumPasswordLength {
		return shared.ErrInvalidInput
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}
	return s.repo.ResetUserPassword(ctx, id, hash)
}

func (s *Service) CurrentPrincipal(ctx context.Context, claims security.Claims) (security.Principal, error) {
	user, err := s.repo.GetUser(ctx, claims.UserID)
	if err != nil {
		return security.Principal{}, shared.ErrUnauthorized
	}
	if user.AuthVersion != claims.AuthVersion {
		return security.Principal{}, shared.ErrUnauthorized
	}
	access, err := s.GetUserAccess(ctx, user.UserEmail)
	if err != nil {
		return security.Principal{}, shared.ErrUnauthorized
	}
	return security.Principal{UserID: user.ID, UserName: user.UserName, UserEmail: user.UserEmail, GlobalRoles: access.GlobalRoles}, nil
}

func (s *Service) ListRoles(ctx context.Context) ([]iam.Role, error) {
	return s.repo.ListRoles(ctx)
}

func (s *Service) GetRole(ctx context.Context, id int64) (iam.Role, error) {
	if id <= 0 {
		return iam.Role{}, shared.ErrInvalidInput
	}
	return s.repo.GetRole(ctx, id)
}

func (s *Service) CreateRole(ctx context.Context, in iam.CreateRoleInput) (iam.Role, error) {
	role := iam.Role{RoleName: strings.TrimSpace(in.RoleName), RoleDesc: strings.TrimSpace(in.RoleDesc)}
	if role.RoleName == "" {
		return iam.Role{}, shared.ErrInvalidInput
	}
	return s.repo.CreateRole(ctx, role, uniqueInt64s(in.PrivilegeIDs))
}

func (s *Service) UpdateRole(ctx context.Context, id int64, in iam.UpdateRoleInput) (iam.Role, error) {
	if id <= 0 {
		return iam.Role{}, shared.ErrInvalidInput
	}
	if in.RoleName != nil {
		v := strings.TrimSpace(*in.RoleName)
		in.RoleName = &v
	}
	if in.RoleDesc != nil {
		v := strings.TrimSpace(*in.RoleDesc)
		in.RoleDesc = &v
	}
	if in.PrivilegeIDs != nil {
		v := uniqueInt64s(*in.PrivilegeIDs)
		in.PrivilegeIDs = &v
	}
	return s.repo.UpdateRole(ctx, id, in)
}

func (s *Service) DeleteRole(ctx context.Context, id int64) error {
	role, err := s.GetRole(ctx, id)
	if err != nil {
		return err
	}
	if role.IsBuiltin {
		return shared.ErrInvalidInput
	}
	return s.repo.DeleteRole(ctx, id)
}

func (s *Service) ListRoleMembers(ctx context.Context, roleID int64) ([]iam.RoleMember, error) {
	if roleID <= 0 {
		return nil, shared.ErrInvalidInput
	}
	return s.repo.ListRoleMembers(ctx, roleID)
}

func (s *Service) ReplaceRoleMembers(ctx context.Context, roleID int64, in iam.UpdateRoleMembersInput) error {
	if roleID <= 0 {
		return shared.ErrInvalidInput
	}
	onDuty := map[string]bool{}
	for _, email := range in.OnDuty {
		email = strings.ToLower(strings.TrimSpace(email))
		if email != "" {
			onDuty[email] = true
		}
	}
	seen := map[string]bool{}
	members := []iam.RoleMember{}
	for _, email := range in.Emails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true
		members = append(members, iam.RoleMember{RoleID: roleID, MemberEmail: email, OnDuty: onDuty[email]})
	}
	return s.repo.ReplaceRoleMembers(ctx, roleID, members)
}

func (s *Service) ListRolePrivileges(ctx context.Context, roleID int64) ([]iam.Privilege, error) {
	if roleID <= 0 {
		return nil, shared.ErrInvalidInput
	}
	return s.repo.ListRolePrivileges(ctx, roleID)
}

func (s *Service) SetRolePrivileges(ctx context.Context, roleID int64, privilegeIDs []int64) error {
	if roleID <= 0 {
		return shared.ErrInvalidInput
	}
	return s.repo.SetRolePrivileges(ctx, roleID, uniqueInt64s(privilegeIDs))
}

func (s *Service) GetUserAccess(ctx context.Context, email string) (iam.UserAccess, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return iam.UserAccess{}, shared.ErrInvalidInput
	}
	roles, err := s.repo.ListUserRoles(ctx, email)
	if err != nil {
		return iam.UserAccess{}, err
	}
	permissions, err := s.repo.GetUserEffectivePermissions(ctx, email)
	if err != nil {
		return iam.UserAccess{}, err
	}
	return iam.UserAccess{
		Roles: roles, Permissions: permissions,
		GlobalRoles: roles,
	}, nil
}

func (s *Service) IsAdmin(ctx context.Context, userEmail string) bool {
	ok, err := s.repo.UserHasGlobalRole(ctx, userEmail, iam.GlobalRoleAdmin)
	return err == nil && ok
}

func (s *Service) ListProjectRoles(ctx context.Context) ([]iam.ProjectRole, error) {
	return s.repo.ListProjectRoles(ctx)
}

func (s *Service) CreateProjectRole(ctx context.Context, in iam.ProjectRoleInput) (iam.ProjectRole, error) {
	role := iam.ProjectRole{RoleName: strings.TrimSpace(in.RoleName), RoleDesc: strings.TrimSpace(in.RoleDesc)}
	if role.RoleName == "" || role.RoleDesc == "" {
		return iam.ProjectRole{}, shared.ErrInvalidInput
	}
	return s.repo.CreateProjectRole(ctx, role)
}

func (s *Service) UpdateProjectRole(ctx context.Context, id int64, in iam.ProjectRoleInput) (iam.ProjectRole, error) {
	if id <= 0 {
		return iam.ProjectRole{}, shared.ErrInvalidInput
	}
	return s.repo.UpdateProjectRole(ctx, id, in)
}

func (s *Service) DeleteProjectRole(ctx context.Context, id int64) error {
	if id <= 0 {
		return shared.ErrInvalidInput
	}
	role, err := s.repo.GetProjectRole(ctx, id)
	if err != nil {
		return err
	}
	if role.IsBuiltin {
		return shared.ErrInvalidInput
	}
	return s.repo.DeleteProjectRole(ctx, id)
}

func (s *Service) ListPrivileges(ctx context.Context) ([]iam.Privilege, error) {
	return s.repo.ListPrivileges(ctx)
}

func (s *Service) GetPrivilege(ctx context.Context, id int64) (iam.Privilege, error) {
	if id <= 0 {
		return iam.Privilege{}, shared.ErrInvalidInput
	}
	return s.repo.GetPrivilege(ctx, id)
}

func (s *Service) CreatePrivilege(ctx context.Context, in iam.CreatePrivilegeInput) (iam.Privilege, error) {
	item := iam.Privilege{
		PrivName:    strings.TrimSpace(in.PrivName),
		Description: strings.TrimSpace(in.Description),
		PrivType:    strings.TrimSpace(in.PrivType),
		Permission:  strings.TrimSpace(in.Permission),
	}
	if item.PrivName == "" || item.Description == "" || item.PrivType == "" || item.Permission == "" {
		return iam.Privilege{}, shared.ErrInvalidInput
	}
	return s.repo.CreatePrivilege(ctx, item)
}

func (s *Service) UpdatePrivilege(ctx context.Context, id int64, in iam.UpdatePrivilegeInput) (iam.Privilege, error) {
	if id <= 0 {
		return iam.Privilege{}, shared.ErrInvalidInput
	}
	current, err := s.repo.GetPrivilege(ctx, id)
	if err != nil {
		return iam.Privilege{}, err
	}
	if current.IsBuiltin {
		return iam.Privilege{}, shared.ErrInvalidInput
	}
	return s.repo.UpdatePrivilege(ctx, id, in)
}

func (s *Service) DeletePrivilege(ctx context.Context, id int64) error {
	current, err := s.GetPrivilege(ctx, id)
	if err != nil {
		return err
	}
	if current.IsBuiltin {
		return shared.ErrInvalidInput
	}
	refs, err := s.repo.CountPrivilegeRoleRefs(ctx, id)
	if err != nil {
		return err
	}
	if refs > 0 {
		return shared.ErrConflict
	}
	return s.repo.DeletePrivilege(ctx, id)
}

func uniqueInt64s(items []int64) []int64 {
	seen := map[int64]bool{}
	out := make([]int64, 0, len(items))
	for _, item := range items {
		if item <= 0 || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}
