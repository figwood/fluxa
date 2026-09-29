package project

import "time"

type Project struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Key             string    `json:"key"`
	Description     string    `json:"description"`
	CurrentUserRole string    `json:"current_user_role,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type CreateProjectInput struct {
	Name        string `json:"name"`
	Key         string `json:"key"`
	Description string `json:"description"`
}

type Member struct {
	ID        int64  `json:"id"`
	ProjectID string `json:"project_id"`
	UserID    int64  `json:"user_id"`
	UserName  string `json:"user_name"`
	UserEmail string `json:"user_email"`
	RoleID    int64  `json:"role_id"`
	RoleName  string `json:"role_name"`
}

type MemberList struct {
	Items     []Member `json:"items"`
	CanManage bool     `json:"can_manage"`
}

type MemberCandidate struct {
	UserID    int64  `json:"user_id"`
	UserName  string `json:"user_name"`
	UserEmail string `json:"user_email"`
}

type AddMemberInput struct {
	UserID   int64  `json:"user_id"`
	RoleName string `json:"role_name"`
}

type UpdateMemberInput struct {
	RoleName string `json:"role_name"`
}
