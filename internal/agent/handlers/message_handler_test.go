package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"logstash-platform/internal/agent/core"
	"logstash-platform/internal/platform/models"
)

// Mock types
type mockAgentCore struct {
	mock.Mock
}

func (m *mockAgentCore) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockAgentCore) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockAgentCore) Register(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockAgentCore) GetStatus() *models.Agent {
	args := m.Called()
	return args.Get(0).(*models.Agent)
}

type mockAPIClient struct {
	mock.Mock
}

func (m *mockAPIClient) Register(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *mockAPIClient) SendHeartbeat(ctx context.Context, agentID string) error {
	args := m.Called(ctx, agentID)
	return args.Error(0)
}

func (m *mockAPIClient) ReportStatus(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *mockAPIClient) GetConfig(ctx context.Context, configID string) (*models.Config, error) {
	args := m.Called(ctx, configID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Config), args.Error(1)
}

func (m *mockAPIClient) ReportConfigApplied(ctx context.Context, agentID string, applied *models.AppliedConfig) error {
	args := m.Called(ctx, agentID, applied)
	return args.Error(0)
}

func (m *mockAPIClient) ConnectWebSocket(ctx context.Context, agentID string, handler core.MessageHandler) error {
	args := m.Called(ctx, agentID, handler)
	return args.Error(0)
}

func (m *mockAPIClient) ReportMetrics(ctx context.Context, agentID string, metrics *core.AgentMetrics) error {
	args := m.Called(ctx, agentID, metrics)
	return args.Error(0)
}

func (m *mockAPIClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

type mockConfigManager struct {
	mock.Mock
}

func (m *mockConfigManager) SaveConfig(config *models.Config) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *mockConfigManager) LoadConfig(configID string) (*models.Config, error) {
	args := m.Called(configID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Config), args.Error(1)
}

func (m *mockConfigManager) DeleteConfig(configID string) error {
	args := m.Called(configID)
	return args.Error(0)
}

func (m *mockConfigManager) ListConfigs() ([]*models.Config, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Config), args.Error(1)
}

func (m *mockConfigManager) GetConfigPath(configID string) string {
	args := m.Called(configID)
	return args.String(0)
}

func (m *mockConfigManager) BackupConfig(configID string) error {
	args := m.Called(configID)
	return args.Error(0)
}

func (m *mockConfigManager) RestoreConfig(configID string) error {
	args := m.Called(configID)
	return args.Error(0)
}

type mockLogstashController struct {
	mock.Mock
}

func (m *mockLogstashController) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockLogstashController) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockLogstashController) Restart(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockLogstashController) Reload(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockLogstashController) IsRunning() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockLogstashController) GetStatus() (*core.LogstashStatus, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.LogstashStatus), args.Error(1)
}

func (m *mockLogstashController) ValidateConfig(configPath string) error {
	args := m.Called(configPath)
	return args.Error(0)
}

type mockMetricsCollector struct {
	mock.Mock
}

func (m *mockMetricsCollector) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockMetricsCollector) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockMetricsCollector) GetMetrics() (*core.AgentMetrics, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.AgentMetrics), args.Error(1)
}

func (m *mockMetricsCollector) SetInterval(interval time.Duration) {
	m.Called(interval)
}

// Tests
func TestMessageHandler_HandleConfigDeploy(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	// Create mocks
	agentCore := new(mockAgentCore)
	apiClient := new(mockAPIClient)
	configManager := new(mockConfigManager)
	logstashCtrl := new(mockLogstashController)
	metricsCollector := new(mockMetricsCollector)

	handler := NewMessageHandler(
		agentCore,
		apiClient,
		configManager,
		logstashCtrl,
		metricsCollector,
		logger,
		"test-agent",
	)

	t.Run("成功部署配置", func(t *testing.T) {
		// Prepare test data
		configID := "test-config"
		version := 1
		payload, _ := json.Marshal(map[string]interface{}{
			"config_id": configID,
			"version":   version,
		})

		testConfig := &models.Config{
			ID:      configID,
			Name:    "Test Config",
			Content: "input { stdin {} }",
			Version: version,
		}

		// Setup expectations
		apiClient.On("GetConfig", mock.Anything, configID).Return(testConfig, nil)
		configManager.On("SaveConfig", testConfig).Return(nil)
		configManager.On("GetConfigPath", configID).Return("/tmp/test-config.conf")
		logstashCtrl.On("ValidateConfig", "/tmp/test-config.conf").Return(nil)
		logstashCtrl.On("IsRunning").Return(true)
		logstashCtrl.On("Reload", mock.Anything).Return(nil)
		apiClient.On("ReportConfigApplied", mock.Anything, "test-agent", mock.Anything).Return(nil)

		// Execute
		err := handler.HandleMessage(core.MsgTypeConfigDeploy, payload)

		// Assert
		assert.NoError(t, err)
		apiClient.AssertExpectations(t)
		configManager.AssertExpectations(t)
		logstashCtrl.AssertExpectations(t)
	})

	t.Run("配置验证失败", func(t *testing.T) {
		// Prepare test data
		configID := "bad-config"
		payload, _ := json.Marshal(map[string]interface{}{
			"config_id": configID,
			"version":   1,
		})

		testConfig := &models.Config{
			ID:      configID,
			Content: "invalid config",
			Version: 1,
		}

		// Setup expectations
		apiClient.On("GetConfig", mock.Anything, configID).Return(testConfig, nil)
		configManager.On("SaveConfig", testConfig).Return(nil)
		configManager.On("GetConfigPath", configID).Return("/tmp/bad-config.conf")
		logstashCtrl.On("ValidateConfig", "/tmp/bad-config.conf").Return(errors.New("invalid syntax"))
		configManager.On("RestoreConfig", configID).Return(nil)

		// Execute
		err := handler.HandleMessage(core.MsgTypeConfigDeploy, payload)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "配置验证失败")
		configManager.AssertCalled(t, "RestoreConfig", configID)
	})
}

