package handlers

import (
	"strconv"

	"fluxa-api/internal/application/projectapp"
	"fluxa-api/internal/domain/project"
	"fluxa-api/internal/shared"

	"github.com/gin-gonic/gin"
)

func RegisterProjectRoutes(r gin.IRouter, svc *projectapp.Service) {
	r.GET("/projects", func(c *gin.Context) {
		items, err := svc.List(c.Request.Context())
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	})
	r.GET("/projects/:id", func(c *gin.Context) {
		item, err := svc.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
	r.POST("/projects", func(c *gin.Context) {
		var in project.CreateProjectInput
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
	r.GET("/projects/:id/members", func(c *gin.Context) {
		items, err := svc.ListMembers(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	})
	r.GET("/projects/:id/member-candidates", func(c *gin.Context) {
		items, err := svc.ListMemberCandidates(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	})
	r.POST("/projects/:id/members", func(c *gin.Context) {
		var in project.AddMemberInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, err)
			return
		}
		item, err := svc.AddMember(c.Request.Context(), c.Param("id"), in)
		if err != nil {
			Error(c, err)
			return
		}
		Created(c, item)
	})
	r.PUT("/projects/:id/members/:user_id", func(c *gin.Context) {
		userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
		if err != nil {
			Error(c, shared.ErrInvalidInput)
			return
		}
		var in project.UpdateMemberInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, err)
			return
		}
		item, err := svc.UpdateMember(c.Request.Context(), c.Param("id"), userID, in)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
	r.DELETE("/projects/:id/members/:user_id", func(c *gin.Context) {
		userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
		if err != nil {
			Error(c, shared.ErrInvalidInput)
			return
		}
		if err := svc.DeleteMember(c.Request.Context(), c.Param("id"), userID); err != nil {
			Error(c, err)
			return
		}
		OK(c, gin.H{"deleted": true})
	})
}
