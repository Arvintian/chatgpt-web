package middlewares

import (
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func RateLimitMiddleware(r rate.Limit, b int) gin.HandlerFunc {
	limiter := rate.NewLimiter(r, b)
	return func(c *gin.Context) {
		if !limiter.Allow() {
			// 请求被限制，返回错误信息
			c.JSON(429, gin.H{
				"status":  "Fail",
				"message": "Too many requests, please try again later",
				"data":    nil,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
