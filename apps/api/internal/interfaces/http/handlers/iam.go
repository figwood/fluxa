package handlers

import (
	"net/http"
	"strconv"

	"fluxa-api/internal/application/iamapp"
	"fluxa-api/internal/domain/iam"
	"fluxa-api/internal/shared"

	"github.com/gin-gonic/gin"
)

type passwordRequest struct {
	Password string `json:"password"`
}

type rolePrivilegesRequest struct {
	PrivilegeIDs []int64 `json:"privilege_ids"`
}

func RegisterIAMRoutes(r gin.IRouter, svc *iamapp.Service) {
	registerUserRoutes(r, svc)
	registerRoleRoutes(r, svc)
	registerPrivilegeRoutes(r, svc)
	registerProjectRoleRoutes(r, svc)
}

func RegisterAuthPublicRoutes(r gin.IRouter, svc *iamapp.Service) {
	r.POST("/auth/login", login(svc))
}

func RegisterAuthRoutes(r gin.IRouter, svc *iamapp.Service) {
	r.GET("/auth/me", me(svc))
	r.POST("/auth/logout", func(c *gin.Context) { OK(c, nil) })
}

func login(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in iam.LoginInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, shared.ErrInvalidInput)
			return
		}
		result, err := svc.Login(c.Request.Context(), in)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, result)
	}
}

func me(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailValue, _ := c.Get("user_email")
		email, _ := emailValue.(string)
		access, err := svc.GetUserAccess(c.Request.Context(), email)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, access)
	}
}

func registerUserRoutes(r gin.IRouter, svc *iamapp.Service) {
	r.GET("/users", listUsers(svc))
	r.POST("/users", createUser(svc))
	r.GET("/users/:id", getUser(svc))
	r.PUT("/users/:id", updateUser(svc))
	r.DELETE("/users/:id", deleteUser(svc))
	r.PUT("/users/:id/password", resetUserPassword(svc))

	grp := r.Group("/user")
	grp.GET("/list", listUsers(svc))
	grp.POST("", createUser(svc))
	grp.GET("/:id", getUser(svc))
	grp.PUT("/:id", updateUser(svc))
	grp.DELETE("/:id", deleteUser(svc))
	grp.PUT("/:id/password", resetUserPassword(svc))
}

func registerRoleRoutes(r gin.IRouter, svc *iamapp.Service) {
	r.GET("/roles", listRoles(svc))
	r.POST("/roles", createRole(svc))
	r.GET("/roles/:id", getRole(svc))
	r.PUT("/roles/:id", updateRole(svc))
	r.DELETE("/roles/:id", deleteRole(svc))
	r.GET("/roles/:id/members", listRoleMembers(svc))
	r.PUT("/roles/:id/members", replaceRoleMembers(svc))
	r.GET("/roles/:id/privileges", listRolePrivileges(svc))
	r.PUT("/roles/:id/privileges", setRolePrivileges(svc))
	r.GET("/roles/user/:email", getUserAccess(svc))

	grp := r.Group("/role")
	grp.GET("/list", listRoles(svc))
	grp.POST("", createRole(svc))
	grp.GET("/user/:email", getUserAccess(svc))
	grp.GET("/:id", getRole(svc))
	grp.PUT("/:id", updateRole(svc))
	grp.DELETE("/:id", deleteRole(svc))
	grp.GET("/:id/member/list", listRoleMembers(svc))
	grp.PUT("/:id/members", replaceRoleMembers(svc))
	grp.GET("/:id/privileges", listRolePrivileges(svc))
	grp.PUT("/:id/privileges", setRolePrivileges(svc))
}

func registerPrivilegeRoutes(r gin.IRouter, svc *iamapp.Service) {
	r.GET("/privileges", listPrivileges(svc))
	r.POST("/privileges", createPrivilege(svc))
	r.GET("/privileges/:id", getPrivilege(svc))
	r.PUT("/privileges/:id", updatePrivilege(svc))
	r.DELETE("/privileges/:id", deletePrivilege(svc))

	grp := r.Group("/privilege")
	grp.GET("/list", listPrivileges(svc))
	grp.POST("", createPrivilege(svc))
	grp.GET("/:id", getPrivilege(svc))
	grp.PUT("/:id", updatePrivilege(svc))
	grp.DELETE("/:id", deletePrivilege(svc))
}

func registerProjectRoleRoutes(r gin.IRouter, svc *iamapp.Service) {
	r.GET("/project-roles", listProjectRoles(svc))
}

func listUsers(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, offset := pagination(c)
		items, err := svc.ListUsers(c.Request.Context(), limit, offset)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	}
}

func getUser(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := intID(c)
		if !ok {
			return
		}
		item, err := svc.GetUser(c.Request.Context(), id)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	}
}

func createUser(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in iam.CreateUserInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, shared.ErrInvalidInput)
			return
		}
		item, err := svc.CreateUser(c.Request.Context(), in)
		if err != nil {
			Error(c, err)
			return
		}
		Created(c, item)
	}
}

