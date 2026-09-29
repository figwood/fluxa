package handlers

import (
	"fluxa-api/internal/application/deliveryapp"
	"fluxa-api/internal/domain/delivery"
	"github.com/gin-gonic/gin"
	"strings"
)

func RegisterCIRoutes(r gin.IRouter, svc *deliveryapp.Service) {
	r.POST("/ci/artifacts", func(c *gin.Context) {
		var in delivery.ReportArtifactInput
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, err)
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
		item, err := svc.Report(c.Request.Context(), token, in)
		if err != nil {
			Error(c, err)
			return
		}
		Created(c, item)
	})
}
func RegisterDeliveryRoutes(r gin.IRouter, svc *deliveryapp.Service) {
	r.GET("/projects/:id/artifacts", func(c *gin.Context) {
		items, err := svc.ListArtifacts(c.Request.Context(), c.Param("id"), c.Query("service_id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	})
	r.GET("/projects/:id/deployments", func(c *gin.Context) {
		items, err := svc.ListDeployments(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	})
	r.GET("/projects/:id/environment-policies", func(c *gin.Context) {
		items, err := svc.ListPolicies(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, items)
	})
	r.PUT("/projects/:id/environment-policies", func(c *gin.Context) {
		var in []delivery.EnvironmentPolicy
		if err := c.ShouldBindJSON(&in); err != nil {
			Error(c, err)
			return
		}
		if err := svc.SetPolicies(c.Request.Context(), c.Param("id"), in); err != nil {
			Error(c, err)
			return
		}
		OK(c, gin.H{"updated": true})
	})
	r.POST("/projects/:id/ci-token/rotate", func(c *gin.Context) {
		token, err := svc.RotateCIToken(c.Request.Context(), c.Param("id"))
		if err != nil {
			Error(c, err)
			return
		}
		OK(c, gin.H{"token": token})
	})
}
