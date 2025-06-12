//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"logstash-platform/internal/agent/client"
	"logstash-platform/internal/agent/config"
	"logstash-platform/internal/agent/core"
	"logstash-platform/internal/platform/models"
)

// TestWebSocketConnection WebSocket连接测试
func TestWebSocketConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试服务器
	server := NewTestPlatformServer()
	defer server.Close()

	// 创建Agent配置
	cfg := &config.AgentConfig{
		AgentID:               "test-agent-ws",
		ServerURL:             server.GetURL(),
		EnableWebSocket:       true,
		WebSocketPingInterval: 1 * time.Second,
		RequestTimeout:        5 * time.Second,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// 创建客户端
	apiClient, err := client.NewClient(cfg, logger)
	require.NoError(t, err)
	defer apiClient.Close()

	// 先通过HTTP注册
	ctx := context.Background()
	agent := &models.Agent{
		AgentID:  cfg.AgentID,
		Hostname: "test-host",
		IP:       "192.168.1.200",
		Status:   "online",
	}
	err = apiClient.Register(ctx, agent)
	require.NoError(t, err)

	// 创建消息处理器
	messageReceived := make(chan *core.WebSocketMessage, 10)
	handler := &testMessageHandler{
		onMessage: func(msgType string, payload []byte) error {
			msg := &core.WebSocketMessage{
				Type:      msgType,
				Timestamp: time.Now(),
				Payload:   json.RawMessage(payload),
			}
			messageReceived <- msg
			return nil
		},
	}

	// 建立WebSocket连接
	err = apiClient.ConnectWebSocket(ctx, cfg.AgentID, handler)
	assert.NoError(t, err)

	// 等待连接稳定
	time.Sleep(500 * time.Millisecond)

	// 从服务器发送消息
	testMsg := map[string]interface{}{
		"type":    "status_request",
		"payload": map[string]interface{}{"request_id": "123"},
	}
	err = server.SendWSMessage(cfg.AgentID, testMsg)
	assert.NoError(t, err)

	// 等待接收消息
	select {
	case msg := <-messageReceived:
		assert.Equal(t, "status_request", msg.Type)
		var payload map[string]interface{}
		err := json.Unmarshal(msg.Payload, &payload)
		assert.NoError(t, err)
		assert.Equal(t, "123", payload["request_id"])
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for WebSocket message")
	}
}

// TestWebSocketConfigDeployment 测试通过WebSocket部署配置
func TestWebSocketConfigDeployment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试服务器
	server := NewTestPlatformServer()
	defer server.Close()

	// 创建测试配置
	testConfig := &models.Config{
		ID:      "ws-deploy-config",
		Name:    "WebSocket Deploy Config",
		Type:    models.ConfigTypeInput,
		Content: `input { http { port => 8080 } }`,
		Version: 1,
		Enabled: true,
	}
	server.AddConfig(testConfig)

	// 创建临时目录
	configDir := t.TempDir()

	// 创建Agent配置
	cfg := &config.AgentConfig{
		AgentID:               "test-agent-ws-deploy",
		ServerURL:             server.GetURL(),
		ConfigDir:             configDir,
		EnableWebSocket:       true,
		WebSocketPingInterval: 1 * time.Second,
		RequestTimeout:        5 * time.Second,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// 创建组件
	apiClient, err := client.NewClient(cfg, logger)
	require.NoError(t, err)
	defer apiClient.Close()

	configMgr, err := config.NewManager(cfg, logger)
	require.NoError(t, err)

	// 注册Agent
	ctx := context.Background()
	agent := &models.Agent{
		AgentID:  cfg.AgentID,
		Hostname: "test-host",
		IP:       "192.168.1.201",
		Status:   "online",
	}
	err = apiClient.Register(ctx, agent)
	require.NoError(t, err)

	// 创建消息处理器（模拟配置部署）
	deploymentReceived := make(chan string, 1)
	handler := &testMessageHandler{
		onMessage: func(msgType string, payload []byte) error {
			if msgType == core.MsgTypeConfigDeploy {
				var deployMsg struct {
					ConfigID string `json:"config_id"`
					Version  int    `json:"version"`
				}
				if err := json.Unmarshal(payload, &deployMsg); err != nil {
					return err
				}

				// 模拟获取并保存配置
				config, err := apiClient.GetConfig(ctx, deployMsg.ConfigID)
				if err != nil {
					return err
				}

				if err := configMgr.SaveConfig(config); err != nil {
					return err
				}

				// 上报部署成功
				applied := &models.AppliedConfig{
					ConfigID:  deployMsg.ConfigID,
					Version:   deployMsg.Version,
					AppliedAt: time.Now(),
				}
				if err := apiClient.ReportConfigApplied(ctx, cfg.AgentID, applied); err != nil {
					return err
				}

				deploymentReceived <- deployMsg.ConfigID
			}
			return nil
		},
	}

	// 建立WebSocket连接
	err = apiClient.ConnectWebSocket(ctx, cfg.AgentID, handler)
	require.NoError(t, err)

	// 等待连接稳定
	time.Sleep(500 * time.Millisecond)

	// 通过WebSocket发送配置部署消息
	deployMsg := map[string]interface{}{
		"type": core.MsgTypeConfigDeploy,
		"payload": map[string]interface{}{
			"config_id": testConfig.ID,
			"version":   testConfig.Version,
		},
	}
	err = server.SendWSMessage(cfg.AgentID, deployMsg)
	assert.NoError(t, err)

	// 等待部署完成
	select {
	case configID := <-deploymentReceived:
		assert.Equal(t, testConfig.ID, configID)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for config deployment")
	}

	// 验证配置已保存
	savedConfig, err := configMgr.LoadConfig(testConfig.ID)
	assert.NoError(t, err)
	assert.Equal(t, testConfig.Content, savedConfig.Content)
}

