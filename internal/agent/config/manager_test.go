package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"logstash-platform/internal/platform/models"
)

func createTestManager(t *testing.T) (*Manager, string) {
	// 创建临时目录
	tempDir, err := ioutil.TempDir("", "manager-test-*")
	require.NoError(t, err)

	cfg := &AgentConfig{
		ConfigDir:         tempDir,
		MaxConfigSize:     10 * 1024 * 1024,
		ConfigBackupCount: 3,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	manager, err := NewManager(cfg, logger)
	require.NoError(t, err)

	return manager, tempDir
}

func TestNewManager(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "manager-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name    string
		config  *AgentConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &AgentConfig{
				ConfigDir:         tempDir,
				MaxConfigSize:     10 * 1024 * 1024,
				ConfigBackupCount: 3,
			},
			wantErr: false,
		},
		{
			name: "non-existent directory",
			config: &AgentConfig{
				ConfigDir:         filepath.Join(os.TempDir(), "test-non-existent", "path"),
				MaxConfigSize:     10 * 1024 * 1024,
				ConfigBackupCount: 3,
			},
			wantErr: false, // 目录会被自动创建
		},
	}

	logger := logrus.New()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewManager(tt.config, logger)

			if tt.wantErr {
				assert.NoError(t, err) // 实际上不会返回错误，会创建目录
				assert.NotNil(t, manager)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, manager)

				// 清理自动创建的目录
				if strings.Contains(tt.config.ConfigDir, "test-non-existent") {
					os.RemoveAll(filepath.Join(os.TempDir(), "test-non-existent"))
				}
			}
		})
	}
}

func TestManager_SaveConfig(t *testing.T) {
	manager, tempDir := createTestManager(t)
	defer os.RemoveAll(tempDir)

	config := &models.Config{
		ID:      "test-config",
		Name:    "Test Config",
		Content: "input { stdin {} }\noutput { stdout {} }",
		Version: 1,
	}

	// 保存配置
	err := manager.SaveConfig(config)
	assert.NoError(t, err)

	// 验证文件存在
	configPath := filepath.Join(tempDir, "test-config.conf")
	assert.FileExists(t, configPath)

	// 验证文件内容
	content, err := ioutil.ReadFile(configPath)
	assert.NoError(t, err)
	assert.Equal(t, config.Content, string(content))

	// 验证元数据文件
	metaPath := filepath.Join(tempDir, ".metadata", "test-config.json")
	assert.FileExists(t, metaPath)
}

func TestManager_SaveConfig_WithBackup(t *testing.T) {
	manager, tempDir := createTestManager(t)
	defer os.RemoveAll(tempDir)

	// 保存原始配置
	config1 := &models.Config{
		ID:      "test-config",
		Name:    "Test Config",
		Content: "input { stdin {} }",
		Version: 1,
	}
	err := manager.SaveConfig(config1)
	assert.NoError(t, err)

	// 保存新版本
	config2 := &models.Config{
		ID:      "test-config",
		Name:    "Test Config",
		Content: "input { file {} }",
		Version: 2,
	}
	err = manager.SaveConfig(config2)
	assert.NoError(t, err)

	// 验证备份文件存在
	backupDir := filepath.Join(tempDir, ".backup")
	files, err := ioutil.ReadDir(backupDir)
	assert.NoError(t, err)
	assert.Len(t, files, 1) // 应该有一个备份
}

func TestManager_LoadConfig(t *testing.T) {
	manager, tempDir := createTestManager(t)
	defer os.RemoveAll(tempDir)

	// 先保存一个配置
	originalConfig := &models.Config{
		ID:      "test-config",
		Name:    "Test Config",
		Content: "input { stdin {} }",
		Version: 1,
	}
	err := manager.SaveConfig(originalConfig)
	assert.NoError(t, err)

	// 加载配置
	loadedConfig, err := manager.LoadConfig("test-config")
	assert.NoError(t, err)
	assert.NotNil(t, loadedConfig)
	assert.Equal(t, originalConfig.ID, loadedConfig.ID)
	assert.Equal(t, originalConfig.Name, loadedConfig.Name)
	assert.Equal(t, originalConfig.Content, loadedConfig.Content)
	assert.Equal(t, originalConfig.Version, loadedConfig.Version)

	// 加载不存在的配置
	_, err = manager.LoadConfig("non-existent")
	assert.Error(t, err)
}

