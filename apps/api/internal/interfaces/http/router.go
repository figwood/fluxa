package httpapi

import (
	"net/http"
	"time"

	"fluxa-api/internal/bootstrap"
	"fluxa-api/internal/interfaces/http/handlers"
	"fluxa-api/internal/interfaces/http/middleware"
	"fluxa-api/internal/shared"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewRouter(container *bootstrap.Container) http.Handler {
	r := gin.New()
	_ = r.SetTrustedProxies(container.Config.TrustedProxies)
	r.Use(gin.Recovery())
	r.Use(middleware.SensitiveRateLimit())
	r.Use(func(c *gin.Context) {
		started := time.Now()
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = shared.NewID("req")
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
		fields := []zap.Field{zap.String("request_id", requestID), zap.String("method", c.Request.Method), zap.String("path", c.Request.URL.Path), zap.Int("status", c.Writer.Status()), zap.Duration("duration", time.Since(started))}
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("error", c.Errors.String()))
			container.Logger.Error("http request", fields...)
		} else {
			container.Logger.Info("http request", fields...)
		}
	})
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.GET("/readyz", func(c *gin.Context) {
		sqlDB, err := container.DB.DB()
		if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	v1 := r.Group("/api/v1")
	handlers.RegisterAuthPublicRoutes(v1, container.IAM)
	handlers.RegisterCIRoutes(v1, container.Delivery)

	protected := v1.Group("")
	protected.Use(middleware.AuthRequired(container.IAM))
	handlers.RegisterAuthRoutes(protected, container.IAM)
	handlers.RegisterWorkbenchRoutes(protected, container.Workbench)
	handlers.RegisterProjectRoutes(protected, container.Projects)
	handlers.RegisterCatalogRoutes(protected, container.Catalog)
	handlers.RegisterDeliveryRoutes(protected, container.Delivery)
	handlers.RegisterTaskRoutes(protected, container.Tasks)
	handlers.RegisterReleaseRoutes(protected, container.Releases)
	handlers.RegisterWorkflowRoutes(protected, container.Workflows)
	handlers.RegisterWorkflowRunRoutes(protected, container.WorkflowRuns)

	admin := protected.Group("")
	admin.Use(middleware.AdminRequired(container.IAM))
	handlers.RegisterWorkerRoutes(admin, container.Workers)
	handlers.RegisterWorkflowAdminRoutes(admin, container.Workflows)
	handlers.RegisterIAMRoutes(admin, container.IAM)
	return r
}
