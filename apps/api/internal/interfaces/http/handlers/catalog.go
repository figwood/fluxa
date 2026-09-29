package handlers

import (
	"strconv"

	"fluxa-api/internal/application/projectapp"
	"fluxa-api/internal/domain/servicecatalog"
	"fluxa-api/internal/shared"

	"github.com/gin-gonic/gin"
)

func RegisterCatalogRoutes(r gin.IRouter, svc *projectapp.CatalogService) {
	r.GET("/gitlab/projects", func(c *gin.Context) {
		items, err := svc.SearchGitlabProjects(c.Request.Context(), c.Query("keyword"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	})
	r.GET("/gitlab-repositories", func(c *gin.Context) {
		items, err := svc.ListGitlabRepositories(c.Request.Context())
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	})
	r.POST("/gitlab-repositories", func(c *gin.Context) {
		var in servicecatalog.CreateGitlabRepositoryInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, err)
			return
		}
		item, err := svc.CreateGitlabRepository(c.Request.Context(), in)
		if err != nil {
			Error(c, err)
			return
		}
		Created(c, item)
	})
	r.GET("/projects/:id/gitlab-repositories", func(c *gin.Context) {
		items, err := svc.ListProjectGitlabRepositories(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	})
	r.POST("/projects/:id/gitlab-repositories", func(c *gin.Context) {
		var in servicecatalog.AttachGitlabRepositoryInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, err)
			return
		}
		item, err := svc.AttachProjectGitlabRepository(c.Request.Context(), c.Param("id"), in)
		if err != nil {
			Error(c, err)
			return
		}
		Created(c, item)
	})
	r.DELETE("/projects/:id/gitlab-repositories/:gitlab_project_id", func(c *gin.Context) {
		gitlabProjectID, err := strconv.ParseInt(c.Param("gitlab_project_id"), 10, 64)
		if err != nil {
			Error(c, shared.ErrInvalidInput)
			return
		}
		if err := svc.DetachProjectGitlabRepository(c.Request.Context(), c.Param("id"), gitlabProjectID); err != nil {
			Error(c, err)
			return
		}
		OK(c, gin.H{"deleted": true})
	})
	r.GET("/project-services", func(c *gin.Context) {
		items, err := svc.ListProjectServices(c.Request.Context(), c.Query("project_id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	})
	r.POST("/project-services", func(c *gin.Context) {
		var in servicecatalog.CreateProjectServiceInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, err)
			return
		}
		item, err := svc.CreateProjectService(c.Request.Context(), in)
		if err != nil {
			Error(c, err)
			return
		}
		Created(c, item)
	})
}
