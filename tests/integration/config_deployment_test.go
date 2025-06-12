//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"logstash-platform/internal/agent/client"
	"logstash-platform/internal/agent/config"
	"logstash-platform/internal/platform/models"
)

// TestConfigDeployment 测试配置部署流程
func TestConfigDeployment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试服务器
	server := NewTestPlatformServer()
	defer server.Close()

	// 创建测试配置
	testConfig := &models.Config{
		ID:      "test-config-001",
		Name:    "Test Logstash Config",
		Type:    models.ConfigTypeInput,
		Content: `input { stdin {} } output { stdout {} }`,
		Version: 1,
		Enabled: true,
	}
	server.AddConfig(testConfig)

	// 创建临时目录
	configDir := t.TempDir()

	// 创建Agent配置
	cfg := &config.AgentConfig{
		AgentID:        "test-agent-config",
		ServerURL:      server.GetURL(),
		ConfigDir:      configDir,
		RequestTimeout: 5 * time.Second,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	// 创建客户端和配置管理器
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
		IP:       "192.168.1.104",
		Status:   "online",
	}
	err = apiClient.Register(ctx, agent)
	require.NoError(t, err)

	// 获取配置
	fetchedConfig, err := apiClient.GetConfig(ctx, testConfig.ID)
	assert.NoError(t, err)
	assert.NotNil(t, fetchedConfig)
	assert.Equal(t, testConfig.ID, fetchedConfig.ID)
	assert.Equal(t, testConfig.Content, fetchedConfig.Content)

	// 保存配置到本地
	err = configMgr.SaveConfig(fetchedConfig)
	assert.NoError(t, err)

	// 验证配置文件存在
	configPath := filepath.Join(configDir, testConfig.ID+".conf")
	assert.FileExists(t, configPath)

	// 读取并验证内容
	content, err := os.ReadFile(configPath)
	assert.NoError(t, err)
	assert.Equal(t, testConfig.Content, string(content))

	// 上报配置应用结果
	applied := &models.AppliedConfig{
		ConfigID:  testConfig.ID,
		Version:   testConfig.Version,
		AppliedAt: time.Now(),
	}
	err = apiClient.ReportConfigApplied(ctx, cfg.AgentID, applied)
	assert.NoError(t, err)

	// 验证服务器端记录
	registeredAgent := server.GetAgent(cfg.AgentID)
	assert.Len(t, registeredAgent.AppliedConfigs, 1)
	assert.Equal(t, testConfig.ID, registeredAgent.AppliedConfigs[0].ConfigID)
}

// TestConfigUpdate 测试配置更新流程
func TestConfigUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试服务器
	server := NewTestPlatformServer()
	defer server.Close()

	// 创建初始配置
	configV1 := &models.Config{
		ID:      "test-config-update",
		Name:    "Test Config V1",
		Type:    models.ConfigTypeInput,
		Content: `input { stdin {} } output { stdout {} }`,
		Version: 1,
		Enabled: true,
	}
	server.AddConfig(configV1)

	// 创建临时目录
	configDir := t.TempDir()

	// 创建Agent配置
	cfg := &config.AgentConfig{
		AgentID:           "test-agent-update",
		ServerURL:         server.GetURL(),
		ConfigDir:         configDir,
		ConfigBackupCount: 3,
		RequestTimeout:    5 * time.Second,
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
		IP:       "192.168.1.105",
		Status:   "online",
	}
	err = apiClient.Register(ctx, agent)
	require.NoError(t, err)

	// 部署V1版本
	config1, err := apiClient.GetConfig(ctx, configV1.ID)
	require.NoError(t, err)
	err = configMgr.SaveConfig(config1)
	require.NoError(t, err)

	// 更新配置到V2
	configV2 := &models.Config{
		ID:      "test-config-update",
		Name:    "Test Config V2",
		Type:    models.ConfigTypeInput,
		Content: `input { file { path => "/var/log/*.log" } } output { elasticsearch {} }`,
		Version: 2,
		Enabled: true,
	}
	server.AddConfig(configV2)

	// 获取并保存V2版本
	config2, err := apiClient.GetConfig(ctx, configV2.ID)
	require.NoError(t, err)
	err = configMgr.SaveConfig(config2)
	require.NoError(t, err)

	// 验证备份被创建
	backupDir := filepath.Join(configDir, ".backup")
	assert.DirExists(t, backupDir)

	// 验证新配置内容
	configPath := filepath.Join(configDir, configV2.ID+".conf")
	content, err := os.ReadFile(configPath)
	assert.NoError(t, err)
	assert.Equal(t, configV2.Content, string(content))

	// 测试配置恢复
	err = configMgr.RestoreConfig(configV2.ID)
	assert.NoError(t, err)

	// 验证恢复后的内容（应该是V1）
	restoredContent, err := os.ReadFile(configPath)
	assert.NoError(t, err)
	assert.Equal(t, configV1.Content, string(restoredContent))
}

