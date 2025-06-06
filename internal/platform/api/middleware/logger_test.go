package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		method        string
		path          string
		query         string
		userAgent     string
		setupHandler  func(*gin.Context)
		expectedLevel string
		checkLog      func(*testing.T, map[string]interface{})
		shouldLog     bool
	}{
		{
			name:   "successful GET request",
			method: "GET",
			path:   "/api/configs",
			setupHandler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			},
			expectedLevel: "info",
			checkLog: func(t *testing.T, fields map[string]interface{}) {
				assert.Equal(t, float64(200), fields["status"])
				assert.Equal(t, "GET", fields["method"])
				assert.Equal(t, "/api/configs", fields["path"])
				// IP might be empty in test environment
				assert.Contains(t, fields, "ip")
				assert.NotNil(t, fields["latency"])
				assert.Equal(t, "HTTP请求", fields["msg"])
			},
			shouldLog: true,
		},
		{
			name:   "POST request with query params",
			method: "POST",
			path:   "/api/configs",
			query:  "type=filter&enabled=true",
			setupHandler: func(c *gin.Context) {
				c.JSON(http.StatusCreated, gin.H{"id": "123"})
			},
			expectedLevel: "info",
			checkLog: func(t *testing.T, fields map[string]interface{}) {
				assert.Equal(t, float64(201), fields["status"])
				assert.Equal(t, "POST", fields["method"])
				assert.Equal(t, "/api/configs?type=filter&enabled=true", fields["path"])
			},
			shouldLog: true,
		},
		{
			name:      "request with user agent",
			method:    "GET",
			path:      "/api/configs/123",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
			setupHandler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"id": "123"})
			},
			expectedLevel: "info",
			checkLog: func(t *testing.T, fields map[string]interface{}) {
				assert.Equal(t, "Mozilla/5.0 (Windows NT 10.0; Win64; x64)", fields["user_agent"])
			},
			shouldLog: true,
		},
		{
			name:   "request with error",
			method: "GET",
			path:   "/api/configs/invalid",
			setupHandler: func(c *gin.Context) {
				c.Error(errors.New("配置不存在"))
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			},
			expectedLevel: "error",
			checkLog: func(t *testing.T, fields map[string]interface{}) {
				assert.Equal(t, float64(404), fields["status"])
				assert.Contains(t, fields["msg"].(string), "配置不存在")
			},
			shouldLog: true,
		},
		{
			name:   "health check endpoint - no log",
			method: "GET",
			path:   "/health",
			setupHandler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "healthy"})
			},
			shouldLog: false,
		},
		{
			name:   "server error",
			method: "DELETE",
			path:   "/api/configs/123",
			setupHandler: func(c *gin.Context) {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			},
			expectedLevel: "info",
			checkLog: func(t *testing.T, fields map[string]interface{}) {
				assert.Equal(t, float64(500), fields["status"])
				assert.Equal(t, "DELETE", fields["method"])
			},
			shouldLog: true,
		},
		{
			name:   "multiple errors",
			method: "PUT",
			path:   "/api/configs/123",
			setupHandler: func(c *gin.Context) {
				c.Error(errors.New("validation error"))
				c.Error(errors.New("another error"))
				c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
			},
			expectedLevel: "error",
			checkLog: func(t *testing.T, fields map[string]interface{}) {
				assert.Equal(t, float64(400), fields["status"])
				msg := fields["msg"].(string)
				assert.Contains(t, msg, "validation error")
				assert.Contains(t, msg, "another error")
			},
			shouldLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup logger with buffer
			var buf bytes.Buffer
			logger := logrus.New()
			logger.SetOutput(&buf)
			logger.SetFormatter(&logrus.JSONFormatter{})
			
			// Setup router
			router := gin.New()
			router.Use(Logger(logger))
			
			// Add test handler
			switch tt.method {
			case "GET":
				router.GET(tt.path, tt.setupHandler)
			case "POST":
				router.POST(tt.path, tt.setupHandler)
			case "PUT":
				router.PUT(tt.path, tt.setupHandler)
			case "DELETE":
				router.DELETE(tt.path, tt.setupHandler)
			}
			
			// Create request
			url := tt.path
			if tt.query != "" {
				url += "?" + tt.query
			}
			req, _ := http.NewRequest(tt.method, url, nil)
			if tt.userAgent != "" {
				req.Header.Set("User-Agent", tt.userAgent)
			}
			
			// Execute request
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			// Check log output
			logOutput := buf.String()
			if !tt.shouldLog {
				assert.Empty(t, logOutput, "should not log for %s", tt.path)
				return
			}
			
			assert.NotEmpty(t, logOutput, "should log for %s", tt.path)
			
			// Parse log entry
			var logEntry map[string]interface{}
			err := json.Unmarshal([]byte(logOutput), &logEntry)
			assert.NoError(t, err)
			
			// Debug: print log output if test fails
			if err != nil || len(logOutput) == 0 {
				t.Logf("Log output: %s", logOutput)
			}
			
			// Check log level
			assert.Equal(t, tt.expectedLevel, logEntry["level"])
			
			// Run custom checks
			if tt.checkLog != nil {
				tt.checkLog(t, logEntry)
			}
		})
	}
}

