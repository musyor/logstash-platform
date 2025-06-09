package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"logstash-platform/internal/agent/config"
	"logstash-platform/internal/platform/models"
)

// Mock implementations
type MockAPIClient struct {
	mock.Mock
}

func (m *MockAPIClient) Register(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockAPIClient) SendHeartbeat(ctx context.Context, agentID string) error {
	args := m.Called(ctx, agentID)
	return args.Error(0)
}

func (m *MockAPIClient) ReportStatus(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockAPIClient) GetConfig(ctx context.Context, configID string) (*models.Config, error) {
	args := m.Called(ctx, configID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Config), args.Error(1)
}

func (m *MockAPIClient) ReportConfigApplied(ctx context.Context, agentID string, applied *models.AppliedConfig) error {
	args := m.Called(ctx, agentID, applied)
	return args.Error(0)
}

func (m *MockAPIClient) ConnectWebSocket(ctx context.Context, agentID string, handler MessageHandler) error {
	args := m.Called(ctx, agentID, handler)
	return args.Error(0)
}

func (m *MockAPIClient) ReportMetrics(ctx context.Context, agentID string, metrics *AgentMetrics) error {
	args := m.Called(ctx, agentID, metrics)
	return args.Error(0)
}

func (m *MockAPIClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

type MockConfigManager struct {
	mock.Mock
}

func (m *MockConfigManager) SaveConfig(config *models.Config) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockConfigManager) LoadConfig(configID string) (*models.Config, error) {
	args := m.Called(configID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Config), args.Error(1)
}

func (m *MockConfigManager) DeleteConfig(configID string) error {
	args := m.Called(configID)
	return args.Error(0)
}

func (m *MockConfigManager) ListConfigs() ([]*models.Config, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Config), args.Error(1)
}

func (m *MockConfigManager) GetConfigPath(configID string) string {
	args := m.Called(configID)
	return args.String(0)
}

func (m *MockConfigManager) BackupConfig(configID string) error {
	args := m.Called(configID)
	return args.Error(0)
}

func (m *MockConfigManager) RestoreConfig(configID string) error {
	args := m.Called(configID)
	return args.Error(0)
}

type MockLogstashController struct {
	mock.Mock
}

func (m *MockLogstashController) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockLogstashController) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockLogstashController) Restart(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockLogstashController) Reload(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockLogstashController) IsRunning() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockLogstashController) GetStatus() (*LogstashStatus, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*LogstashStatus), args.Error(1)
}

func (m *MockLogstashController) ValidateConfig(configPath string) error {
	args := m.Called(configPath)
	return args.Error(0)
}

type MockHeartbeatService struct {
	mock.Mock
}

func (m *MockHeartbeatService) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockHeartbeatService) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockHeartbeatService) SetInterval(interval time.Duration) {
	m.Called(interval)
}

type MockMetricsCollector struct {
	mock.Mock
}

func (m *MockMetricsCollector) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockMetricsCollector) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockMetricsCollector) GetMetrics() (*AgentMetrics, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AgentMetrics), args.Error(1)
}

func (m *MockMetricsCollector) SetInterval(interval time.Duration) {
	m.Called(interval)
}

