//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"logstash-platform/internal/platform/models"
)

// TestPlatformServer 模拟的管理平台服务器
type TestPlatformServer struct {
	server      *httptest.Server
	router      *gin.Engine
	agents      map[string]*models.Agent
	configs     map[string]*models.Config
	heartbeats  map[string]time.Time
	metrics     map[string]*models.Agent
	testResults map[string]*models.TestResult
	mu          sync.RWMutex
	
	// WebSocket相关
	wsUpgrader websocket.Upgrader
	wsConns    map[string]*websocket.Conn
}

// NewTestPlatformServer 创建测试服务器
func NewTestPlatformServer() *TestPlatformServer {
	gin.SetMode(gin.TestMode)
	
	s := &TestPlatformServer{
		router:     gin.New(),
		agents:     make(map[string]*models.Agent),
		configs:    make(map[string]*models.Config),
		heartbeats: make(map[string]time.Time),
		metrics:    make(map[string]*models.Agent),
		wsConns:    make(map[string]*websocket.Conn),
		wsUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
	
	s.setupRoutes()
	s.server = httptest.NewServer(s.router)
	
	return s
}

// Start 启动服务器
func (s *TestPlatformServer) Start() error {
	return nil
}

// Stop 停止服务器
func (s *TestPlatformServer) Stop() {
	s.Close()
}

// GetURL 获取服务器URL
func (s *TestPlatformServer) GetURL() string {
	return s.server.URL
}

// Close 关闭服务器
func (s *TestPlatformServer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 关闭所有WebSocket连接
	for _, conn := range s.wsConns {
		conn.Close()
	}
	
	s.server.Close()
}

// setupRoutes 设置路由
func (s *TestPlatformServer) setupRoutes() {
	// Agent注册
	s.router.POST("/api/v1/agents/register", s.handleRegister)
	
	// 心跳
	s.router.POST("/api/v1/agents/:id/heartbeat", s.handleHeartbeat)
	
	// 状态上报
	s.router.PUT("/api/v1/agents/:id/status", s.handleStatusUpdate)
	
	// 配置管理
	s.router.GET("/api/v1/configs/:id", s.handleGetConfig)
	s.router.POST("/api/v1/agents/:id/configs/applied", s.handleConfigApplied)
	
	// 指标上报
	s.router.POST("/api/v1/agents/:id/metrics", s.handleMetricsReport)
	
	// WebSocket
	s.router.GET("/ws", s.handleWebSocket)
	
	// 测试相关路由
	s.router.POST("/api/v1/test", s.handleCreateTest)
	s.router.GET("/api/v1/test/:id", s.handleGetTestResult)
}

// handleRegister 处理注册请求
func (s *TestPlatformServer) handleRegister(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	
	agentID, _ := req["agent_id"].(string)
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Agent ID required"})
		return
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 检查是否已注册
	if _, exists := s.agents[agentID]; exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Agent already registered"})
		return
	}
	
	// 注册新Agent
	agent := &models.Agent{
		AgentID:       agentID,
		Hostname:      req["hostname"].(string),
		IP:            req["ip"].(string),
		Status:        "online",
		LastHeartbeat: time.Now(),
	}
	
	if version, ok := req["logstash_version"].(string); ok {
		agent.LogstashVersion = version
	}
	
	s.agents[agentID] = agent
	s.heartbeats[agentID] = time.Now()
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Agent registered successfully",
		"agent":   agent,
	})
}

// handleHeartbeat 处理心跳
func (s *TestPlatformServer) handleHeartbeat(c *gin.Context) {
	agentID := c.Param("id")
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	agent, exists := s.agents[agentID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}
	
	// 更新心跳时间
	s.heartbeats[agentID] = time.Now()
	agent.LastHeartbeat = time.Now()
	agent.Status = "online"
	
	c.JSON(http.StatusOK, gin.H{"message": "Heartbeat received"})
}

