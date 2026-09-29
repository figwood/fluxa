package handlers

import (
	"fluxa-api/internal/application/workflowapp"
	"fluxa-api/internal/domain/workflow"

	"github.com/gin-gonic/gin"
)

func RegisterWorkflowRoutes(r gin.IRouter, svc *workflowapp.Service) {
	r.GET("/projects/:id/workflows", func(c *gin.Context) {
		items, err := svc.List(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	})
	r.GET("/projects/:id/workflows/:kind", func(c *gin.Context) {
		item, err := svc.Get(c.Request.Context(), c.Param("id"), workflow.Kind(c.Param("kind")))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
	r.GET("/projects/:id/workflows/:kind/transitions", func(c *gin.Context) {
		items, err := svc.AllowedTransitions(c.Request.Context(), c.Param("id"), workflow.Kind(c.Param("kind")), c.Query("from_status"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	})
	r.PUT("/projects/:id/workflows/:kind", func(c *gin.Context) {
		var in workflow.UpdateWorkflowInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, err)
			return
		}
		item, err := svc.Update(c.Request.Context(), c.Param("id"), workflow.Kind(c.Param("kind")), in)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
}

func RegisterWorkflowAdminRoutes(r gin.IRouter, svc *workflowapp.Service) {
	r.GET("/workflows", func(c *gin.Context) {
		items, err := svc.List(c.Request.Context(), "")
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	})
}
