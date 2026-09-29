package iam

const (
	GlobalRoleAdmin  = "admin"
	GlobalRoleNormal = "normal"

	ProjectRoleOwner     = "owner"
	ProjectRoleDeveloper = "developer"
	ProjectRoleLead      = "lead"
)

type User struct {
	ID          int64  `json:"id"`
	UserName    string `json:"user_name"`
	UserNameCN  string `json:"user_name_cn"`
	UserEmail   string `json:"user_email"`
	UserType    int    `json:"user_type"`
	AuthVersion int    `json:"-"`
}

type CreateUserInput struct {
	UserName        string `json:"user_name"`
	UserNameCN      string `json:"user_name_cn"`
	UserEmail       string `json:"user_email"`
	UserType        int    `json:"user_type"`
	InitialPassword string `json:"initial_password"`
}

type UpdateUserInput struct {
	UserName   *string `json:"user_name"`
	UserNameCN *string `json:"user_name_cn"`
	UserEmail  *string `json:"user_email"`
	UserType   *int    `json:"user_type"`
}

type Role struct {
	ID        int64  `json:"id"`
	RoleName  string `json:"role_name"`
	RoleDesc  string `json:"role_desc"`
	IsBuiltin bool   `json:"is_builtin"`
}

type CreateRoleInput struct {
	RoleName     string  `json:"role_name"`
	RoleDesc     string  `json:"role_desc"`
	PrivilegeIDs []int64 `json:"privilege_ids"`
}

type UpdateRoleInput struct {
	RoleName     *string  `json:"role_name"`
	RoleDesc     *string  `json:"role_desc"`
	PrivilegeIDs *[]int64 `json:"privilege_ids"`
}

type RoleMember struct {
	ID          int64  `json:"id"`
	RoleID      int64  `json:"role_id"`
	MemberEmail string `json:"member_email"`
	OnDuty      bool   `json:"on_duty"`
}

type UpdateRoleMembersInput struct {
	Emails []string `json:"emails"`
	OnDuty []string `json:"on_duty"`
}

type Privilege struct {
	ID          int64  `json:"id"`
	PrivName    string `json:"priv_name"`
	Description string `json:"description"`
	PrivType    string `json:"priv_type"`
	Permission  string `json:"permission"`
	IsBuiltin   bool   `json:"is_builtin"`
}

type CreatePrivilegeInput struct {
	PrivName    string `json:"priv_name"`
	Description string `json:"description"`
	PrivType    string `json:"priv_type"`
	Permission  string `json:"permission"`
}

type UpdatePrivilegeInput struct {
	Description *string `json:"description"`
	PrivType    *string `json:"priv_type"`
	Permission  *string `json:"permission"`
}

type UserAccess struct {
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	GlobalRoles []string `json:"global_roles"`
}

type LoginInput struct {
	UserName string `json:"user_name"`
	Password string `json:"password"`
}

type LoginResult struct {
	AccessToken string     `json:"access_token"`
	User        User       `json:"user"`
	Auth        UserAccess `json:"auth"`
}

type ProjectRole struct {
	ID        int64  `json:"id"`
	RoleName  string `json:"role_name"`
	RoleDesc  string `json:"role_desc"`
	IsBuiltin bool   `json:"is_builtin"`
}

type ProjectRoleInput struct {
	RoleName string `json:"role_name"`
	RoleDesc string `json:"role_desc"`
}
