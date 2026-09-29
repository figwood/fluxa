package gormdb

import (
	"context"
	"errors"
	"os"
	"strings"

	"fluxa-api/internal/domain/iam"
	"fluxa-api/internal/security"
	"fluxa-api/internal/shared"

	"gorm.io/gorm"
)

type IAMRepository struct {
	db         *gorm.DB
	production bool
}

func NewIAMRepository(db *gorm.DB) *IAMRepository { return &IAMRepository{db: db} }

func (r *IAMRepository) SetProduction(production bool) { r.production = production }

func (r *IAMRepository) EnsureBuiltinAccess(ctx context.Context) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		adminRole, err := ensureRole(ctx, tx, iam.GlobalRoleAdmin, "系统管理员")
		if err != nil {
			return err
		}
		normalRole, err := ensureRole(ctx, tx, iam.GlobalRoleNormal, "普通用户")
		if err != nil {
			return err
		}
		adminPriv, err := ensurePrivilege(ctx, tx, "global-admin", "全局管理员权限", "global:admin")
		if err != nil {
			return err
		}
		normalPriv, err := ensurePrivilege(ctx, tx, "global-normal", "普通用户权限", "global:normal")
		if err != nil {
			return err
		}
		for _, item := range []struct {
			name       string
			desc       string
			permission string
		}{
			{"project-owner", "项目负责人权限", "project:owner"},
			{"project-developer", "项目开发权限", "project:developer"},
			{"project-lead", "项目技术负责人权限", "project:lead"},
		} {
			if _, err := ensurePrivilege(ctx, tx, item.name, item.desc, item.permission); err != nil {
				return err
			}
		}
		for _, item := range []struct {
			name string
			desc string
		}{
			{iam.ProjectRoleOwner, "项目负责人"},
			{iam.ProjectRoleDeveloper, "开发人员"},
			{iam.ProjectRoleLead, "技术负责人"},
		} {
			if _, err := ensureProjectRole(ctx, tx, item.name, item.desc); err != nil {
				return err
			}
		}
		if err := ensureRolePrivilege(ctx, tx, adminRole.ID, adminPriv.ID); err != nil {
			return err
		}
		if err := ensureRolePrivilege(ctx, tx, normalRole.ID, normalPriv.ID); err != nil {
			return err
		}
		return ensureBootstrapAdmin(ctx, tx, adminRole.ID, r.production)
	})
}

func (r *IAMRepository) ListUsers(ctx context.Context, limit, offset int) ([]iam.User, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var rows []UserModel
	if err := r.db.WithContext(ctx).Order("id desc").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]iam.User, 0, len(rows))
	for _, row := range rows {
		out = append(out, toUser(row))
	}
	return out, nil
}

func (r *IAMRepository) GetUser(ctx context.Context, id int64) (iam.User, error) {
	var row UserModel
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return iam.User{}, shared.ErrNotFound
		}
		return iam.User{}, err
	}
	return toUser(row), nil
}

func (r *IAMRepository) FindUserForLogin(ctx context.Context, login string) (iam.User, string, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	var row UserModel
	err := r.db.WithContext(ctx).
		Where("LOWER(user_name) = ? OR LOWER(user_email) = ?", login, login).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return iam.User{}, "", shared.ErrNotFound
		}
		return iam.User{}, "", err
	}
	return toUser(row), row.UserPassword, nil
}

func (r *IAMRepository) CreateUser(ctx context.Context, item iam.User, passwordHash string) (iam.User, error) {
	row := UserModel{
		UserName: item.UserName, UserNameCN: item.UserNameCN, UserEmail: item.UserEmail,
		UserType: item.UserType, UserPassword: passwordHash,
	}
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		normalRole, err := ensureRole(ctx, tx, iam.GlobalRoleNormal, "普通用户")
		if err != nil {
			return err
		}
		return ensureRoleMember(ctx, tx, normalRole.ID, row.UserEmail)
	}); err != nil {
		return iam.User{}, err
	}
	return toUser(row), nil
}