// TestWebSocketReconnection 测试WebSocket重连机制
func TestWebSocketReconnection(t *testing.T) {
	t.Skip("Skipping reconnection test - needs implementation")
	// TODO: 实现重连测试
	// 1. 建立连接
	// 2. 服务器主动断开
	// 3. 验证客户端自动重连
	// 4. 验证重连后功能正常
}

// TestWebSocketHeartbeat 测试WebSocket心跳
func TestWebSocketHeartbeat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试服务器
	server := NewTestPlatformServer()
	defer server.Close()

	// 创建Agent配置（短心跳间隔）
	cfg := &config.AgentConfig{
		AgentID:               "test-agent-ws-heartbeat",
		ServerURL:             server.GetURL(),
		EnableWebSocket:       true,
		WebSocketPingInterval: 500 * time.Millisecond,
		HeartbeatInterval:     1 * time.Second,
		RequestTimeout:        5 * time.Second,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	// 创建客户端
	apiClient, err := client.NewClient(cfg, logger)
	require.NoError(t, err)
	defer apiClient.Close()

	// 注册Agent
	ctx := context.Background()
	agent := &models.Agent{
		AgentID:  cfg.AgentID,
		Hostname: "test-host",
		IP:       "192.168.1.202",
		Status:   "online",
	}
	err = apiClient.Register(ctx, agent)
	require.NoError(t, err)

	// 记录收到的心跳消息
	heartbeatCount := 0
	var mu sync.Mutex
	handler := &testMessageHandler{
		onMessage: func(msgType string, payload []byte) error {
			if msgType == core.MsgTypeHeartbeat {
				mu.Lock()
				heartbeatCount++
				mu.Unlock()
			}
			return nil
		},
	}

	// 建立WebSocket连接
	err = apiClient.ConnectWebSocket(ctx, cfg.AgentID, handler)
	require.NoError(t, err)

	// 发送几个心跳
	for i := 0; i < 3; i++ {
		err = apiClient.SendHeartbeat(ctx, cfg.AgentID)
		assert.NoError(t, err)
		time.Sleep(200 * time.Millisecond)
	}

	// 验证心跳是否通过WebSocket发送
	mu.Lock()
	count := heartbeatCount
	mu.Unlock()
	assert.GreaterOrEqual(t, count, 2, "Should have sent at least 2 heartbeats via WebSocket")
}

// 辅助类型

type testMessageHandler struct {
	onMessage    func(msgType string, payload []byte) error
	onConnect    func() error
	onDisconnect func(err error)
}

func (h *testMessageHandler) HandleMessage(msgType string, payload []byte) error {
	if h.onMessage != nil {
		return h.onMessage(msgType, payload)
	}
	return nil
}

func (h *testMessageHandler) OnConnect() error {
	if h.onConnect != nil {
		return h.onConnect()
	}
	return nil
}

func (h *testMessageHandler) OnDisconnect(err error) {
	if h.onDisconnect != nil {
		h.onDisconnect(err)
	}
}