func updateUser(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := intID(c)
		if !ok {
			return
		}
		var in iam.UpdateUserInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, shared.ErrInvalidInput)
			return
		}
		item, err := svc.UpdateUser(c.Request.Context(), id, in)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	}
}

func deleteUser(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := intID(c)
		if !ok {
			return
		}
		if err := svc.DeleteUser(c.Request.Context(), id); err != nil {
			Error(c, err)
			return
		}
		OK(c, nil)
	}
}

func resetUserPassword(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := intID(c)
		if !ok {
			return
		}
		var in passwordRequest
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, shared.ErrInvalidInput)
			return
		}
		if err := svc.ResetUserPassword(c.Request.Context(), id, in.Password); err != nil {
			Error(c, err)
			return
		}
		OK(c, nil)
	}
}

func listRoles(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := svc.ListRoles(c.Request.Context())
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	}
}

func getRole(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := intID(c)
		if !ok {
			return
		}
		item, err := svc.GetRole(c.Request.Context(), id)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	}
}

func createRole(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in iam.CreateRoleInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, shared.ErrInvalidInput)
			return
		}
		item, err := svc.CreateRole(c.Request.Context(), in)
		if err != nil {
			Error(c, err)
			return
		}
		Created(c, item)
	}
}

func updateRole(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := intID(c)
		if !ok {
			return
		}
		var in iam.UpdateRoleInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, shared.ErrInvalidInput)
			return
		}
		item, err := svc.UpdateRole(c.Request.Context(), id, in)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	}
}

func deleteRole(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := intID(c)
		if !ok {
			return
		}
		if err := svc.DeleteRole(c.Request.Context(), id); err != nil {
			Error(c, err)
			return
		}
		OK(c, nil)
	}
}

func listRoleMembers(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := intID(c)
		if !ok {
			return
		}
		items, err := svc.ListRoleMembers(c.Request.Context(), id)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	}
}

func replaceRoleMembers(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := intID(c)
		if !ok {
			return
		}
		var in iam.UpdateRoleMembersInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, shared.ErrInvalidInput)
			return
		}
		if err := svc.ReplaceRoleMembers(c.Request.Context(), id, in); err != nil {
			Error(c, err)
			return
		}
		items, err := svc.ListRoleMembers(c.Request.Context(), id)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	}
}

func listRolePrivileges(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := intID(c)
		if !ok {
			return
		}
		items, err := svc.ListRolePrivileges(c.Request.Context(), id)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	}
}

func setRolePrivileges(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := intID(c)
		if !ok {
			return
		}
		var in rolePrivilegesRequest
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, shared.ErrInvalidInput)
			return
		}
		if err := svc.SetRolePrivileges(c.Request.Context(), id, in.PrivilegeIDs); err != nil {
			Error(c, err)
			return
		}
		items, err := svc.ListRolePrivileges(c.Request.Context(), id)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	}
}

func getUserAccess(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		item, err := svc.GetUserAccess(c.Request.Context(), c.Param("email"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	}
}

func listProjectRoles(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := svc.ListProjectRoles(c.Request.Context())
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	}
}

func createProjectRole(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in iam.ProjectRoleInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, shared.ErrInvalidInput)
			return
		}
		item, err := svc.CreateProjectRole(c.Request.Context(), in)
		if err != nil {
			Error(c, err)
			return
		}
		Created(c, item)
	}
}

func updateProjectRole(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := intID(c)
		if !ok {
			return
		}
		var in iam.ProjectRoleInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, shared.ErrInvalidInput)
			return
		}
		item, err := svc.UpdateProjectRole(c.Request.Context(), id, in)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	}
}

func deleteProjectRole(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := intID(c)
		if !ok {
			return
		}
		if err := svc.DeleteProjectRole(c.Request.Context(), id); err != nil {
			Error(c, err)
			return
		}
		OK(c, nil)
	}
}

func listPrivileges(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := svc.ListPrivileges(c.Request.Context())
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	}
}

func getPrivilege(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := intID(c)
		if !ok {
			return
		}
		item, err := svc.GetPrivilege(c.Request.Context(), id)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	}
}

func createPrivilege(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in iam.CreatePrivilegeInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, shared.ErrInvalidInput)
			return
		}
		item, err := svc.CreatePrivilege(c.Request.Context(), in)
		if err != nil {
			Error(c, err)
			return
		}
		Created(c, item)
	}
}

func updatePrivilege(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := intID(c)
		if !ok {
			return
		}
		var in iam.UpdatePrivilegeInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, shared.ErrInvalidInput)
			return
		}
		item, err := svc.UpdatePrivilege(c.Request.Context(), id, in)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	}
}

func deletePrivilege(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := intID(c)
		if !ok {
			return
		}
		if err := svc.DeletePrivilege(c.Request.Context(), id); err != nil {
			Error(c, err)
			return
		}
		OK(c, nil)
	}
}

func intID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: shared.ErrInvalidInput.Error()})
		return 0, false
	}
	return id, true
}

func pagination(c *gin.Context) (int, int) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
