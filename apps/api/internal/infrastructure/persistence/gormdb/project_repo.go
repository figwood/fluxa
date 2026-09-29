package gormdb

import (
	"context"
	"errors"

	"fluxa-api/internal/domain/project"
	"fluxa-api/internal/shared"

	"gorm.io/gorm"
)

type ProjectRepository struct{ db *gorm.DB }

func NewProjectRepository(db *gorm.DB) *ProjectRepository { return &ProjectRepository{db: db} }

func (r *ProjectRepository) Create(ctx context.Context, item project.Project) (project.Project, error) {
	model := ProjectModel{
		ID: item.ID, Name: item.Name, Key: item.Key, Description: item.Description,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return project.Project{}, err
	}
	return toProject(model), nil
}

func (r *ProjectRepository) CreateWithOwner(ctx context.Context, item project.Project, userID int64) (project.Project, error) {
	var created project.Project
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user UserModel
		if err := tx.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return shared.ErrNotFound
			}
			return err
		}
		repo := NewProjectRepository(tx)
		var err error
		created, err = repo.Create(ctx, item)
		if err != nil {
			return err
		}
		role, err := findBuiltinProjectRole(ctx, tx, "owner")
		if err != nil {
			return err
		}
		return tx.WithContext(ctx).Create(&ProjectMemberModel{ProjectID: item.ID, UserID: userID, ProjectRoleID: role.ID}).Error
	})
	return created, err
}

func (r *ProjectRepository) List(ctx context.Context) ([]project.Project, error) {
	var rows []ProjectModel
	if err := r.db.WithContext(ctx).Order("created_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]project.Project, 0, len(rows))
	for _, row := range rows {
		out = append(out, toProject(row))
	}
	return out, nil
}

func (r *ProjectRepository) ListForUser(ctx context.Context, userID int64) ([]project.Project, error) {
	var rows []ProjectModel
	err := r.db.WithContext(ctx).
		Joins("JOIN project_members pm ON pm.project_id = projects.id").
		Where("pm.user_id = ?", userID).Order("projects.created_at desc").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]project.Project, 0, len(rows))
	for _, row := range rows {
		out = append(out, toProject(row))
	}
	return out, nil
}

func (r *ProjectRepository) Get(ctx context.Context, id string) (project.Project, error) {
	var row ProjectModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return project.Project{}, shared.ErrNotFound
		}
		return project.Project{}, err
	}
	return toProject(row), nil
}

func (r *ProjectRepository) GetMemberRole(ctx context.Context, projectID string, userID int64) (string, error) {
	var role ProjectRoleModel
	err := r.db.WithContext(ctx).Table("project_role_defs pr").
		Select("pr.*").Joins("JOIN project_members pm ON pm.project_role_id = pr.id").
		Where("pm.project_id = ? AND pm.user_id = ?", projectID, userID).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", shared.ErrForbidden
	}
	return role.RoleName, err
}

func (r *ProjectRepository) ListMembers(ctx context.Context, projectID string) ([]project.Member, error) {
	var rows []struct {
		ID, UserID, RoleID  int64
		ProjectID, UserName string
		UserEmail, RoleName string
	}
	err := r.db.WithContext(ctx).Table("project_members pm").
		Select("pm.id, pm.project_id, pm.user_id, u.user_name, u.user_email, pr.id AS role_id, pr.role_name").
		Joins("JOIN users u ON u.id = pm.user_id").
		Joins("JOIN project_role_defs pr ON pr.id = pm.project_role_id").
		Where("pm.project_id = ?", projectID).Order("pr.role_name, u.user_name").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]project.Member, 0, len(rows))
	for _, row := range rows {
		out = append(out, project.Member{ID: row.ID, ProjectID: row.ProjectID, UserID: row.UserID, UserName: row.UserName, UserEmail: row.UserEmail, RoleID: row.RoleID, RoleName: row.RoleName})
	}
	return out, nil
}

