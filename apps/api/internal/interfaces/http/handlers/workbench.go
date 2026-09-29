package handlers

import (
	"fluxa-api/internal/application/workbenchapp"

	"github.com/gin-gonic/gin"
)

func RegisterWorkbenchRoutes(r gin.IRouter, svc *workbenchapp.Service) {
	r.GET("/workbench", func(c *gin.Context) {
		item, err := svc.Get(c.Request.Context(), c.Query("project_id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, item)
	})
}
