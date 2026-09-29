package handlers

import (
	"fluxa-api/internal/application/releaseapp"
	"fluxa-api/internal/domain/release"

	"github.com/gin-gonic/gin"
)

func RegisterReleaseRoutes(r gin.IRouter, svc *releaseapp.Service) {
	r.GET("/releases", func(c *gin.Context) {
		items, err := svc.List(c.Request.Context(), c.Query("project_id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	})
	r.POST("/releases", func(c *gin.Context) {
		var in release.CreateReleaseInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, err)
			return
		}
		item, err := svc.Create(c.Request.Context(), in)
		if err != nil {
			Error(c, err)
			return
		}
		Created(c, item)
	})
	r.POST("/projects/:id/releases/from-tasks", func(c *gin.Context) {
		var in release.CreateReleaseInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, err)
			return
		}
		in.ProjectID = c.Param("id")
		item, err := svc.Create(c.Request.Context(), in)
		if err != nil {
			Error(c, err)
			return
		}
		Created(c, item)
	})
	r.GET("/releases/:id", func(c *gin.Context) {
		item, err := svc.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
	r.POST("/releases/:id/submit", func(c *gin.Context) {
		item, err := svc.Submit(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
	r.POST("/releases/:id/approve", func(c *gin.Context) {
		item, err := svc.Approve(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
	r.POST("/releases/:id/publish", func(c *gin.Context) {
		item, err := svc.Publish(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
	r.POST("/releases/:id/retry", func(c *gin.Context) {
		item, err := svc.Retry(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		Created(c, item)
	})
	r.PATCH("/releases/:id/status", func(c *gin.Context) {
		var in struct {
			Status release.ReleaseStatus `json:"status"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, err)
			return
		}
		item, execution, err := svc.SetStatus(c.Request.Context(), c.Param("id"), in.Status)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, gin.H{"release": item, "execution": execution})
	})
	r.GET("/release-jobs/:id", func(c *gin.Context) {
		item, err := svc.GetJob(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
}
