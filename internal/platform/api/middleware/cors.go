package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"net/http"
	"strings"
)

// CORS 跨域中间件
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !viper.GetBool("security.cors.enabled") {
			c.Next()
			return
		}

		origin := c.Request.Header.Get("Origin")
		
		// 检查是否是允许的源
		allowedOrigins := viper.GetStringSlice("security.cors.allowed_origins")
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		// 设置其他CORS头
		c.Header("Access-Control-Allow-Methods", strings.Join(viper.GetStringSlice("security.cors.allowed_methods"), ", "))
		c.Header("Access-Control-Allow-Headers", strings.Join(viper.GetStringSlice("security.cors.allowed_headers"), ", "))
		c.Header("Access-Control-Expose-Headers", strings.Join(viper.GetStringSlice("security.cors.exposed_headers"), ", "))

		if viper.GetBool("security.cors.allow_credentials") {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if maxAge := viper.GetInt("security.cors.max_age"); maxAge > 0 {
			c.Header("Access-Control-Max-Age", string(rune(maxAge)))
		}

		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}