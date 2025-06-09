package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestWebSocketHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name           string
		setupLogger    func() *logrus.Logger
		expectedStatus int
		expectedBody   map[string]interface{}
		checkLog       bool
		expectedLog    string
	}{
		{
			name: "WebSocket连接请求返回待实现信息",
			setupLogger: func() *logrus.Logger {
				logger := logrus.New()
				logger.SetLevel(logrus.InfoLevel)
				return logger
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": "WebSocket功能待实现",
			},
			checkLog:    true,
			expectedLog: "WebSocket连接请求",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置logger
			logger := tt.setupLogger()
			
			// 捕获日志输出
			var logBuffer bytes.Buffer
			if tt.checkLog {
				logger.SetOutput(&logBuffer)
			}
			
			// 创建handler
			handler := WebSocketHandler(logger)
			
			// 创建测试上下文
			w := httptest.NewRecorder()
			c, router := gin.CreateTestContext(w)
			
			// 注册路由
			router.GET("/ws", handler)
			
			// 创建请求
			c.Request = httptest.NewRequest(http.MethodGet, "/ws", nil)
			
			// 执行请求
			router.ServeHTTP(w, c.Request)
			
			// 验证响应
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBody, response)
			
			// 检查日志
			if tt.checkLog {
				logOutput := logBuffer.String()
				assert.Contains(t, logOutput, tt.expectedLog)
			}
		})
	}
}

// TestWebSocketHandler_WithDifferentLogLevels 测试不同日志级别
func TestWebSocketHandler_WithDifferentLogLevels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name        string
		logLevel    logrus.Level
		expectLog   bool
	}{
		{
			name:      "Debug级别记录日志",
			logLevel:  logrus.DebugLevel,
			expectLog: true,
		},
		{
			name:      "Info级别记录日志",
			logLevel:  logrus.InfoLevel,
			expectLog: true,
		},
		{
			name:      "Warn级别不记录Info日志",
			logLevel:  logrus.WarnLevel,
			expectLog: false,
		},
		{
			name:      "Error级别不记录Info日志",
			logLevel:  logrus.ErrorLevel,
			expectLog: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建logger并设置级别
			logger := logrus.New()
			logger.SetLevel(tt.logLevel)
			
			// 捕获日志输出
			var logBuffer bytes.Buffer
			logger.SetOutput(&logBuffer)
			
			// 创建handler
			handler := WebSocketHandler(logger)
			
			// 创建测试上下文
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			
			// 创建请求
			c.Request = httptest.NewRequest(http.MethodGet, "/ws", nil)
			
			// 执行handler
			handler(c)
			
			// 验证日志输出
			logOutput := logBuffer.String()
			if tt.expectLog {
				assert.Contains(t, logOutput, "WebSocket连接请求")
			} else {
				assert.NotContains(t, logOutput, "WebSocket连接请求")
			}
		})
	}
}

// TestWebSocketHandler_Concurrent 测试并发WebSocket请求
func TestWebSocketHandler_Concurrent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	logger := logrus.New()
	handler := WebSocketHandler(logger)
	
	router := gin.New()
	router.GET("/ws", handler)
	
	// 并发请求数
	concurrency := 100
	done := make(chan bool, concurrency)
	
	// 启动并发请求
	for i := 0; i < concurrency; i++ {
		go func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			router.ServeHTTP(w, req)
			
			// 验证响应
			assert.Equal(t, http.StatusOK, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, "WebSocket功能待实现", response["message"])
			
			done <- true
		}()
	}
	
	// 等待所有请求完成
	for i := 0; i < concurrency; i++ {
		<-done
	}
}

// TestWebSocketHandler_LoggerFormats 测试不同的日志格式
func TestWebSocketHandler_LoggerFormats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name          string
		formatter     logrus.Formatter
		checkContains string
	}{
		{
			name:          "文本格式",
			formatter:     &logrus.TextFormatter{DisableTimestamp: true},
			checkContains: "level=info msg=\"WebSocket连接请求\"",
		},
		{
			name:          "JSON格式",
			formatter:     &logrus.JSONFormatter{},
			checkContains: `"msg":"WebSocket连接请求"`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建logger并设置格式
			logger := logrus.New()
			logger.SetFormatter(tt.formatter)
			
			// 捕获日志输出
			var logBuffer bytes.Buffer
			logger.SetOutput(&logBuffer)
			
			// 创建handler
			handler := WebSocketHandler(logger)
			
			// 创建测试上下文
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/ws", nil)
			
			// 执行handler
			handler(c)
			
			// 验证日志格式
			logOutput := logBuffer.String()
			assert.Contains(t, logOutput, tt.checkContains)
		})
	}
}

// BenchmarkWebSocketHandler 性能测试
func BenchmarkWebSocketHandler(b *testing.B) {
	gin.SetMode(gin.TestMode)
	
	logger := logrus.New()
	logger.SetOutput(nil) // 禁用日志输出以获得准确的性能数据
	
	handler := WebSocketHandler(logger)
	router := gin.New()
	router.GET("/ws", handler)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			router.ServeHTTP(w, req)
		}
	})
}

// TestWebSocketHandler_Integration 集成测试
func TestWebSocketHandler_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// 创建真实的logger配置
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.SetFormatter(&logrus.JSONFormatter{})
	
	// 捕获日志
	var logBuffer bytes.Buffer
	logger.SetOutput(&logBuffer)
	
	// 创建路由
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/api/ws", WebSocketHandler(logger))
	
	t.Run("WebSocket端点集成测试", func(t *testing.T) {
		// 发送请求
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/ws", nil)
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Connection", "upgrade")
		
		router.ServeHTTP(w, req)
		
		// 验证响应
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "WebSocket功能待实现", response["message"])
		
		// 验证日志
		logOutput := logBuffer.String()
		assert.Contains(t, logOutput, "WebSocket连接请求")
		
		// 解析JSON日志验证结构
		var logEntry map[string]interface{}
		err = json.Unmarshal([]byte(logOutput), &logEntry)
		assert.NoError(t, err)
		assert.Equal(t, "info", logEntry["level"])
		assert.Equal(t, "WebSocket连接请求", logEntry["msg"])
	})
}