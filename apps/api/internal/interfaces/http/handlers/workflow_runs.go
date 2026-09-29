package handlers

import (
	"fluxa-api/internal/application/workflowrunapp"
	"fluxa-api/internal/domain/workflowrun"

	"github.com/gin-gonic/gin"
)

func RegisterWorkflowRunRoutes(r gin.IRouter, svc *workflowrunapp.Service) {
	r.POST("/releases/:id/executions", func(c *gin.Context) {
		var in workflowrun.StartInput
		if c.Request.ContentLength > 0 {
			if err := c.ShouldBindJSON(&in); err != nil {
				Error(c, err)
				return
			}
		}
		item, err := svc.Start(c.Request.Context(), c.Param("id"), in)
		if err != nil {
			Error(c, err)
			return
		}
		Created(c, item)
	})
	r.GET("/releases/:id/execution", func(c *gin.Context) {
		item, err := svc.GetByRelease(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
	r.GET("/executions/:id", func(c *gin.Context) {
		item, err := svc.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
	r.POST("/executions/:id/approvals/:taskId", func(c *gin.Context) {
		var in struct {
			Decision string `json:"decision"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, err)
			return
		}
		item, err := svc.Approve(c.Request.Context(), c.Param("id"), c.Param("taskId"), in.Decision)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
	r.POST("/executions/:id/retry", func(c *gin.Context) {
		item, err := svc.Retry(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
	r.POST("/executions/:id/cancel", func(c *gin.Context) {
		item, err := svc.Cancel(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
}