func (r *ProjectRepository) ListMemberCandidates(ctx context.Context, projectID string) ([]project.MemberCandidate, error) {
	var users []UserModel
	err := r.db.WithContext(ctx).Where("id NOT IN (?)", r.db.Table("project_members").Select("user_id").Where("project_id = ?", projectID)).
		Order("user_name").Find(&users).Error
	if err != nil {
		return nil, err
	}
	out := make([]project.MemberCandidate, 0, len(users))
	for _, user := range users {
		out = append(out, project.MemberCandidate{UserID: user.ID, UserName: user.UserName, UserEmail: user.UserEmail})
	}
	return out, nil
}

func (r *ProjectRepository) AddMember(ctx context.Context, projectID string, in project.AddMemberInput) (project.Member, error) {
	role, err := findBuiltinProjectRole(ctx, r.db, in.RoleName)
	if err != nil {
		return project.Member{}, err
	}
	var user UserModel
	if err := r.db.WithContext(ctx).Where("id = ?", in.UserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return project.Member{}, shared.ErrNotFound
		}
		return project.Member{}, err
	}
	row := ProjectMemberModel{ProjectID: projectID, UserID: in.UserID, ProjectRoleID: role.ID}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return project.Member{}, shared.ErrConflict
	}
	return project.Member{ID: row.ID, ProjectID: projectID, UserID: user.ID, UserName: user.UserName, UserEmail: user.UserEmail, RoleID: role.ID, RoleName: role.RoleName}, nil
}

func (r *ProjectRepository) UpdateMember(ctx context.Context, projectID string, userID int64, in project.UpdateMemberInput) (project.Member, error) {
	role, err := findBuiltinProjectRole(ctx, r.db, in.RoleName)
	if err != nil {
		return project.Member{}, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureOwnerRemains(ctx, tx, projectID, userID, in.RoleName); err != nil {
			return err
		}
		res := tx.WithContext(ctx).Model(&ProjectMemberModel{}).Where("project_id = ? AND user_id = ?", projectID, userID).Update("project_role_id", role.ID)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return shared.ErrNotFound
		}
		return nil
	})
	if err != nil {
		return project.Member{}, err
	}
	members, err := r.ListMembers(ctx, projectID)
	for _, member := range members {
		if member.UserID == userID {
			return member, nil
		}
	}
	return project.Member{}, err
}

func (r *ProjectRepository) DeleteMember(ctx context.Context, projectID string, userID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureOwnerRemains(ctx, tx, projectID, userID, ""); err != nil {
			return err
		}
		res := tx.WithContext(ctx).Where("project_id = ? AND user_id = ?", projectID, userID).Delete(&ProjectMemberModel{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return shared.ErrNotFound
		}
		return nil
	})
}

func findBuiltinProjectRole(ctx context.Context, db *gorm.DB, name string) (ProjectRoleModel, error) {
	if name != "owner" && name != "lead" && name != "developer" {
		return ProjectRoleModel{}, shared.ErrInvalidInput
	}
	var role ProjectRoleModel
	if err := db.WithContext(ctx).Where("role_name = ? AND is_builtin = ?", name, true).First(&role).Error; err != nil {
		return ProjectRoleModel{}, err
	}
	return role, nil
}

func ensureOwnerRemains(ctx context.Context, db *gorm.DB, projectID string, userID int64, nextRole string) error {
	var current ProjectRoleModel
	err := db.WithContext(ctx).Table("project_role_defs pr").Select("pr.*").
		Joins("JOIN project_members pm ON pm.project_role_id = pr.id").
		Where("pm.project_id = ? AND pm.user_id = ?", projectID, userID).First(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return shared.ErrNotFound
	}
	if err != nil || current.RoleName != "owner" || nextRole == "owner" {
		return err
	}
	var count int64
	if err := db.WithContext(ctx).Table("project_members pm").Joins("JOIN project_role_defs pr ON pr.id = pm.project_role_id").
		Where("pm.project_id = ? AND pr.role_name = ?", projectID, "owner").Count(&count).Error; err != nil {
		return err
	}
	if count <= 1 {
		return shared.ErrConflict
	}
	return nil
}

func toProject(row ProjectModel) project.Project {
	return project.Project{
		ID: row.ID, Name: row.Name, Key: row.Key, Description: row.Description,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
