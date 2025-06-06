package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestErrorHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupHandler   func(*gin.Context)
		expectedStatus int
		expectedCode   string
		expectedMsg    string
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "no errors - passes through",
			setupHandler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "success", resp["message"])
			},
		},
		{
			name: "binding error",
			setupHandler: func(c *gin.Context) {
				c.Error(errors.New("invalid json")).SetType(gin.ErrorTypeBind)
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_REQUEST",
			expectedMsg:    "invalid json",
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "INVALID_REQUEST", resp["code"])
				assert.Equal(t, "invalid json", resp["message"])
				assert.Nil(t, resp["details"])
			},
		},
		{
			name: "public error",
			setupHandler: func(c *gin.Context) {
				c.Error(errors.New("bad request data")).SetType(gin.ErrorTypePublic)
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "BAD_REQUEST",
			expectedMsg:    "bad request data",
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "BAD_REQUEST", resp["code"])
				assert.Equal(t, "bad request data", resp["message"])
			},
		},
		{
			name: "internal error",
			setupHandler: func(c *gin.Context) {
				c.Error(errors.New("database connection failed")).SetType(gin.ErrorTypePrivate)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_ERROR",
			expectedMsg:    "database connection failed",
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "INTERNAL_ERROR", resp["code"])
				assert.Equal(t, "database connection failed", resp["message"])
			},
		},
		{
			name: "multiple errors - uses last",
			setupHandler: func(c *gin.Context) {
				c.Error(errors.New("first error"))
				c.Error(errors.New("second error")).SetType(gin.ErrorTypeBind)
				c.Error(errors.New("last error")).SetType(gin.ErrorTypePublic)
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "BAD_REQUEST",
			expectedMsg:    "last error",
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "BAD_REQUEST", resp["code"])
				assert.Equal(t, "last error", resp["message"])
			},
		},
		{
			name: "preserves custom status code",
			setupHandler: func(c *gin.Context) {
				c.Status(http.StatusConflict)
				c.Error(errors.New("conflict error"))
			},
			expectedStatus: http.StatusConflict,
			expectedCode:   "INTERNAL_ERROR",
			expectedMsg:    "conflict error",
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "INTERNAL_ERROR", resp["code"])
			},
		},
		{
			name: "error with meta type",
			setupHandler: func(c *gin.Context) {
				c.Error(errors.New("meta error")).SetType(gin.ErrorTypeAny)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_ERROR",
			expectedMsg:    "meta error",
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "INTERNAL_ERROR", resp["code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			router := gin.New()
			router.Use(ErrorHandler())
			
			router.GET("/test", tt.setupHandler)
			
			// Execute
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			router.ServeHTTP(w, req)
			
			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			
			if tt.checkResponse != nil {
				tt.checkResponse(t, response)
			}
		})
	}
}

func TestHandleError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		statusCode     int
		code           string
		message        string
		expectedStatus int
		expectedBody   ErrorResponse
	}{
		{
			name:           "bad request error",
			statusCode:     http.StatusBadRequest,
			code:           "INVALID_PARAM",
			message:        "参数错误",
			expectedStatus: http.StatusBadRequest,
			expectedBody: ErrorResponse{
				Code:    "INVALID_PARAM",
				Message: "参数错误",
			},
		},
		{
			name:           "unauthorized error",
			statusCode:     http.StatusUnauthorized,
			code:           "UNAUTHORIZED",
			message:        "未授权访问",
			expectedStatus: http.StatusUnauthorized,
			expectedBody: ErrorResponse{
				Code:    "UNAUTHORIZED",
				Message: "未授权访问",
			},
		},
		{
			name:           "not found error",
			statusCode:     http.StatusNotFound,
			code:           "NOT_FOUND",
			message:        "资源不存在",
			expectedStatus: http.StatusNotFound,
			expectedBody: ErrorResponse{
				Code:    "NOT_FOUND",
				Message: "资源不存在",
			},
		},
		{
			name:           "internal server error",
			statusCode:     http.StatusInternalServerError,
			code:           "INTERNAL_ERROR",
			message:        "服务器内部错误",
			expectedStatus: http.StatusInternalServerError,
			expectedBody: ErrorResponse{
				Code:    "INTERNAL_ERROR",
				Message: "服务器内部错误",
			},
		},
		{
			name:           "custom error code",
			statusCode:     http.StatusForbidden,
			code:           "PERMISSION_DENIED",
			message:        "权限不足",
			expectedStatus: http.StatusForbidden,
			expectedBody: ErrorResponse{
				Code:    "PERMISSION_DENIED",
				Message: "权限不足",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			
			// Execute
			HandleError(c, tt.statusCode, tt.code, tt.message)
			
			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.True(t, c.IsAborted())
			
			var response ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBody, response)
		})
	}
}

func TestErrorHandlerWithCustomHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("error after response written", func(t *testing.T) {
		router := gin.New()
		router.Use(ErrorHandler())
		
		router.GET("/test", func(c *gin.Context) {
			// Add error first
			c.Error(errors.New("late error"))
			// The error handler will process this after c.Next()
		})
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		
		// Should get error response
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "INTERNAL_ERROR", response["code"])
		assert.Equal(t, "late error", response["message"])
	})
}

func TestErrorResponseSerialization(t *testing.T) {
	tests := []struct {
		name     string
		response ErrorResponse
		expected string
	}{
		{
			name: "with details",
			response: ErrorResponse{
				Code:    "ERROR_CODE",
				Message: "Error message",
				Details: "Additional details",
			},
			expected: `{"code":"ERROR_CODE","message":"Error message","details":"Additional details"}`,
		},
		{
			name: "without details",
			response: ErrorResponse{
				Code:    "ERROR_CODE",
				Message: "Error message",
			},
			expected: `{"code":"ERROR_CODE","message":"Error message"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.response)
			assert.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))
		})
	}
}

func BenchmarkErrorHandler(b *testing.B) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ErrorHandler())
	
	// Success case
	router.GET("/success", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	
	// Error case
	router.GET("/error", func(c *gin.Context) {
		c.Error(errors.New("test error"))
	})
	
	b.Run("success case", func(b *testing.B) {
		req, _ := http.NewRequest("GET", "/success", nil)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
	
	b.Run("error case", func(b *testing.B) {
		req, _ := http.NewRequest("GET", "/error", nil)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}