package project

import "context"

type Repository interface {
	Create(ctx context.Context, item Project) (Project, error)
	CreateWithOwner(ctx context.Context, item Project, userID int64) (Project, error)
	List(ctx context.Context) ([]Project, error)
	ListForUser(ctx context.Context, userID int64) ([]Project, error)
	Get(ctx context.Context, id string) (Project, error)
	GetMemberRole(ctx context.Context, projectID string, userID int64) (string, error)
	ListMembers(ctx context.Context, projectID string) ([]Member, error)
	ListMemberCandidates(ctx context.Context, projectID string) ([]MemberCandidate, error)
	AddMember(ctx context.Context, projectID string, in AddMemberInput) (Member, error)
	UpdateMember(ctx context.Context, projectID string, userID int64, in UpdateMemberInput) (Member, error)
	DeleteMember(ctx context.Context, projectID string, userID int64) error
}