// TestConfigDeletion 测试配置删除流程
func TestConfigDeletion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试服务器
	server := NewTestPlatformServer()
	defer server.Close()

	// 创建测试配置
	testConfig := &models.Config{
		ID:      "test-config-delete",
		Name:    "Test Config to Delete",
		Type:    models.ConfigTypeOutput,
		Content: `output { file { path => "/tmp/test.log" } }`,
		Version: 1,
		Enabled: true,
	}
	server.AddConfig(testConfig)

	// 创建临时目录
	configDir := t.TempDir()

	// 创建Agent配置
	cfg := &config.AgentConfig{
		AgentID:        "test-agent-delete",
		ServerURL:      server.GetURL(),
		ConfigDir:      configDir,
		RequestTimeout: 5 * time.Second,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

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
		IP:       "192.168.1.106",
		Status:   "online",
	}
	err = apiClient.Register(ctx, agent)
	require.NoError(t, err)

	// 部署配置
	config, err := apiClient.GetConfig(ctx, testConfig.ID)
	require.NoError(t, err)
	err = configMgr.SaveConfig(config)
	require.NoError(t, err)

	// 验证配置存在
	configPath := filepath.Join(configDir, testConfig.ID+".conf")
	assert.FileExists(t, configPath)

	// 删除配置
	err = configMgr.DeleteConfig(testConfig.ID)
	assert.NoError(t, err)

	// 验证配置已删除
	assert.NoFileExists(t, configPath)

	// 验证备份存在
	backupDir := filepath.Join(configDir, ".backup")
	assert.DirExists(t, backupDir)
}

// TestMultipleConfigDeployment 测试多配置部署
func TestMultipleConfigDeployment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试服务器
	server := NewTestPlatformServer()
	defer server.Close()

	// 创建多个测试配置
	configs := []*models.Config{
		{
			ID:      "input-config",
			Name:    "Input Config",
			Type:    models.ConfigTypeInput,
			Content: `input { beats { port => 5044 } }`,
			Version: 1,
			Enabled: true,
		},
		{
			ID:      "filter-config",
			Name:    "Filter Config",
			Type:    models.ConfigTypeFilter,
			Content: `filter { grok { match => { "message" => "%{SYSLOGTIMESTAMP:timestamp}" } } }`,
			Version: 1,
			Enabled: true,
		},
		{
			ID:      "output-config",
			Name:    "Output Config",
			Type:    models.ConfigTypeOutput,
			Content: `output { elasticsearch { hosts => ["localhost:9200"] } }`,
			Version: 1,
			Enabled: true,
		},
	}

	// 添加配置到服务器
	for _, cfg := range configs {
		server.AddConfig(cfg)
	}

	// 创建临时目录
	configDir := t.TempDir()

	// 创建Agent配置
	cfg := &config.AgentConfig{
		AgentID:        "test-agent-multi",
		ServerURL:      server.GetURL(),
		ConfigDir:      configDir,
		RequestTimeout: 5 * time.Second,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

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
		IP:       "192.168.1.107",
		Status:   "online",
	}
	err = apiClient.Register(ctx, agent)
	require.NoError(t, err)

	// 部署所有配置
	for _, testConfig := range configs {
		config, err := apiClient.GetConfig(ctx, testConfig.ID)
		require.NoError(t, err)
		err = configMgr.SaveConfig(config)
		require.NoError(t, err)

		// 上报应用结果
		applied := &models.AppliedConfig{
			ConfigID:  testConfig.ID,
			Version:   testConfig.Version,
			AppliedAt: time.Now(),
		}
		err = apiClient.ReportConfigApplied(ctx, cfg.AgentID, applied)
		assert.NoError(t, err)
	}

	// 验证所有配置文件都存在
	for _, testConfig := range configs {
		configPath := filepath.Join(configDir, testConfig.ID+".conf")
		assert.FileExists(t, configPath)
	}

	// 列出所有配置
	configList, err := configMgr.ListConfigs()
	assert.NoError(t, err)
	assert.Len(t, configList, 3)

	// 验证服务器端记录
	registeredAgent := server.GetAgent(cfg.AgentID)
	assert.Len(t, registeredAgent.AppliedConfigs, 3)
}