func (r *IAMRepository) UpdateUser(ctx context.Context, id int64, in iam.UpdateUserInput) (iam.User, error) {
	updates := map[string]any{}
	if in.UserName != nil {
		updates["user_name"] = *in.UserName
	}
	if in.UserNameCN != nil {
		updates["user_name_cn"] = *in.UserNameCN
	}
	if in.UserEmail != nil {
		updates["user_email"] = *in.UserEmail
	}
	if in.UserType != nil {
		updates["user_type"] = *in.UserType
	}
	if len(updates) > 0 {
		var current UserModel
		if err := r.db.WithContext(ctx).First(&current, id).Error; err != nil {
			return iam.User{}, shared.ErrNotFound
		}
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&UserModel{}).Where("id = ?", id).Updates(updates)
			if res.Error != nil || res.RowsAffected == 0 {
				return res.Error
			}
			if in.UserEmail != nil && current.UserEmail != *in.UserEmail {
				return tx.Model(&RoleMemberModel{}).Where("member_email = ?", current.UserEmail).Update("member_email", *in.UserEmail).Error
			}
			return nil
		})
		if err != nil {
			return iam.User{}, err
		}
	}
	return r.GetUser(ctx, id)
}

func (r *IAMRepository) DeleteUser(ctx context.Context, id int64) error {
	user, err := r.GetUser(ctx, id)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("member_email = ?", user.UserEmail).Delete(&RoleMemberModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&ProjectMemberModel{}).Error; err != nil {
			return err
		}
		res := tx.Delete(&UserModel{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return shared.ErrNotFound
		}
		return nil
	})
}

func (r *IAMRepository) ResetUserPassword(ctx context.Context, id int64, passwordHash string) error {
	res := r.db.WithContext(ctx).Model(&UserModel{}).Where("id = ?", id).Updates(map[string]any{"user_password": passwordHash, "auth_version": gorm.Expr("auth_version + 1")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (r *IAMRepository) ListRoles(ctx context.Context) ([]iam.Role, error) {
	var rows []RoleModel
	if err := r.db.WithContext(ctx).Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]iam.Role, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRole(row))
	}
	return out, nil
}

func (r *IAMRepository) GetRole(ctx context.Context, id int64) (iam.Role, error) {
	var row RoleModel
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return iam.Role{}, shared.ErrNotFound
		}
		return iam.Role{}, err
	}
	return toRole(row), nil
}

func (r *IAMRepository) CreateRole(ctx context.Context, item iam.Role, privilegeIDs []int64) (iam.Role, error) {
	row := RoleModel{RoleName: item.RoleName, RoleDesc: item.RoleDesc, IsBuiltin: item.IsBuiltin}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return replaceRolePrivileges(tx, row.ID, privilegeIDs)
	})
	if err != nil {
		return iam.Role{}, err
	}
	return toRole(row), nil
}

func (r *IAMRepository) UpdateRole(ctx context.Context, id int64, in iam.UpdateRoleInput) (iam.Role, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{}
		if in.RoleName != nil {
			updates["role_name"] = *in.RoleName
		}
		if in.RoleDesc != nil {
			updates["role_desc"] = *in.RoleDesc
		}
		if len(updates) > 0 {
			res := tx.Model(&RoleModel{}).Where("id = ?", id).Updates(updates)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return shared.ErrNotFound
			}
		}
		if in.PrivilegeIDs != nil {
			if err := replaceRolePrivileges(tx, id, *in.PrivilegeIDs); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return iam.Role{}, err
	}
	return r.GetRole(ctx, id)
}

func (r *IAMRepository) DeleteRole(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", id).Delete(&RolePrivilegeModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("role_id = ?", id).Delete(&RoleMemberModel{}).Error; err != nil {
			return err
		}
		res := tx.Delete(&RoleModel{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return shared.ErrNotFound
		}
		return nil
	})
}

func (r *IAMRepository) ListRoleMembers(ctx context.Context, roleID int64) ([]iam.RoleMember, error) {
	var rows []RoleMemberModel
	if err := r.db.WithContext(ctx).Where("role_id = ?", roleID).Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]iam.RoleMember, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRoleMember(row))
	}
	return out, nil
}

