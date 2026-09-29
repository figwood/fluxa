package handlers

import (
	"fluxa-api/internal/application/workerapp"

	"github.com/gin-gonic/gin"
)

func RegisterWorkerRoutes(r gin.IRouter, svc *workerapp.ControlService) {
	r.GET("/workers", func(c *gin.Context) {
		item, err := svc.Snapshot(c.Request.Context())
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
	r.PATCH("/workers/config", func(c *gin.Context) {
		var in struct {
			DesiredReplicas int `json:"desired_replicas"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, err)
			return
		}
		item, err := svc.UpdateDesiredReplicas(c.Request.Context(), in.DesiredReplicas)
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
}