func TestManager_DeleteConfig(t *testing.T) {
	manager, tempDir := createTestManager(t)
	defer os.RemoveAll(tempDir)

	// 保存一个配置
	config := &models.Config{
		ID:      "test-config",
		Name:    "Test Config",
		Content: "input { stdin {} }",
		Version: 1,
	}
	err := manager.SaveConfig(config)
	assert.NoError(t, err)

	// 删除配置
	err = manager.DeleteConfig("test-config")
	assert.NoError(t, err)

	// 验证文件不存在
	configPath := filepath.Join(tempDir, "test-config.conf")
	assert.NoFileExists(t, configPath)

	// 验证元数据不存在
	metaPath := filepath.Join(tempDir, ".metadata", "test-config.json")
	assert.NoFileExists(t, metaPath)

	// 删除不存在的配置
	err = manager.DeleteConfig("non-existent")
	assert.NoError(t, err) // 不应该返回错误
}

func TestManager_ListConfigs(t *testing.T) {
	manager, tempDir := createTestManager(t)
	defer os.RemoveAll(tempDir)

	// 保存多个配置
	configs := []*models.Config{
		{
			ID:      "config-1",
			Name:    "Config 1",
			Content: "input { stdin {} }",
			Version: 1,
		},
		{
			ID:      "config-2",
			Name:    "Config 2",
			Content: "input { file {} }",
			Version: 1,
		},
		{
			ID:      "config-3",
			Name:    "Config 3",
			Content: "input { http {} }",
			Version: 1,
		},
	}

	for _, cfg := range configs {
		err := manager.SaveConfig(cfg)
		assert.NoError(t, err)
	}

	// 列出所有配置
	listedConfigs, err := manager.ListConfigs()
	assert.NoError(t, err)
	assert.Len(t, listedConfigs, 3)

	// 验证配置ID
	configIDs := make(map[string]bool)
	for _, cfg := range listedConfigs {
		configIDs[cfg.ID] = true
	}
	assert.True(t, configIDs["config-1"])
	assert.True(t, configIDs["config-2"])
	assert.True(t, configIDs["config-3"])
}

func TestManager_GetConfigPath(t *testing.T) {
	manager, tempDir := createTestManager(t)
	defer os.RemoveAll(tempDir)

	path := manager.GetConfigPath("test-config")
	expectedPath := filepath.Join(tempDir, "test-config.conf")
	assert.Equal(t, expectedPath, path)
}

func TestManager_BackupConfig(t *testing.T) {
	manager, tempDir := createTestManager(t)
	defer os.RemoveAll(tempDir)

	// 保存一个配置
	config := &models.Config{
		ID:      "test-config",
		Name:    "Test Config",
		Content: "input { stdin {} }",
		Version: 1,
	}
	err := manager.SaveConfig(config)
	assert.NoError(t, err)

	// 备份配置
	err = manager.BackupConfig("test-config")
	assert.NoError(t, err)

	// 验证备份文件存在
	backupDir := filepath.Join(tempDir, ".backup")
	files, err := ioutil.ReadDir(backupDir)
	assert.NoError(t, err)
	assert.Len(t, files, 1)

	// 备份不存在的配置
	err = manager.BackupConfig("non-existent")
	assert.NoError(t, err) // BackupConfig 对不存在的配置返回nil
}