// handleStatusUpdate 处理状态更新
func (s *TestPlatformServer) handleStatusUpdate(c *gin.Context) {
	agentID := c.Param("id")
	
	var agent models.Agent
	if err := c.ShouldBindJSON(&agent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	existingAgent, exists := s.agents[agentID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}
	
	// 更新状态
	existingAgent.Status = agent.Status
	existingAgent.LastHeartbeat = time.Now()
	
	c.JSON(http.StatusOK, gin.H{"message": "Status updated"})
}

// handleGetConfig 处理获取配置请求
func (s *TestPlatformServer) handleGetConfig(c *gin.Context) {
	configID := c.Param("id")
	
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	config, exists := s.configs[configID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
		return
	}
	
	c.JSON(http.StatusOK, config)
}

// handleConfigApplied 处理配置应用结果
func (s *TestPlatformServer) handleConfigApplied(c *gin.Context) {
	agentID := c.Param("id")
	
	var applied models.AppliedConfig
	if err := c.ShouldBindJSON(&applied); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	agent, exists := s.agents[agentID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}
	
	// 更新应用的配置
	agent.AppliedConfigs = append(agent.AppliedConfigs, applied)
	
	c.JSON(http.StatusOK, gin.H{"message": "Config applied successfully"})
}

// handleMetricsReport 处理指标上报
func (s *TestPlatformServer) handleMetricsReport(c *gin.Context) {
	agentID := c.Param("id")
	
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	_, exists := s.agents[agentID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}
	
	// 存储指标（简化处理）
	c.JSON(http.StatusOK, gin.H{"message": "Metrics received"})
}

// handleWebSocket 处理WebSocket连接
func (s *TestPlatformServer) handleWebSocket(c *gin.Context) {
	agentID := c.Query("agent_id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Agent ID required"})
		return
	}
	
	conn, err := s.wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	
	s.mu.Lock()
	s.wsConns[agentID] = conn
	s.mu.Unlock()
	
	// 处理WebSocket消息
	go s.handleWSConnection(agentID, conn)
}

// handleWSConnection 处理WebSocket连接
func (s *TestPlatformServer) handleWSConnection(agentID string, conn *websocket.Conn) {
	defer func() {
		s.mu.Lock()
		delete(s.wsConns, agentID)
		s.mu.Unlock()
		conn.Close()
	}()
	
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		
		if messageType == websocket.TextMessage {
			var msg map[string]interface{}
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			
			// 处理认证消息
			if msg["type"] == "auth" {
				response := map[string]interface{}{
					"type":    "auth_success",
					"message": "Authenticated",
				}
				conn.WriteJSON(response)
			}
		}
	}
}

// 辅助方法

// AddConfig 添加测试配置
func (s *TestPlatformServer) AddConfig(config *models.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configs[config.ID] = config
}

// GetAgent 获取Agent
func (s *TestPlatformServer) GetAgent(agentID string) *models.Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.agents[agentID]
}

// SendWSMessage 发送WebSocket消息
func (s *TestPlatformServer) SendWSMessage(agentID string, msg interface{}) error {
	s.mu.RLock()
	conn, exists := s.wsConns[agentID]
	s.mu.RUnlock()
	
	if !exists {
		return http.ErrNotSupported
	}
	
	return conn.WriteJSON(msg)
}

// handleCreateTest 处理创建测试请求
func (s *TestPlatformServer) handleCreateTest(c *gin.Context) {
	var req models.TestConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_REQUEST",
			"message": err.Error(),
		})
		return
	}
	
	// 生成测试ID
	testID := generateID()
	
	// 创建测试结果
	testResult := &models.TestResult{
		TestID:      testID,
		Status:      "running",
		InputCount:  0,
		OutputCount: 0,
		Results:     []models.TestOutput{},
		Errors:      []string{},
		StartTime:   time.Now(),
	}
	
	s.mu.Lock()
	if s.testResults == nil {
		s.testResults = make(map[string]*models.TestResult)
	}
	s.testResults[testID] = testResult
	s.mu.Unlock()
	
	// 异步执行测试
	go s.executeTest(testID, &req)
	
	c.JSON(http.StatusAccepted, gin.H{
		"test_id": testID,
		"status":  "running",
		"message": "测试任务已创建",
	})
}