func (r *IAMRepository) ReplaceRoleMembers(ctx context.Context, roleID int64, members []iam.RoleMember) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&RoleMemberModel{}).Error; err != nil {
			return err
		}
		for _, item := range members {
			row := RoleMemberModel{RoleID: roleID, MemberEmail: item.MemberEmail, OnDuty: item.OnDuty}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *IAMRepository) ListRolePrivileges(ctx context.Context, roleID int64) ([]iam.Privilege, error) {
	var rows []PrivilegeModel
	if err := r.db.WithContext(ctx).
		Table((&PrivilegeModel{}).TableName()+" p").
		Select("p.*").
		Joins("JOIN "+(&RolePrivilegeModel{}).TableName()+" rp ON rp.privilege_id = p.id").
		Where("rp.role_id = ?", roleID).
		Order("p.id asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]iam.Privilege, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPrivilege(row))
	}
	return out, nil
}

func (r *IAMRepository) SetRolePrivileges(ctx context.Context, roleID int64, privilegeIDs []int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return replaceRolePrivileges(tx, roleID, privilegeIDs)
	})
}

func (r *IAMRepository) ListUserRoles(ctx context.Context, userEmail string) ([]string, error) {
	var roles []string
	err := r.db.WithContext(ctx).
		Table((&RoleMemberModel{}).TableName()+" rm").
		Select("r.role_name").
		Joins("JOIN "+(&RoleModel{}).TableName()+" r ON r.id = rm.role_id").
		Where("rm.member_email = ?", userEmail).
		Order("r.id asc").
		Pluck("r.role_name", &roles).Error
	return roles, err
}

func (r *IAMRepository) GetUserEffectivePermissions(ctx context.Context, userEmail string) ([]string, error) {
	var permissions []string
	err := r.db.WithContext(ctx).
		Table((&PrivilegeModel{}).TableName()+" p").
		Distinct("p.permission").
		Joins("JOIN "+(&RolePrivilegeModel{}).TableName()+" rp ON rp.privilege_id = p.id").
		Joins("JOIN "+(&RoleMemberModel{}).TableName()+" rm ON rm.role_id = rp.role_id").
		Where("rm.member_email = ?", userEmail).
		Order("p.permission asc").
		Pluck("p.permission", &permissions).Error
	return permissions, err
}

func (r *IAMRepository) UserHasGlobalRole(ctx context.Context, userEmail, role string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table((&RoleMemberModel{}).TableName()+" rm").
		Joins("JOIN "+(&RoleModel{}).TableName()+" r ON r.id = rm.role_id").
		Where("rm.member_email = ? AND r.role_name = ?", strings.ToLower(strings.TrimSpace(userEmail)), strings.TrimSpace(role)).
		Count(&count).Error
	return count > 0, err
}

func (r *IAMRepository) ListPrivileges(ctx context.Context) ([]iam.Privilege, error) {
	var rows []PrivilegeModel
	if err := r.db.WithContext(ctx).Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]iam.Privilege, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPrivilege(row))
	}
	return out, nil
}

func (r *IAMRepository) GetPrivilege(ctx context.Context, id int64) (iam.Privilege, error) {
	var row PrivilegeModel
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return iam.Privilege{}, shared.ErrNotFound
		}
		return iam.Privilege{}, err
	}
	return toPrivilege(row), nil
}

func (r *IAMRepository) CreatePrivilege(ctx context.Context, item iam.Privilege) (iam.Privilege, error) {
	row := PrivilegeModel{
		PrivName: item.PrivName, Description: item.Description, PrivType: item.PrivType,
		Permission: item.Permission, IsBuiltin: item.IsBuiltin,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return iam.Privilege{}, err
	}
	return toPrivilege(row), nil
}

func (r *IAMRepository) UpdatePrivilege(ctx context.Context, id int64, in iam.UpdatePrivilegeInput) (iam.Privilege, error) {
	updates := map[string]any{}
	if in.Description != nil {
		updates["description"] = *in.Description
	}
	if in.PrivType != nil {
		updates["priv_type"] = *in.PrivType
	}
	if in.Permission != nil {
		updates["permission"] = *in.Permission
	}
	if len(updates) > 0 {
		res := r.db.WithContext(ctx).Model(&PrivilegeModel{}).Where("id = ?", id).Updates(updates)
		if res.Error != nil {
			return iam.Privilege{}, res.Error
		}
		if res.RowsAffected == 0 {
			return iam.Privilege{}, shared.ErrNotFound
		}
	}
	return r.GetPrivilege(ctx, id)
}

