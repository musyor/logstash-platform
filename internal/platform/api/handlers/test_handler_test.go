package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewTestHandler(t *testing.T) {
	mockService := new(MockConfigService)
	logger := logrus.New()
	
	handler := NewTestHandler(mockService, logger)
	
	assert.NotNil(t, handler)
	assert.Equal(t, mockService, handler.configService)
	assert.Equal(t, logger, handler.logger)
}

func TestTestHandler_CreateTest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name           string
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:           "创建测试任务返回待实现信息",
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": "测试功能待实现",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建mock服务和handler
			mockService := new(MockConfigService)
			logger := logrus.New()
			handler := NewTestHandler(mockService, logger)
			
			// 创建测试上下文
			w := httptest.NewRecorder()
			c, router := gin.CreateTestContext(w)
			
			// 注册路由
			router.POST("/test", handler.CreateTest)
			
			// 创建请求
			c.Request = httptest.NewRequest(http.MethodPost, "/test", nil)
			
			// 执行请求
			router.ServeHTTP(w, c.Request)
			
			// 验证响应
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBody, response)
			
			// 验证mock调用
			mockService.AssertExpectations(t)
		})
	}
}

func TestTestHandler_GetTestResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name           string
		testID         string
		setupParams    func(*gin.Context)
		expectedStatus int
		expectedBody   map[string]interface{}
		checkError     bool
	}{
		{
			name:           "成功获取测试结果",
			testID:         "test-123",
			setupParams:    nil,
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"test_id": "test-123",
				"status":  "pending",
				"message": "测试结果功能待实现",
			},
			checkError: false,
		},
		{
			name:   "测试ID为空返回错误",
			testID: "",
			setupParams: func(c *gin.Context) {
				// 清除参数以模拟空ID
				c.Params = nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"error":   "INVALID_REQUEST",
				"message": "测试ID不能为空",
			},
			checkError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建mock服务和handler
			mockService := new(MockConfigService)
			logger := logrus.New()
			handler := NewTestHandler(mockService, logger)
			
			// 创建测试上下文
			w := httptest.NewRecorder()
			c, router := gin.CreateTestContext(w)
			
			// 注册路由
			router.GET("/test/:id", handler.GetTestResult)
			
			// 设置参数
			if tt.testID != "" {
				c.Params = gin.Params{
					{Key: "id", Value: tt.testID},
				}
				c.Request = httptest.NewRequest(http.MethodGet, "/test/"+tt.testID, nil)
			} else {
				c.Request = httptest.NewRequest(http.MethodGet, "/test/", nil)
			}
			
			// 如果有额外的参数设置
			if tt.setupParams != nil {
				tt.setupParams(c)
			}
			
			// 执行请求
			router.ServeHTTP(w, c.Request)
			
			// 验证响应
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			
			if tt.checkError {
				assert.Equal(t, tt.expectedBody["error"], response["error"])
				assert.Equal(t, tt.expectedBody["message"], response["message"])
			} else {
				assert.Equal(t, tt.expectedBody, response)
			}
			
			// 验证mock调用
			mockService.AssertExpectations(t)
		})
	}
}

// TestTestHandler_Integration 集成测试
func TestTestHandler_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// 创建真实的logger
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	// 创建mock服务
	mockService := new(MockConfigService)
	
	// 创建handler
	handler := NewTestHandler(mockService, logger)
	
	// 创建路由
	router := gin.New()
	router.POST("/api/test", handler.CreateTest)
	router.GET("/api/test/:id", handler.GetTestResult)
	
	t.Run("完整测试流程", func(t *testing.T) {
		// 1. 创建测试
		w1 := httptest.NewRecorder()
		req1 := httptest.NewRequest(http.MethodPost, "/api/test", nil)
		router.ServeHTTP(w1, req1)
		
		assert.Equal(t, http.StatusOK, w1.Code)
		
		// 2. 获取测试结果
		w2 := httptest.NewRecorder()
		req2 := httptest.NewRequest(http.MethodGet, "/api/test/test-456", nil)
		router.ServeHTTP(w2, req2)
		
		assert.Equal(t, http.StatusOK, w2.Code)
		
		var result map[string]interface{}
		err := json.Unmarshal(w2.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.Equal(t, "test-456", result["test_id"])
		assert.Equal(t, "pending", result["status"])
	})
}