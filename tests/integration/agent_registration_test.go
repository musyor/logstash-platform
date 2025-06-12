//go:build integration
// +build integration

package integration

import (
	"context"
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

// TestAgentRegistration 测试Agent注册流程
func TestAgentRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试服务器
	server := NewTestPlatformServer()
	defer server.Close()

	// 创建Agent配置
	cfg := &config.AgentConfig{
		AgentID:           "test-agent-001",
		ServerURL:         server.GetURL(),
		Token:             "test-token",
		HeartbeatInterval: 1 * time.Second,
		RequestTimeout:    5 * time.Second,
	}

	// 创建日志
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	// 创建API客户端
	apiClient, err := client.NewClient(cfg, logger)
	require.NoError(t, err)
	defer apiClient.Close()

	// 测试注册
	ctx := context.Background()
	agent := &models.Agent{
		AgentID:         cfg.AgentID,
		Hostname:        "test-host",
		IP:              "192.168.1.100",
		LogstashVersion: "8.0.0",
		Status:          "online",
	}

	err = apiClient.Register(ctx, agent)
	assert.NoError(t, err)

	// 验证服务器端收到注册
	registeredAgent := server.GetAgent(cfg.AgentID)
	assert.NotNil(t, registeredAgent)
	assert.Equal(t, cfg.AgentID, registeredAgent.AgentID)
	assert.Equal(t, "test-host", registeredAgent.Hostname)
	assert.Equal(t, "192.168.1.100", registeredAgent.IP)

	// 测试重复注册
	err = apiClient.Register(ctx, agent)
	assert.Error(t, err) // 应该返回冲突错误
}

// TestAgentHeartbeat 测试心跳机制
func TestAgentHeartbeat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试服务器
	server := NewTestPlatformServer()
	defer server.Close()

	// 创建并注册Agent
	cfg := &config.AgentConfig{
		AgentID:           "test-agent-002",
		ServerURL:         server.GetURL(),
		HeartbeatInterval: 500 * time.Millisecond,
		RequestTimeout:    5 * time.Second,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	apiClient, err := client.NewClient(cfg, logger)
	require.NoError(t, err)
	defer apiClient.Close()

	// 先注册
	ctx := context.Background()
	agent := &models.Agent{
		AgentID:  cfg.AgentID,
		Hostname: "test-host",
		IP:       "192.168.1.101",
		Status:   "online",
	}
	err = apiClient.Register(ctx, agent)
	require.NoError(t, err)

	// 发送心跳
	err = apiClient.SendHeartbeat(ctx, cfg.AgentID)
	assert.NoError(t, err)

	// 等待一下确保心跳被处理
	time.Sleep(100 * time.Millisecond)

	// 验证心跳更新
	registeredAgent := server.GetAgent(cfg.AgentID)
	assert.NotNil(t, registeredAgent)
	assert.WithinDuration(t, time.Now(), registeredAgent.LastHeartbeat, 2*time.Second)
}

// TestAgentStatusReport 测试状态上报
func TestAgentStatusReport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试服务器
	server := NewTestPlatformServer()
	defer server.Close()

	// 创建Agent
	cfg := &config.AgentConfig{
		AgentID:        "test-agent-003",
		ServerURL:      server.GetURL(),
		RequestTimeout: 5 * time.Second,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	apiClient, err := client.NewClient(cfg, logger)
	require.NoError(t, err)
	defer apiClient.Close()

	// 注册Agent
	ctx := context.Background()
	agent := &models.Agent{
		AgentID:  cfg.AgentID,
		Hostname: "test-host",
		IP:       "192.168.1.102",
		Status:   "online",
	}
	err = apiClient.Register(ctx, agent)
	require.NoError(t, err)

	// 上报不同状态
	testCases := []struct {
		name   string
		status string
	}{
		{"报告在线状态", "online"},
		{"报告离线状态", "offline"},
		{"报告错误状态", "error"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			agent.Status = tc.status
			err := apiClient.ReportStatus(ctx, agent)
			assert.NoError(t, err)

			// 验证状态更新
			registeredAgent := server.GetAgent(cfg.AgentID)
			assert.Equal(t, tc.status, registeredAgent.Status)
		})
	}
}

// TestAgentMetricsReport 测试指标上报
func TestAgentMetricsReport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试服务器
	server := NewTestPlatformServer()
	defer server.Close()

	// 创建Agent
	cfg := &config.AgentConfig{
		AgentID:        "test-agent-004",
		ServerURL:      server.GetURL(),
		RequestTimeout: 5 * time.Second,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	apiClient, err := client.NewClient(cfg, logger)
	require.NoError(t, err)
	defer apiClient.Close()

	// 注册Agent
	ctx := context.Background()
	agent := &models.Agent{
		AgentID:  cfg.AgentID,
		Hostname: "test-host",
		IP:       "192.168.1.103",
		Status:   "online",
	}
	err = apiClient.Register(ctx, agent)
	require.NoError(t, err)

	// 上报指标
	metrics := &core.AgentMetrics{
		Timestamp:      time.Now(),
		CPUUsage:       45.5,
		MemoryUsage:    60.2,
		DiskUsage:      70.0,
		EventsReceived: 1000,
		EventsSent:     950,
		EventsFailed:   50,
		Uptime:         3600,
	}

	err = apiClient.ReportMetrics(ctx, cfg.AgentID, metrics)
	assert.NoError(t, err)
}

// TestAgentFullLifecycle 测试Agent完整生命周期
func TestAgentFullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试服务器
	server := NewTestPlatformServer()
	defer server.Close()

	// 创建Agent配置
	cfg := &config.AgentConfig{
		AgentID:              "test-agent-lifecycle",
		ServerURL:            server.GetURL(),
		LogstashPath:         "/usr/share/logstash/bin/logstash",
		ConfigDir:            t.TempDir(),
		DataDir:              t.TempDir(),
		LogDir:               t.TempDir(),
		HeartbeatInterval:    1 * time.Second,
		MetricsInterval:      2 * time.Second,
		RequestTimeout:       5 * time.Second,
		EnableWebSocket:      false,
	}

	// 创建日志
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// 创建Agent
	agent, err := core.NewAgent(cfg, logger)
	require.NoError(t, err)

	// 创建真实的客户端
	apiClient, err := client.NewClient(cfg, logger)
	require.NoError(t, err)

	// 创建配置管理器
	configMgr, err := config.NewManager(cfg, logger)
	require.NoError(t, err)

	// 组装Agent（使用部分真实组件）
	agent.
		WithAPIClient(apiClient).
		WithConfigManager(configMgr).
		WithLogstashController(&mockLogstashController{}).
		WithHeartbeatService(&mockHeartbeatService{}).
		WithMetricsCollector(&mockMetricsCollector{})

	// 启动Agent
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = agent.Start(ctx)
	require.NoError(t, err)

	// 等待Agent运行
	time.Sleep(2 * time.Second)

	// 验证Agent已注册
	registeredAgent := server.GetAgent(cfg.AgentID)
	assert.NotNil(t, registeredAgent)
	assert.Equal(t, "online", registeredAgent.Status)

	// 获取状态
	status := agent.GetStatus()
	assert.Equal(t, cfg.AgentID, status.AgentID)
	assert.Equal(t, "online", status.Status)

	// 停止Agent
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	err = agent.Stop(stopCtx)
	assert.NoError(t, err)

	// 验证状态
	status = agent.GetStatus()
	assert.Equal(t, "offline", status.Status)
}