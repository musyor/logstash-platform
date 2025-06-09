package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestHealthCheck(t *testing.T) {
	// 设置测试模式
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:           "成功返回健康状态",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				// 检查必需字段
				assert.Equal(t, "healthy", resp["status"])
				assert.Equal(t, "logstash-platform", resp["service"])
				
				// 检查时间戳
				timeValue, ok := resp["time"].(float64)
				assert.True(t, ok, "time 应该是数字")
				
				// 验证时间戳在合理范围内（当前时间前后5秒）
				now := time.Now().Unix()
				assert.True(t, timeValue >= float64(now-5) && timeValue <= float64(now+5), 
					"时间戳应该接近当前时间")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试路由
			w := httptest.NewRecorder()
			c, router := gin.CreateTestContext(w)
			
			// 注册路由
			router.GET("/health", HealthCheck)
			
			// 创建请求
			c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)
			
			// 执行请求
			router.ServeHTTP(w, c.Request)
			
			// 验证状态码
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			// 解析响应
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			
			// 执行自定义检查
			if tt.checkResponse != nil {
				tt.checkResponse(t, response)
			}
		})
	}
}

// TestHealthCheckConcurrency 测试并发健康检查
func TestHealthCheckConcurrency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// 创建路由
	router := gin.New()
	router.GET("/health", HealthCheck)
	
	// 并发请求数
	concurrency := 100
	done := make(chan bool, concurrency)
	
	// 启动并发请求
	for i := 0; i < concurrency; i++ {
		go func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			router.ServeHTTP(w, req)
			
			// 验证响应
			assert.Equal(t, http.StatusOK, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, "healthy", response["status"])
			
			done <- true
		}()
	}
	
	// 等待所有请求完成
	for i := 0; i < concurrency; i++ {
		<-done
	}
}

// BenchmarkHealthCheck 性能测试
func BenchmarkHealthCheck(b *testing.B) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health", HealthCheck)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			router.ServeHTTP(w, req)
		}
	})
}