// handleGetTestResult 处理获取测试结果请求
func (s *TestPlatformServer) handleGetTestResult(c *gin.Context) {
	testID := c.Param("id")
	
	s.mu.RLock()
	result, exists := s.testResults[testID]
	s.mu.RUnlock()
	
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "TEST_NOT_FOUND",
			"message": "测试任务不存在",
		})
		return
	}
	
	c.JSON(http.StatusOK, result)
}

// executeTest 执行测试
func (s *TestPlatformServer) executeTest(testID string, req *models.TestConfigRequest) {
	// 获取配置
	s.mu.RLock()
	config, exists := s.configs[req.ConfigID]
	s.mu.RUnlock()
	
	if !exists {
		s.updateTestResult(testID, func(result *models.TestResult) {
			result.Status = "failed"
			result.Errors = append(result.Errors, "获取配置失败: 配置不存在")
			endTime := time.Now()
			result.EndTime = &endTime
		})
		return
	}
	
	// 根据测试类型执行
	switch req.TestData.Type {
	case "sample":
		s.executeSampleTest(testID, config, req.TestData.Samples)
	case "kafka":
		s.executeKafkaTest(testID, config, &req.TestData.KafkaConfig)
	default:
		s.updateTestResult(testID, func(result *models.TestResult) {
			result.Status = "failed"
			result.Errors = append(result.Errors, "不支持的测试类型: "+req.TestData.Type)
			endTime := time.Now()
			result.EndTime = &endTime
		})
	}
}

// executeSampleTest 执行样本测试
func (s *TestPlatformServer) executeSampleTest(testID string, config *models.Config, samples []string) {
	// 更新输入计数
	s.updateTestResult(testID, func(result *models.TestResult) {
		result.InputCount = len(samples)
	})
	
	// 模拟Logstash处理
	for i, sample := range samples {
		// 解析输入
		var input map[string]interface{}
		if err := json.Unmarshal([]byte(sample), &input); err != nil {
			// 如果不是JSON，作为字符串处理
			input = map[string]interface{}{"message": sample}
		}
		
		// 模拟处理
		output := make(map[string]interface{})
		for k, v := range input {
			output[k] = v
		}
		
		// 添加处理字段（简化的逻辑）
		output["@timestamp"] = time.Now().Format(time.RFC3339)
		output["processed"] = "true"
		
		// 根据配置内容添加特定处理
		if strings.Contains(config.Content, "add_field") {
			output["test_field"] = fmt.Sprintf("processed_%d", i)
		}
		
		// 如果有错误级别，添加标签
		if level, ok := input["level"]; ok && level == "ERROR" {
			output["tags"] = []interface{}{"error"}
		}
		
		// 添加到结果
		s.updateTestResult(testID, func(result *models.TestResult) {
			result.Results = append(result.Results, models.TestOutput{
				Input:  sample,
				Output: output,
			})
			result.OutputCount++
		})
		
		// 模拟处理延迟
		time.Sleep(100 * time.Millisecond)
	}
	
	// 标记完成
	s.updateTestResult(testID, func(result *models.TestResult) {
		result.Status = "completed"
		endTime := time.Now()
		result.EndTime = &endTime
	})
}

// executeKafkaTest 执行Kafka测试
func (s *TestPlatformServer) executeKafkaTest(testID string, config *models.Config, kafkaConfig *models.KafkaConfig) {
	// Kafka测试暂未实现
	s.updateTestResult(testID, func(result *models.TestResult) {
		result.Status = "failed"
		result.Errors = append(result.Errors, "Kafka测试功能尚未实现")
		endTime := time.Now()
		result.EndTime = &endTime
	})
}

// updateTestResult 更新测试结果
func (s *TestPlatformServer) updateTestResult(testID string, update func(*models.TestResult)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if result, exists := s.testResults[testID]; exists {
		update(result)
	}
}

// generateID 生成ID
func generateID() string {
	return uuid.New().String()
}