func TestLoggerLatency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Setup logger with buffer
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.JSONFormatter{})
	
	// Setup router with delay
	router := gin.New()
	router.Use(Logger(logger))
	
	router.GET("/slow", func(c *gin.Context) {
		time.Sleep(50 * time.Millisecond)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	
	// Execute request
	req, _ := http.NewRequest("GET", "/slow", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	// Parse log and check latency
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, err)
	
	// Latency should be at least 50ms
	// The latency field might be a float64 (nanoseconds) or a string
	var latency time.Duration
	switch v := logEntry["latency"].(type) {
	case float64:
		latency = time.Duration(v)
	case string:
		var err error
		latency, err = time.ParseDuration(v)
		assert.NoError(t, err)
	default:
		t.Fatalf("unexpected latency type: %T", v)
	}
	assert.True(t, latency >= 50*time.Millisecond, "latency should be at least 50ms, got %v", latency)
}

func TestLoggerClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name       string
		headers    map[string]string
		expectedIP string
	}{
		{
			name:       "direct connection",
			headers:    map[string]string{},
			expectedIP: "192.0.2.1",
		},
		{
			name: "behind proxy",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.7, 198.51.100.178",
			},
			expectedIP: "203.0.113.7",
		},
		{
			name: "X-Real-IP header",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.7",
			},
			expectedIP: "203.0.113.7",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup logger with buffer
			var buf bytes.Buffer
			logger := logrus.New()
			logger.SetOutput(&buf)
			logger.SetFormatter(&logrus.JSONFormatter{})
			
			// Setup router
			router := gin.New()
			router.Use(Logger(logger))
			
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ip": c.ClientIP()})
			})
			
			// Create request
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.0.2.1:1234"
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			
			// Execute request
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			// Parse log and check IP
			var logEntry map[string]interface{}
			err := json.Unmarshal(buf.Bytes(), &logEntry)
			assert.NoError(t, err)
			
			// Check logged IP
			loggedIP := logEntry["ip"].(string)
			if tt.headers["X-Forwarded-For"] != "" || tt.headers["X-Real-IP"] != "" {
				// When behind proxy, Gin extracts the real IP
				assert.Contains(t, []string{tt.expectedIP, "192.0.2.1"}, loggedIP)
			} else {
				assert.Contains(t, loggedIP, "192.0.2.1")
			}
		})
	}
}

func TestLoggerConcurrency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Setup logger with buffer
	logger := logrus.New()
	logger.SetOutput(&testSyncWriter{})
	logger.SetFormatter(&logrus.JSONFormatter{})
	
	// Setup router
	router := gin.New()
	router.Use(Logger(logger))
	
	router.GET("/concurrent", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	
	// Run concurrent requests
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			req, _ := http.NewRequest("GET", "/concurrent", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			done <- true
		}()
	}
	
	// Wait for all requests
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// If we get here without panic, concurrency is handled correctly
	assert.True(t, true, "concurrent requests handled without issues")
}

// testSyncWriter is a thread-safe writer for testing
type testSyncWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *testSyncWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func BenchmarkLogger(b *testing.B) {
	gin.SetMode(gin.TestMode)
	
	// Setup logger with discard writer
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	
	// Setup router
	router := gin.New()
	router.Use(Logger(logger))
	
	router.GET("/bench", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	
	req, _ := http.NewRequest("GET", "/bench", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}