func (r *IAMRepository) DeletePrivilege(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Delete(&PrivilegeModel{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (r *IAMRepository) CountPrivilegeRoleRefs(ctx context.Context, id int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&RolePrivilegeModel{}).Where("privilege_id = ?", id).Count(&count).Error
	return count, err
}

func (r *IAMRepository) ListProjectRoles(ctx context.Context) ([]iam.ProjectRole, error) {
	var rows []ProjectRoleModel
	if err := r.db.WithContext(ctx).
		Order("id asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return toProjectRoles(rows), nil
}

func (r *IAMRepository) GetProjectRole(ctx context.Context, id int64) (iam.ProjectRole, error) {
	var row ProjectRoleModel
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return iam.ProjectRole{}, shared.ErrNotFound
		}
		return iam.ProjectRole{}, err
	}
	return toProjectRole(row), nil
}

func (r *IAMRepository) CreateProjectRole(ctx context.Context, item iam.ProjectRole) (iam.ProjectRole, error) {
	row := ProjectRoleModel{RoleName: normalizeProjectRole(item.RoleName), RoleDesc: item.RoleDesc, IsBuiltin: item.IsBuiltin}
	if row.RoleName == "" {
		return iam.ProjectRole{}, shared.ErrInvalidInput
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return iam.ProjectRole{}, err
	}
	return toProjectRole(row), nil
}

func (r *IAMRepository) UpdateProjectRole(ctx context.Context, id int64, in iam.ProjectRoleInput) (iam.ProjectRole, error) {
	updates := map[string]any{}
	if roleName := normalizeProjectRole(in.RoleName); roleName != "" {
		updates["role_name"] = roleName
	}
	if strings.TrimSpace(in.RoleDesc) != "" {
		updates["role_desc"] = strings.TrimSpace(in.RoleDesc)
	}
	if len(updates) > 0 {
		res := r.db.WithContext(ctx).Model(&ProjectRoleModel{}).Where("id = ?", id).Updates(updates)
		if res.Error != nil {
			return iam.ProjectRole{}, res.Error
		}
		if res.RowsAffected == 0 {
			return iam.ProjectRole{}, shared.ErrNotFound
		}
	}
	return r.GetProjectRole(ctx, id)
}

func (r *IAMRepository) DeleteProjectRole(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Delete(&ProjectRoleModel{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func replaceRolePrivileges(tx *gorm.DB, roleID int64, privilegeIDs []int64) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&RolePrivilegeModel{}).Error; err != nil {
		return err
	}
	for _, id := range privilegeIDs {
		row := RolePrivilegeModel{RoleID: roleID, PrivilegeID: id}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func toUser(row UserModel) iam.User {
	return iam.User{ID: row.ID, UserName: row.UserName, UserNameCN: row.UserNameCN, UserEmail: row.UserEmail, UserType: row.UserType, AuthVersion: row.AuthVersion}
}

func toRole(row RoleModel) iam.Role {
	return iam.Role{ID: row.ID, RoleName: row.RoleName, RoleDesc: row.RoleDesc, IsBuiltin: row.IsBuiltin}
}

func toRoleMember(row RoleMemberModel) iam.RoleMember {
	return iam.RoleMember{ID: row.ID, RoleID: row.RoleID, MemberEmail: row.MemberEmail, OnDuty: row.OnDuty}
}

func toPrivilege(row PrivilegeModel) iam.Privilege {
	return iam.Privilege{
		ID: row.ID, PrivName: row.PrivName, Description: row.Description,
		PrivType: row.PrivType, Permission: row.Permission, IsBuiltin: row.IsBuiltin,
	}
}

func toProjectRoles(rows []ProjectRoleModel) []iam.ProjectRole {
	out := make([]iam.ProjectRole, 0, len(rows))
	for _, row := range rows {
		out = append(out, toProjectRole(row))
	}
	return out
}

func toProjectRole(row ProjectRoleModel) iam.ProjectRole {
	return iam.ProjectRole{ID: row.ID, RoleName: row.RoleName, RoleDesc: row.RoleDesc, IsBuiltin: row.IsBuiltin}
}

func ensureRole(ctx context.Context, tx *gorm.DB, name, desc string) (RoleModel, error) {
	var row RoleModel
	err := tx.WithContext(ctx).Where("role_name = ?", name).First(&row).Error
	if err == nil {
		if !row.IsBuiltin {
			if err := tx.WithContext(ctx).Model(&RoleModel{}).Where("id = ?", row.ID).Update("is_builtin", true).Error; err != nil {
				return row, err
			}
			row.IsBuiltin = true
		}
		return row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return RoleModel{}, err
	}
	row = RoleModel{RoleName: name, RoleDesc: desc, IsBuiltin: true}
	return row, tx.WithContext(ctx).Create(&row).Error
}

func ensurePrivilege(ctx context.Context, tx *gorm.DB, name, desc, permission string) (PrivilegeModel, error) {
	var row PrivilegeModel
	err := tx.WithContext(ctx).Where("priv_name = ?", name).First(&row).Error
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return PrivilegeModel{}, err
	}
	row = PrivilegeModel{PrivName: name, Description: desc, PrivType: "builtin", Permission: permission, IsBuiltin: true}
	return row, tx.WithContext(ctx).Create(&row).Error
}

func ensureProjectRole(ctx context.Context, tx *gorm.DB, name, desc string) (ProjectRoleModel, error) {
	name = normalizeProjectRole(name)
	if name == "" {
		return ProjectRoleModel{}, shared.ErrInvalidInput
	}
	var row ProjectRoleModel
	err := tx.WithContext(ctx).Where("role_name = ?", name).First(&row).Error
	if err == nil {
		if !row.IsBuiltin {
			if err := tx.WithContext(ctx).Model(&ProjectRoleModel{}).Where("id = ?", row.ID).Update("is_builtin", true).Error; err != nil {
				return row, err
			}
			row.IsBuiltin = true
		}
		return row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return ProjectRoleModel{}, err
	}
	row = ProjectRoleModel{RoleName: name, RoleDesc: desc, IsBuiltin: true}
	return row, tx.WithContext(ctx).Create(&row).Error
}

func ensureRolePrivilege(ctx context.Context, tx *gorm.DB, roleID, privilegeID int64) error {
	var count int64
	if err := tx.WithContext(ctx).Model(&RolePrivilegeModel{}).
		Where("role_id = ? AND privilege_id = ?", roleID, privilegeID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return tx.WithContext(ctx).Create(&RolePrivilegeModel{RoleID: roleID, PrivilegeID: privilegeID}).Error
}

func ensureRoleMember(ctx context.Context, tx *gorm.DB, roleID int64, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	var count int64
	if err := tx.WithContext(ctx).Model(&RoleMemberModel{}).
		Where("role_id = ? AND member_email = ?", roleID, email).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return tx.WithContext(ctx).Create(&RoleMemberModel{RoleID: roleID, MemberEmail: email}).Error
}

func ensureBootstrapAdmin(ctx context.Context, tx *gorm.DB, adminRoleID int64, production bool) error {
	const email = "admin@local"
	var row UserModel
	err := tx.WithContext(ctx).Where("user_email = ?", email).First(&row).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		password := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"))
		if password == "" {
			if production {
				return errors.New("BOOTSTRAP_ADMIN_PASSWORD is required for the first production startup")
			}
			password = "development-only-password"
		}
		if len(password) < 8 {
			return errors.New("BOOTSTRAP_ADMIN_PASSWORD must contain at least 12 characters")
		}
		hash, err := security.HashPassword(password)
		if err != nil {
			return err
		}
		row = UserModel{
			UserName: "admin", UserNameCN: "管理员", UserEmail: email,
			UserType: 0, UserPassword: hash,
		}
		if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
	}
	return ensureRoleMember(ctx, tx, adminRoleID, email)
}

func normalizeProjectRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case iam.ProjectRoleOwner:
		return iam.ProjectRoleOwner
	case iam.ProjectRoleDeveloper, "dev":
		return iam.ProjectRoleDeveloper
	case iam.ProjectRoleLead, "tl":
		return iam.ProjectRoleLead
	default:
		return ""
	}
}
