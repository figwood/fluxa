package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateWindow struct {
	count int
	reset time.Time
}

func SensitiveRateLimit() gin.HandlerFunc {
	var mu sync.Mutex
	windows := map[string]rateWindow{}
	return func(c *gin.Context) {
		limit := 0
		switch c.Request.URL.Path {
		case "/api/v1/auth/login":
			limit = 20
		case "/api/v1/ci/artifacts":
			limit = 120
		}
		if limit == 0 {
			c.Next()
			return
		}
		now := time.Now()
		key := c.Request.URL.Path + ":" + c.ClientIP()
		mu.Lock()
		window := windows[key]
		if window.reset.IsZero() || now.After(window.reset) {
			window = rateWindow{reset: now.Add(time.Minute)}
		}
		window.count++
		windows[key] = window
		if len(windows) > 10000 {
			for itemKey, item := range windows {
				if now.After(item.reset) {
					delete(windows, itemKey)
				}
			}
		}
		blocked := window.count > limit
		mu.Unlock()
		if blocked {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"code": 429, "message": "rate_limited"})
			return
		}
		c.Next()
	}
}