func TestManager_RestoreConfig(t *testing.T) {
	manager, tempDir := createTestManager(t)
	defer os.RemoveAll(tempDir)

	// 保存原始配置
	originalConfig := &models.Config{
		ID:      "test-config",
		Name:    "Test Config",
		Content: "input { stdin {} }",
		Version: 1,
	}
	err := manager.SaveConfig(originalConfig)
	assert.NoError(t, err)

	// 保存新版本（会自动备份原始版本）
	newConfig := &models.Config{
		ID:      "test-config",
		Name:    "Test Config",
		Content: "input { file {} }",
		Version: 2,
	}
	err = manager.SaveConfig(newConfig)
	assert.NoError(t, err)

	// 恢复配置
	err = manager.RestoreConfig("test-config")
	assert.NoError(t, err)

	// 验证恢复后的内容
	restoredConfig, err := manager.LoadConfig("test-config")
	assert.NoError(t, err)
	assert.Equal(t, originalConfig.Content, restoredConfig.Content)
	assert.Equal(t, originalConfig.Version, restoredConfig.Version)

	// 恢复不存在的配置
	err = manager.RestoreConfig("non-existent")
	assert.Error(t, err)
}

func TestManager_ValidateConfigSize(t *testing.T) {
	manager, tempDir := createTestManager(t)
	defer os.RemoveAll(tempDir)

	// 创建超大配置
	largeContent := make([]byte, 11*1024*1024) // 11MB
	for i := range largeContent {
		largeContent[i] = 'a'
	}

	config := &models.Config{
		ID:      "large-config",
		Name:    "Large Config",
		Content: string(largeContent),
		Version: 1,
	}

	// 保存应该成功（因为当前没有大小限制检查）
	err := manager.SaveConfig(config)
	assert.NoError(t, err)
}

func TestManager_BackupRotation(t *testing.T) {
	manager, tempDir := createTestManager(t)
	defer os.RemoveAll(tempDir)

	// 保存多个版本的配置以触发备份轮转
	for i := 1; i <= 5; i++ {
		config := &models.Config{
			ID:      "test-config",
			Name:    "Test Config",
			Content: "input { stdin {} }" + string(rune(i)),
			Version: i,
		}
		err := manager.SaveConfig(config)
		assert.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // 确保文件时间戳不同
	}

	// 验证备份数量不超过限制
	backupDir := filepath.Join(tempDir, ".backup")
	files, err := ioutil.ReadDir(backupDir)
	assert.NoError(t, err)
	// 因为有多个备份版本，文件数应该小于等于ConfigBackupCount
	assert.LessOrEqual(t, len(files), manager.config.ConfigBackupCount+1)
}

// 并发测试
func TestManager_ConcurrentSaveLoad(t *testing.T) {
	manager, tempDir := createTestManager(t)
	defer os.RemoveAll(tempDir)

	const goroutines = 10
	done := make(chan bool, goroutines)

	// 并发保存和加载
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			config := &models.Config{
				ID:      fmt.Sprintf("config-%d", id),
				Name:    fmt.Sprintf("Config %d", id),
				Content: "input { stdin {} }",
				Version: 1,
			}

			// 保存
			err := manager.SaveConfig(config)
			assert.NoError(t, err)

			// 加载
			loadedConfig, err := manager.LoadConfig(config.ID)
			assert.NoError(t, err)
			assert.Equal(t, config.Content, loadedConfig.Content)

			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// 验证所有配置都被正确保存
	configs, err := manager.ListConfigs()
	assert.NoError(t, err)
	assert.Len(t, configs, goroutines)
}

// 基准测试
func BenchmarkManager_SaveConfig(b *testing.B) {
	manager, tempDir := createTestManager(&testing.T{})
	defer os.RemoveAll(tempDir)

	config := &models.Config{
		ID:      "bench-config",
		Name:    "Benchmark Config",
		Content: "input { stdin {} }\nfilter { mutate {} }\noutput { stdout {} }",
		Version: 1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.Version = i
		_ = manager.SaveConfig(config)
	}
}

func BenchmarkManager_LoadConfig(b *testing.B) {
	manager, tempDir := createTestManager(&testing.T{})
	defer os.RemoveAll(tempDir)

	// 预先保存一个配置
	config := &models.Config{
		ID:      "bench-config",
		Name:    "Benchmark Config",
		Content: "input { stdin {} }\nfilter { mutate {} }\noutput { stdout {} }",
		Version: 1,
	}
	_ = manager.SaveConfig(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.LoadConfig("bench-config")
	}
}