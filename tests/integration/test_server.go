//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"logstash-platform/internal/platform/models"
)

// TestPlatformServer 模拟的管理平台服务器
type TestPlatformServer struct {
	server     *httptest.Server
	router     *gin.Engine
	agents     map[string]*models.Agent
	configs    map[string]*models.Config
	heartbeats map[string]time.Time
	metrics    map[string]*models.Agent
	mu         sync.RWMutex
	
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