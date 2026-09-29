package handlers

import (
	"fluxa-api/internal/application/taskapp"
	"fluxa-api/internal/domain/task"

	"github.com/gin-gonic/gin"
)

func RegisterTaskRoutes(r gin.IRouter, svc *taskapp.Service) {
	r.GET("/tasks", func(c *gin.Context) {
		items, err := svc.List(c.Request.Context(), c.Query("project_id"), task.Status(c.Query("status")))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	})
	r.GET("/tasks/:id", func(c *gin.Context) {
		item, err := svc.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
	r.POST("/tasks", func(c *gin.Context) {
		var in task.CreateTaskInput
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
	r.PATCH("/tasks/:id/status", func(c *gin.Context) {
		var in struct {
			Status task.Status `json:"status"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, err)
			return
		}
		item, err := svc.Transition(c.Request.Context(), c.Param("id"), in.Status)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
	r.PATCH("/tasks/:id/assignee", func(c *gin.Context) {
		var in struct {
			Assignee string `json:"assignee"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, err)
			return
		}
		item, err := svc.UpdateAssignee(c.Request.Context(), c.Param("id"), in.Assignee)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
}