// Test helper functions
func createTestAgent(t *testing.T) (*Agent, *MockAPIClient, *MockConfigManager, *MockLogstashController, *MockHeartbeatService, *MockMetricsCollector) {
	cfg := &config.AgentConfig{
		AgentID:              "test-agent",
		ServerURL:            "http://localhost:8080",
		HeartbeatInterval:    30 * time.Second,
		MetricsInterval:      60 * time.Second,
		ReconnectInterval:    5 * time.Second,
		EnableWebSocket:      true,
		EnableAutoReload:     true,
		MaxReconnectAttempts: 3,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	agent, err := NewAgent(cfg, logger)
	assert.NoError(t, err)

	// 创建mock对象
	mockAPIClient := new(MockAPIClient)
	mockConfigMgr := new(MockConfigManager)
	mockLogstashCtrl := new(MockLogstashController)
	mockHeartbeat := new(MockHeartbeatService)
	mockMetrics := new(MockMetricsCollector)

	// 注入mock对象
	agent.apiClient = mockAPIClient
	agent.configMgr = mockConfigMgr
	agent.logstashCtrl = mockLogstashCtrl
	agent.heartbeat = mockHeartbeat
	agent.metrics = mockMetrics

	return agent, mockAPIClient, mockConfigMgr, mockLogstashCtrl, mockHeartbeat, mockMetrics
}

// Tests
func TestNewAgent(t *testing.T) {
	cfg := &config.AgentConfig{
		AgentID:   "test-agent",
		ServerURL: "http://localhost:8080",
	}

	logger := logrus.New()

	agent, err := NewAgent(cfg, logger)
	assert.NoError(t, err)
	assert.NotNil(t, agent)
	assert.Equal(t, "test-agent", agent.config.AgentID)
	assert.NotNil(t, agent.status)
	assert.Equal(t, "offline", agent.status.Status)
}

func TestAgent_Start(t *testing.T) {
	agent, mockAPI, _, mockLogstash, mockHeartbeat, mockMetrics := createTestAgent(t)

	// 设置mock期望
	mockAPI.On("Register", mock.Anything, mock.Anything).Return(nil)
	mockLogstash.On("Start", mock.Anything).Return(nil)
	mockLogstash.On("GetStatus").Return(&LogstashStatus{
		Version: "8.0.0",
		Running: true,
	}, nil)
	mockHeartbeat.On("Start", mock.Anything).Return(nil)
	mockMetrics.On("Start", mock.Anything).Return(nil)
	mockAPI.On("ConnectWebSocket", mock.Anything, "test-agent", mock.Anything).Return(errors.New("test error"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := agent.Start(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "online", agent.GetStatus().Status)

	// 等待一些goroutine启动
	time.Sleep(100 * time.Millisecond)

	// 验证mock调用
	mockAPI.AssertCalled(t, "Register", mock.Anything, mock.Anything)
	mockLogstash.AssertCalled(t, "Start", mock.Anything)
	mockHeartbeat.AssertCalled(t, "Start", mock.Anything)
	mockMetrics.AssertCalled(t, "Start", mock.Anything)
}

func TestAgent_Stop(t *testing.T) {
	agent, mockAPI, _, mockLogstash, mockHeartbeat, mockMetrics := createTestAgent(t)

	// 先启动Agent
	mockAPI.On("Register", mock.Anything, mock.Anything).Return(nil)
	mockAPI.On("ReportStatus", mock.Anything, mock.Anything).Return(nil)
	mockAPI.On("Close").Return(nil)
	mockLogstash.On("Start", mock.Anything).Return(nil)
	mockLogstash.On("Stop", mock.Anything).Return(nil)
	mockLogstash.On("GetStatus").Return(&LogstashStatus{Version: "8.0.0"}, nil)
	mockHeartbeat.On("Start", mock.Anything).Return(nil)
	mockHeartbeat.On("Stop").Return(nil)
	mockMetrics.On("Start", mock.Anything).Return(nil)
	mockMetrics.On("Stop").Return(nil)
	mockAPI.On("ConnectWebSocket", mock.Anything, "test-agent", mock.Anything).Return(errors.New("test error"))

	ctx := context.Background()
	err := agent.Start(ctx)
	assert.NoError(t, err)

	// 停止Agent
	err = agent.Stop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "offline", agent.GetStatus().Status)

	// 验证清理方法被调用
	mockAPI.AssertCalled(t, "ReportStatus", mock.Anything, mock.Anything)
	mockAPI.AssertCalled(t, "Close")
	mockLogstash.AssertCalled(t, "Stop", mock.Anything)
	mockHeartbeat.AssertCalled(t, "Stop")
	mockMetrics.AssertCalled(t, "Stop")
}

func TestAgent_HandleConfigDeploy(t *testing.T) {
	agent, mockAPI, mockConfigMgr, mockLogstash, _, _ := createTestAgent(t)

	// 准备测试数据
	config := &models.Config{
		ID:      "test-config",
		Name:    "Test Config",
		Content: "input { stdin {} }",
		Version: 1,
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"config_id": "test-config",
		"version":   1,
	})

	// 设置mock期望
	mockAPI.On("GetConfig", mock.Anything, "test-config").Return(config, nil)
	mockConfigMgr.On("SaveConfig", config).Return(nil)
	mockLogstash.On("IsRunning").Return(true)
	mockLogstash.On("Reload", mock.Anything).Return(nil)
	mockAPI.On("ReportConfigApplied", mock.Anything, "test-agent", mock.Anything).Return(nil)

	// 启动Agent上下文
	agent.ctx, agent.cancel = context.WithCancel(context.Background())
	defer agent.cancel()

	err := agent.handleConfigDeploy(json.RawMessage(payload))
	assert.NoError(t, err)

	// 验证配置已添加到状态
	status := agent.GetStatus()
	assert.Len(t, status.AppliedConfigs, 1)
	assert.Equal(t, "test-config", status.AppliedConfigs[0].ConfigID)
	assert.Equal(t, 1, status.AppliedConfigs[0].Version)
}

func TestAgent_HandleConfigDelete(t *testing.T) {
	agent, _, mockConfigMgr, mockLogstash, _, _ := createTestAgent(t)

	// 先添加一个配置
	agent.updateStatus(func(s *models.Agent) {
		s.AppliedConfigs = append(s.AppliedConfigs, models.AppliedConfig{
			ConfigID: "test-config",
			Version:  1,
		})
	})

	payload, _ := json.Marshal(map[string]interface{}{
		"config_id": "test-config",
	})

	// 设置mock期望
	mockConfigMgr.On("DeleteConfig", "test-config").Return(nil)
	mockLogstash.On("IsRunning").Return(true)
	mockLogstash.On("Reload", mock.Anything).Return(nil)

	err := agent.handleConfigDelete(json.RawMessage(payload))
	assert.NoError(t, err)

	// 验证配置已从状态中移除
	status := agent.GetStatus()
	assert.Len(t, status.AppliedConfigs, 0)
}

func TestAgent_HandleReloadRequest(t *testing.T) {
	agent, _, _, mockLogstash, _, _ := createTestAgent(t)

	// 测试Logstash运行中的情况
	mockLogstash.On("IsRunning").Return(true).Once()
	mockLogstash.On("Reload", mock.Anything).Return(nil).Once()

	err := agent.handleReloadRequest()
	assert.NoError(t, err)

	// 测试Logstash未运行的情况
	mockLogstash.On("IsRunning").Return(false).Once()

	err = agent.handleReloadRequest()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Logstash未运行")
}

func TestAgent_HandleStatusRequest(t *testing.T) {
	agent, mockAPI, _, _, _, _ := createTestAgent(t)

	mockAPI.On("ReportStatus", mock.Anything, mock.Anything).Return(nil)

	err := agent.handleStatusRequest()
	assert.NoError(t, err)

	mockAPI.AssertCalled(t, "ReportStatus", mock.Anything, mock.Anything)
}

func TestAgent_HandleMetricsRequest(t *testing.T) {
	agent, mockAPI, _, _, _, mockMetrics := createTestAgent(t)

	metrics := &AgentMetrics{
		CPUUsage:    50.0,
		MemoryUsage: 60.0,
		DiskUsage:   70.0,
	}

	mockMetrics.On("GetMetrics").Return(metrics, nil)
	mockAPI.On("ReportMetrics", mock.Anything, "test-agent", metrics).Return(nil)

	err := agent.handleMetricsRequest()
	assert.NoError(t, err)

	mockMetrics.AssertCalled(t, "GetMetrics")
	mockAPI.AssertCalled(t, "ReportMetrics", mock.Anything, "test-agent", metrics)
}

func TestAgent_HandleMessage(t *testing.T) {
	agent, mockAPI, _, _, _, _ := createTestAgent(t)

	// 设置mock期望
	mockAPI.On("ReportStatus", mock.Anything, mock.Anything).Return(nil)

	// 启动消息处理
	agent.ctx, agent.cancel = context.WithCancel(context.Background())
	defer agent.cancel()

	agent.wg.Add(1)
	go agent.processMessages()

	// 测试有效消息
	payload := []byte(`{"test": "data"}`)
	err := agent.HandleMessage(MsgTypeStatusRequest, payload)
	assert.NoError(t, err)

	// 给消息处理一些时间
	time.Sleep(50 * time.Millisecond)
}

func TestAgent_OnConnect(t *testing.T) {
	agent, mockAPI, _, _, _, _ := createTestAgent(t)

	mockAPI.On("ReportStatus", mock.Anything, mock.Anything).Return(nil)

	err := agent.OnConnect()
	assert.NoError(t, err)
	assert.Equal(t, "online", agent.GetStatus().Status)
}

func TestAgent_OnDisconnect(t *testing.T) {
	agent, _, _, _, _, _ := createTestAgent(t)

	// 测试带错误的断开
	agent.OnDisconnect(errors.New("connection lost"))

	// 测试正常断开
	agent.OnDisconnect(nil)
}

func TestGetLocalIP(t *testing.T) {
	ip, err := getLocalIP()
	// 在测试环境中可能没有有效的非回环IP
	if err != nil {
		assert.Contains(t, err.Error(), "未找到有效的IP地址")
	} else {
		assert.NotEmpty(t, ip)
		assert.NotEqual(t, "127.0.0.1", ip)
	}
}

func TestGetHostname(t *testing.T) {
	hostname, err := getHostname()
	assert.NoError(t, err)
	assert.NotEmpty(t, hostname)
}

// 基准测试
func BenchmarkAgent_UpdateStatus(b *testing.B) {
	agent, _, _, _, _, _ := createTestAgent(&testing.T{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.updateStatus(func(s *models.Agent) {
			s.LastHeartbeat = time.Now()
		})
	}
}

func BenchmarkAgent_GetStatus(b *testing.B) {
	agent, _, _, _, _, _ := createTestAgent(&testing.T{})

	// 添加一些配置
	agent.updateStatus(func(s *models.Agent) {
		for i := 0; i < 10; i++ {
			s.AppliedConfigs = append(s.AppliedConfigs, models.AppliedConfig{
				ConfigID: "config-" + string(rune(i)),
				Version:  i,
			})
		}
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = agent.GetStatus()
	}
}