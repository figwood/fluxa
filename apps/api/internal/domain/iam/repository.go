package iam

import "context"

type Repository interface {
	EnsureBuiltinAccess(ctx context.Context) error
	ListUsers(ctx context.Context, limit, offset int) ([]User, error)
	GetUser(ctx context.Context, id int64) (User, error)
	FindUserForLogin(ctx context.Context, login string) (User, string, error)
	CreateUser(ctx context.Context, item User, passwordHash string) (User, error)
	UpdateUser(ctx context.Context, id int64, in UpdateUserInput) (User, error)
	DeleteUser(ctx context.Context, id int64) error
	ResetUserPassword(ctx context.Context, id int64, passwordHash string) error

	ListRoles(ctx context.Context) ([]Role, error)
	GetRole(ctx context.Context, id int64) (Role, error)
	CreateRole(ctx context.Context, item Role, privilegeIDs []int64) (Role, error)
	UpdateRole(ctx context.Context, id int64, in UpdateRoleInput) (Role, error)
	DeleteRole(ctx context.Context, id int64) error
	ListRoleMembers(ctx context.Context, roleID int64) ([]RoleMember, error)
	ReplaceRoleMembers(ctx context.Context, roleID int64, members []RoleMember) error
	ListRolePrivileges(ctx context.Context, roleID int64) ([]Privilege, error)
	SetRolePrivileges(ctx context.Context, roleID int64, privilegeIDs []int64) error
	ListUserRoles(ctx context.Context, userEmail string) ([]string, error)
	GetUserEffectivePermissions(ctx context.Context, userEmail string) ([]string, error)
	UserHasGlobalRole(ctx context.Context, userEmail, role string) (bool, error)

	ListPrivileges(ctx context.Context) ([]Privilege, error)
	GetPrivilege(ctx context.Context, id int64) (Privilege, error)
	CreatePrivilege(ctx context.Context, item Privilege) (Privilege, error)
	UpdatePrivilege(ctx context.Context, id int64, in UpdatePrivilegeInput) (Privilege, error)
	DeletePrivilege(ctx context.Context, id int64) error
	CountPrivilegeRoleRefs(ctx context.Context, id int64) (int64, error)

	ListProjectRoles(ctx context.Context) ([]ProjectRole, error)
	GetProjectRole(ctx context.Context, id int64) (ProjectRole, error)
	CreateProjectRole(ctx context.Context, item ProjectRole) (ProjectRole, error)
	UpdateProjectRole(ctx context.Context, id int64, in ProjectRoleInput) (ProjectRole, error)
	DeleteProjectRole(ctx context.Context, id int64) error
}
