package middleware

import (
	"net/http"
	"strings"

	"fluxa-api/internal/application/iamapp"
	"fluxa-api/internal/domain/iam"
	"fluxa-api/internal/security"

	"github.com/gin-gonic/gin"
)

func AuthRequired(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "missing_token"})
			return
		}
		claims, err := svc.ParseToken(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "invalid_token"})
			return
		}
		principal, err := svc.CurrentPrincipal(c.Request.Context(), claims)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "invalid_token"})
			return
		}
		c.Set("user_id", principal.UserID)
		c.Set("user_name", principal.UserName)
		c.Set("user_email", principal.UserEmail)
		c.Set("global_roles", principal.GlobalRoles)
		c.Set(security.PrincipalGinKey, principal)
		c.Request = c.Request.WithContext(security.WithPrincipal(c.Request.Context(), principal))
		c.Next()
	}
}

func AdminRequired(svc *iamapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		email := CurrentUserEmail(c)
		if email == "" || !svc.IsAdmin(c.Request.Context(), email) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "forbidden"})
			return
		}
		c.Next()
	}
}

func CurrentUserEmail(c *gin.Context) string {
	value, _ := c.Get("user_email")
	email, _ := value.(string)
	return strings.TrimSpace(email)
}

func UserHasGlobalRole(c *gin.Context, role string) bool {
	value, _ := c.Get("global_roles")
	roles, _ := value.([]string)
	for _, item := range roles {
		if item == role || (role == iam.GlobalRoleNormal && item == iam.GlobalRoleAdmin) {
			return true
		}
	}
	return false
}
