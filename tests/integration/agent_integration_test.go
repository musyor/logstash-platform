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
	"logstash-platform/internal/agent/config"
	"logstash-platform/internal/agent/core"
	"logstash-platform/internal/platform/models"
)

// TestAgentIntegration 测试Agent的基本功能
func TestAgentIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试配置
	cfg := &config.AgentConfig{
		AgentID:              "integration-test-agent",
		ServerURL:            "http://localhost:8080",
		Token:                "test-token",
		LogstashPath:         "/usr/share/logstash/bin/logstash",
		ConfigDir:            "/tmp/test-logstash/conf.d",
		DataDir:              "/tmp/test-logstash/data",
		LogDir:               "/tmp/test-logstash/logs",
		PipelineWorkers:      2,
		BatchSize:            125,
		HeartbeatInterval:    1 * time.Second,
		MetricsInterval:      2 * time.Second,
		ReconnectInterval:    5 * time.Second,
		RequestTimeout:       30 * time.Second,
		MaxReconnectAttempts: 3,
		EnableWebSocket:      false, // 禁用WebSocket以简化测试
		EnableAutoReload:     true,
		MaxConfigSize:        10 * 1024 * 1024,
		ConfigBackupCount:    3,
		ReloadDebounceTime:   5 * time.Second,
	}

	// 创建日志
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// 创建基本Agent
	agent, err := core.NewAgent(cfg, logger)
	require.NoError(t, err)

	// 创建mock组件
	mockAPIClient := &mockAPIClient{}
	mockConfigMgr := &mockConfigManager{}
	mockLogstashCtrl := &mockLogstashController{running: false}
	mockHeartbeat := &mockHeartbeatService{}
	mockMetrics := &mockMetricsCollector{}

	// 组装Agent
	agent.
		WithAPIClient(mockAPIClient).
		WithConfigManager(mockConfigMgr).
		WithLogstashController(mockLogstashCtrl).
		WithHeartbeatService(mockHeartbeat).
		WithMetricsCollector(mockMetrics)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动Agent
	err = agent.Start(ctx)
	assert.NoError(t, err)

	// 等待一些时间让Agent运行
	time.Sleep(3 * time.Second)

	// 验证状态
	status := agent.GetStatus()
	assert.Equal(t, "integration-test-agent", status.AgentID)
	assert.Equal(t, "online", status.Status)

	// 停止Agent
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	err = agent.Stop(stopCtx)
	assert.NoError(t, err)

	// 验证停止后的状态
	status = agent.GetStatus()
	assert.Equal(t, "offline", status.Status)
}

// Mock implementations for testing
type mockAPIClient struct{}

func (m *mockAPIClient) Register(ctx context.Context, agent *models.Agent) error {
	return nil
}

func (m *mockAPIClient) SendHeartbeat(ctx context.Context, agentID string) error {
	return nil
}

func (m *mockAPIClient) ReportStatus(ctx context.Context, agent *models.Agent) error {
	return nil
}

func (m *mockAPIClient) GetConfig(ctx context.Context, configID string) (*models.Config, error) {
	return nil, nil
}

func (m *mockAPIClient) ReportConfigApplied(ctx context.Context, agentID string, applied *models.AppliedConfig) error {
	return nil
}

func (m *mockAPIClient) ConnectWebSocket(ctx context.Context, agentID string, handler core.MessageHandler) error {
	return nil
}

func (m *mockAPIClient) ReportMetrics(ctx context.Context, agentID string, metrics *core.AgentMetrics) error {
	return nil
}

func (m *mockAPIClient) Close() error {
	return nil
}

type mockConfigManager struct{}

func (m *mockConfigManager) SaveConfig(config *models.Config) error {
	return nil
}

func (m *mockConfigManager) LoadConfig(configID string) (*models.Config, error) {
	return nil, nil
}

func (m *mockConfigManager) DeleteConfig(configID string) error {
	return nil
}

func (m *mockConfigManager) ListConfigs() ([]*models.Config, error) {
	return nil, nil
}

func (m *mockConfigManager) GetConfigPath(configID string) string {
	return "/tmp/test-config.conf"
}

func (m *mockConfigManager) BackupConfig(configID string) error {
	return nil
}

func (m *mockConfigManager) RestoreConfig(configID string) error {
	return nil
}

type mockLogstashController struct {
	running bool
}

func (m *mockLogstashController) Start(ctx context.Context) error {
	m.running = true
	return nil
}

func (m *mockLogstashController) Stop(ctx context.Context) error {
	m.running = false
	return nil
}

func (m *mockLogstashController) Restart(ctx context.Context) error {
	return nil
}

func (m *mockLogstashController) Reload(ctx context.Context) error {
	return nil
}

func (m *mockLogstashController) IsRunning() bool {
	return m.running
}

func (m *mockLogstashController) GetStatus() (*core.LogstashStatus, error) {
	return &core.LogstashStatus{
		Running: m.running,
		Version: "8.0.0",
	}, nil
}

func (m *mockLogstashController) ValidateConfig(configPath string) error {
	return nil
}

type mockHeartbeatService struct{}

func (m *mockHeartbeatService) Start(ctx context.Context) error {
	return nil
}

func (m *mockHeartbeatService) Stop() error {
	return nil
}

func (m *mockHeartbeatService) SetInterval(interval time.Duration) {}

type mockMetricsCollector struct{}

func (m *mockMetricsCollector) Start(ctx context.Context) error {
	return nil
}

func (m *mockMetricsCollector) Stop() error {
	return nil
}

func (m *mockMetricsCollector) GetMetrics() (*core.AgentMetrics, error) {
	return &core.AgentMetrics{
		CPUUsage:    50.0,
		MemoryUsage: 60.0,
		DiskUsage:   70.0,
		Uptime:      3600,
	}, nil
}

func (m *mockMetricsCollector) SetInterval(interval time.Duration) {}