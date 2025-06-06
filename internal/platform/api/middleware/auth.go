package middleware

import (
	"github.com/gin-gonic/gin"
)

// AuthorizeWebSocket WebSocket认证中间件
func AuthorizeWebSocket() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: 实现WebSocket认证逻辑
		// 暂时允许所有连接
		c.Next()
	}
}