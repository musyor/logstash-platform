package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewAgentHandler(t *testing.T) {
	mockService := &MockConfigService{}
	logger := logrus.New()
	
	handler := NewAgentHandler(mockService, logger)
	
	assert.NotNil(t, handler)
	assert.Equal(t, mockService, handler.configService)
	assert.Equal(t, logger, handler.logger)
}

func TestAgentHandler_ListAgents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name           string
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:           "获取Agent列表返回空列表",
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"items":   []interface{}{},
				"total":   float64(0),
				"message": "Agent管理功能待实现",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建handler
			mockService := &MockConfigService{}
			logger := logrus.New()
			handler := NewAgentHandler(mockService, logger)
			
			// 创建测试上下文
			w := httptest.NewRecorder()
			c, router := gin.CreateTestContext(w)
			
			// 注册路由
			router.GET("/agents", handler.ListAgents)
			
			// 创建请求
			c.Request = httptest.NewRequest(http.MethodGet, "/agents", nil)
			
			// 执行请求
			router.ServeHTTP(w, c.Request)
			
			// 验证响应
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBody, response)
		})
	}
}

func TestAgentHandler_GetAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name           string
		agentID        string
		setupParams    func(*gin.Context)
		expectedStatus int
		expectedBody   map[string]interface{}
		checkError     bool
	}{
		{
			name:           "成功获取Agent信息",
			agentID:        "agent-001",
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"agent_id": "agent-001",
				"status":   "offline",
				"message":  "Agent详情功能待实现",
			},
		},
		{
			name:    "Agent ID为空返回错误",
			agentID: "",
			setupParams: func(c *gin.Context) {
				c.Params = nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"error":   "INVALID_REQUEST",
				"message": "Agent ID不能为空",
			},
			checkError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建handler
			mockService := &MockConfigService{}
			logger := logrus.New()
			handler := NewAgentHandler(mockService, logger)
			
			// 创建测试上下文
			w := httptest.NewRecorder()
			c, router := gin.CreateTestContext(w)
			
			// 注册路由
			router.GET("/agents/:id", handler.GetAgent)
			
			// 设置参数
			if tt.agentID != "" {
				c.Params = gin.Params{{Key: "id", Value: tt.agentID}}
				c.Request = httptest.NewRequest(http.MethodGet, "/agents/"+tt.agentID, nil)
			} else {
				c.Request = httptest.NewRequest(http.MethodGet, "/agents/", nil)
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
		})
	}
}

func TestAgentHandler_DeployConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name           string
		agentID        string
		setupParams    func(*gin.Context)
		expectedStatus int
		expectedBody   map[string]interface{}
		checkError     bool
	}{
		{
			name:           "成功部署配置",
			agentID:        "agent-002",
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"agent_id": "agent-002",
				"status":   "pending",
				"message":  "部署功能待实现",
			},
		},
		{
			name:    "Agent ID为空返回错误",
			agentID: "",
			setupParams: func(c *gin.Context) {
				c.Params = nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"error":   "INVALID_REQUEST",
				"message": "Agent ID不能为空",
			},
			checkError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建handler
			mockService := &MockConfigService{}
			logger := logrus.New()
			handler := NewAgentHandler(mockService, logger)
			
			// 创建测试上下文
			w := httptest.NewRecorder()
			c, router := gin.CreateTestContext(w)
			
			// 注册路由
			router.POST("/agents/:id/deploy", handler.DeployConfig)
			
			// 设置参数
			if tt.agentID != "" {
				c.Params = gin.Params{{Key: "id", Value: tt.agentID}}
				c.Request = httptest.NewRequest(http.MethodPost, "/agents/"+tt.agentID+"/deploy", nil)
			} else {
				c.Request = httptest.NewRequest(http.MethodPost, "/agents//deploy", nil)
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
		})
	}
}

func TestBatchDeploy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name           string
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:           "批量部署返回待实现信息",
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"status":  "pending",
				"message": "批量部署功能待实现",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建mock服务和logger
			mockService := &MockConfigService{}
			logger := logrus.New()
			
			// 创建测试上下文
			w := httptest.NewRecorder()
			c, router := gin.CreateTestContext(w)
			
			// 注册路由
			router.POST("/batch-deploy", BatchDeploy(mockService, logger))
			
			// 创建请求
			c.Request = httptest.NewRequest(http.MethodPost, "/batch-deploy", nil)
			
			// 执行请求
			router.ServeHTTP(w, c.Request)
			
			// 验证响应
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBody, response)
		})
	}
}

// TestAgentHandler_ConcurrentRequests 测试并发请求
func TestAgentHandler_ConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	mockService := &MockConfigService{}
	logger := logrus.New()
	handler := NewAgentHandler(mockService, logger)
	
	router := gin.New()
	router.GET("/agents", handler.ListAgents)
	router.GET("/agents/:id", handler.GetAgent)
	
	// 并发请求数
	concurrency := 50
	done := make(chan bool, concurrency*2)
	
	// 并发请求ListAgents
	for i := 0; i < concurrency; i++ {
		go func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/agents", nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}()
	}
	
	// 并发请求GetAgent
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/agents/agent-%d", id), nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}(i)
	}
	
	// 等待所有请求完成
	for i := 0; i < concurrency*2; i++ {
		<-done
	}
}

// TestAgentHandler_FullIntegration 完整集成测试
func TestAgentHandler_FullIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// 创建真实的logger
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	// 创建mock服务
	mockService := &MockConfigService{}
	
	// 创建handler
	handler := NewAgentHandler(mockService, logger)
	
	// 创建路由
	router := gin.New()
	router.GET("/api/agents", handler.ListAgents)
	router.GET("/api/agents/:id", handler.GetAgent)
	router.POST("/api/agents/:id/deploy", handler.DeployConfig)
	router.POST("/api/batch-deploy", BatchDeploy(mockService, logger))
	
	t.Run("完整Agent管理流程", func(t *testing.T) {
		// 1. 获取Agent列表
		w1 := httptest.NewRecorder()
		req1 := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)
		
		// 2. 获取特定Agent
		w2 := httptest.NewRecorder()
		req2 := httptest.NewRequest(http.MethodGet, "/api/agents/agent-test", nil)
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
		
		// 3. 部署配置到Agent
		w3 := httptest.NewRecorder()
		req3 := httptest.NewRequest(http.MethodPost, "/api/agents/agent-test/deploy", nil)
		router.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusOK, w3.Code)
		
		// 4. 批量部署
		w4 := httptest.NewRecorder()
		req4 := httptest.NewRequest(http.MethodPost, "/api/batch-deploy", nil)
		router.ServeHTTP(w4, req4)
		assert.Equal(t, http.StatusOK, w4.Code)
	})
}