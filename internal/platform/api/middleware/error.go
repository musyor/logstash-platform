package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// ErrorHandler 错误处理中间件
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 处理错误
		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			
			// 根据错误类型返回不同的状态码
			var statusCode int
			var code string
			
			switch err.Type {
			case gin.ErrorTypeBind:
				statusCode = http.StatusBadRequest
				code = "INVALID_REQUEST"
			case gin.ErrorTypePublic:
				statusCode = http.StatusBadRequest
				code = "BAD_REQUEST"
			default:
				statusCode = http.StatusInternalServerError
				code = "INTERNAL_ERROR"
			}

			// 设置状态码
			if c.Writer.Status() == http.StatusOK {
				c.Status(statusCode)
			}

			c.JSON(c.Writer.Status(), ErrorResponse{
				Code:    code,
				Message: err.Error(),
			})
		}
	}
}

// HandleError 处理错误的辅助函数
func HandleError(c *gin.Context, statusCode int, code, message string) {
	c.AbortWithStatusJSON(statusCode, ErrorResponse{
		Code:    code,
		Message: message,
	})
}