func TestMessageHandler_HandleStatusRequest(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	// Create mocks
	agentCore := new(mockAgentCore)
	apiClient := new(mockAPIClient)
	configManager := new(mockConfigManager)
	logstashCtrl := new(mockLogstashController)
	metricsCollector := new(mockMetricsCollector)

	handler := NewMessageHandler(
		agentCore,
		apiClient,
		configManager,
		logstashCtrl,
		metricsCollector,
		logger,
		"test-agent",
	)

	// Prepare test data
	agentStatus := &models.Agent{
		AgentID:  "test-agent",
		Status:   "online",
		Hostname: "test-host",
	}

	logstashStatus := &core.LogstashStatus{
		Running: true,
		Version: "8.0.0",
	}

	// Setup expectations
	agentCore.On("GetStatus").Return(agentStatus)
	logstashCtrl.On("GetStatus").Return(logstashStatus, nil)

	// Execute
	err := handler.HandleMessage(core.MsgTypeStatusRequest, []byte("{}"))

	// Assert
	assert.NoError(t, err)
	agentCore.AssertExpectations(t)
	logstashCtrl.AssertExpectations(t)
}

func TestMessageHandler_HandleMetricsRequest(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	// Create mocks
	agentCore := new(mockAgentCore)
	apiClient := new(mockAPIClient)
	configManager := new(mockConfigManager)
	logstashCtrl := new(mockLogstashController)
	metricsCollector := new(mockMetricsCollector)

	handler := NewMessageHandler(
		agentCore,
		apiClient,
		configManager,
		logstashCtrl,
		metricsCollector,
		logger,
		"test-agent",
	)

	// Prepare test data
	metrics := &core.AgentMetrics{
		Timestamp:   time.Now(),
		CPUUsage:    50.0,
		MemoryUsage: 60.0,
		DiskUsage:   70.0,
	}

	// Setup expectations
	metricsCollector.On("GetMetrics").Return(metrics, nil)

	// Execute
	err := handler.HandleMessage(core.MsgTypeMetricsRequest, []byte("{}"))

	// Assert
	assert.NoError(t, err)
	metricsCollector.AssertExpectations(t)
}

func TestMessageHandler_HandleConfigDelete(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	// Create mocks
	agentCore := new(mockAgentCore)
	apiClient := new(mockAPIClient)
	configManager := new(mockConfigManager)
	logstashCtrl := new(mockLogstashController)
	metricsCollector := new(mockMetricsCollector)

	handler := NewMessageHandler(
		agentCore,
		apiClient,
		configManager,
		logstashCtrl,
		metricsCollector,
		logger,
		"test-agent",
	)

	// Prepare test data
	configID := "delete-config"
	payload, _ := json.Marshal(map[string]interface{}{
		"config_id": configID,
	})

	// Setup expectations
	configManager.On("DeleteConfig", configID).Return(nil)
	logstashCtrl.On("IsRunning").Return(true)
	logstashCtrl.On("Reload", mock.Anything).Return(nil)

	// Execute
	err := handler.HandleMessage(core.MsgTypeConfigDelete, payload)

	// Assert
	assert.NoError(t, err)
	configManager.AssertExpectations(t)
	logstashCtrl.AssertExpectations(t)
}

func TestMessageHandler_OnConnect(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	// Create mocks
	agentCore := new(mockAgentCore)
	apiClient := new(mockAPIClient)
	configManager := new(mockConfigManager)
	logstashCtrl := new(mockLogstashController)
	metricsCollector := new(mockMetricsCollector)

	handler := NewMessageHandler(
		agentCore,
		apiClient,
		configManager,
		logstashCtrl,
		metricsCollector,
		logger,
		"test-agent",
	)

	// Prepare test data
	agentStatus := &models.Agent{
		AgentID: "test-agent",
		Status:  "online",
	}

	// Setup expectations
	agentCore.On("GetStatus").Return(agentStatus)
	apiClient.On("ReportStatus", mock.Anything, agentStatus).Return(nil)

	// Execute
	err := handler.OnConnect()

	// Assert
	assert.NoError(t, err)
	agentCore.AssertExpectations(t)
	apiClient.AssertExpectations(t)
}