package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"logstash-platform/internal/platform/models"
)

// Manager 配置管理器实现
type Manager struct {
	config     *AgentConfig
	logger     *logrus.Logger
	
	// 配置缓存
	configs    map[string]*models.Config
	configsMux sync.RWMutex
	
	// 元数据文件路径
	metadataFile string
}

// ConfigMetadata 配置元数据
type ConfigMetadata struct {
	ConfigID    string    `json:"config_id"`
	Version     int       `json:"version"`
	FilePath    string    `json:"file_path"`
	BackupPaths []string  `json:"backup_paths"`
	AppliedAt   time.Time `json:"applied_at"`
	Hash        string    `json:"hash"`
}

// NewManager 创建配置管理器
func NewManager(cfg *AgentConfig, logger *logrus.Logger) (*Manager, error) {
	// 确保配置目录存在
	if err := os.MkdirAll(cfg.ConfigDir, 0755); err != nil {
		return nil, fmt.Errorf("创建配置目录失败: %w", err)
	}
	
	// 创建必要的子目录
	metadataDir := filepath.Join(cfg.ConfigDir, ".metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建元数据目录失败: %w", err)
	}
	
	backupDir := filepath.Join(cfg.ConfigDir, ".backup")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("创建备份目录失败: %w", err)
	}
	
	manager := &Manager{
		config:       cfg,
		logger:       logger,
		configs:      make(map[string]*models.Config),
		metadataFile: filepath.Join(cfg.ConfigDir, ".metadata.json"),
	}
	
	// 加载现有配置元数据
	if err := manager.loadMetadata(); err != nil {
		logger.WithError(err).Warn("加载配置元数据失败")
	}
	
	return manager, nil
}

// SaveConfig 保存配置到本地
func (m *Manager) SaveConfig(config *models.Config) error {
	m.logger.WithField("config_id", config.ID).Info("保存配置")
	
	// 获取配置文件路径
	configPath := m.GetConfigPath(config.ID)
	
	// 备份现有配置（如果存在）
	if _, err := os.Stat(configPath); err == nil {
		if err := m.BackupConfig(config.ID); err != nil {
			m.logger.WithError(err).Warn("备份配置失败")
		}
	}
	
	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	
	// 写入配置文件
	if err := ioutil.WriteFile(configPath, []byte(config.Content), 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	
	// 更新缓存
	m.configsMux.Lock()
	m.configs[config.ID] = config
	m.configsMux.Unlock()
	
	// 保存元数据
	// 先尝试加载现有元数据以保留备份路径
	existingMetadata, _ := m.loadConfigMetadata(config.ID)
	
	metadata := &ConfigMetadata{
		ConfigID:  config.ID,
		Version:   config.Version,
		FilePath:  configPath,
		AppliedAt: time.Now(),
		Hash:      m.calculateHash(config.Content),
	}
	
	// 保留现有的备份路径
	if existingMetadata != nil {
		metadata.BackupPaths = existingMetadata.BackupPaths
	}
	
	if err := m.saveConfigMetadata(config.ID, metadata); err != nil {
		m.logger.WithError(err).Warn("保存配置元数据失败")
	}
	
	m.logger.WithFields(logrus.Fields{
		"config_id": config.ID,
		"version":   config.Version,
		"path":      configPath,
	}).Info("配置保存成功")
	
	return nil
}

// LoadConfig 加载本地配置
func (m *Manager) LoadConfig(configID string) (*models.Config, error) {
	// 先从缓存查找
	m.configsMux.RLock()
	if config, ok := m.configs[configID]; ok {
		m.configsMux.RUnlock()
		return config, nil
	}
	m.configsMux.RUnlock()
	
	// 从文件加载
	configPath := m.GetConfigPath(configID)
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	
	// 加载元数据
	metadata, err := m.loadConfigMetadata(configID)
	if err != nil {
		m.logger.WithError(err).Warn("加载配置元数据失败")
		// 创建基本配置
		return &models.Config{
			ID:      configID,
			Content: string(content),
		}, nil
	}
	
	// 创建配置对象
	config := &models.Config{
		ID:      configID,
		Version: metadata.Version,
		Content: string(content),
	}
	
	// 更新缓存
	m.configsMux.Lock()
	m.configs[configID] = config
	m.configsMux.Unlock()
	
	return config, nil
}

// DeleteConfig 删除本地配置
func (m *Manager) DeleteConfig(configID string) error {
	m.logger.WithField("config_id", configID).Info("删除配置")
	
	// 获取配置文件路径
	configPath := m.GetConfigPath(configID)
	
	// 备份配置
	if err := m.BackupConfig(configID); err != nil {
		m.logger.WithError(err).Warn("备份配置失败")
	}
	
	// 删除配置文件
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除配置文件失败: %w", err)
	}
	
	// 从缓存删除
	m.configsMux.Lock()
	delete(m.configs, configID)
	m.configsMux.Unlock()
	
	// 删除元数据
	if err := m.deleteConfigMetadata(configID); err != nil {
		m.logger.WithError(err).Warn("删除配置元数据失败")
	}
	
	return nil
}

// ListConfigs 列出所有本地配置
func (m *Manager) ListConfigs() ([]*models.Config, error) {
	// 从元数据目录读取所有配置
	metadataDir := filepath.Join(m.config.ConfigDir, ".metadata")
	files, err := ioutil.ReadDir(metadataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*models.Config{}, nil
		}
		return nil, fmt.Errorf("读取元数据目录失败: %w", err)
	}
	
	var configs []*models.Config
	for _, file := range files {
		// 跳过非JSON文件
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		
		// 从文件名提取配置ID
		configID := strings.TrimSuffix(file.Name(), ".json")
		
		// 加载配置
		config, err := m.LoadConfig(configID)
		if err != nil {
			m.logger.WithError(err).WithField("config_id", configID).Warn("加载配置失败")
			continue
		}
		
		configs = append(configs, config)
	}
	
	return configs, nil
}

// GetConfigPath 获取配置文件路径
func (m *Manager) GetConfigPath(configID string) string {
	return m.config.GetLogstashConfigPath(configID)
}

// BackupConfig 备份配置
func (m *Manager) BackupConfig(configID string) error {
	configPath := m.GetConfigPath(configID)
	
	// 检查配置文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil // 配置不存在，无需备份
	}
	
	// 读取当前配置
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	
	// 获取当前版本号
	metadata, err := m.loadConfigMetadata(configID)
	version := 1
	if err == nil && metadata != nil {
		version = metadata.Version
	} else {
		// 如果元数据不存在，创建新的
		metadata = &ConfigMetadata{
			ConfigID:    configID,
			Version:     version,
			FilePath:    configPath,
			AppliedAt:   time.Now(),
			BackupPaths: []string{},
		}
	}
	
	// 生成备份文件路径
	backupPath := m.config.GetConfigBackupPath(configID, version)
	
	// 写入备份文件
	if err := ioutil.WriteFile(backupPath, content, 0644); err != nil {
		return fmt.Errorf("写入备份文件失败: %w", err)
	}
	
	// 更新元数据
	metadata.BackupPaths = append(metadata.BackupPaths, backupPath)
	// 限制备份数量
	if len(metadata.BackupPaths) > m.config.ConfigBackupCount {
		// 删除最旧的备份
		oldBackup := metadata.BackupPaths[0]
		if err := os.Remove(oldBackup); err != nil {
			m.logger.WithError(err).Warn("删除旧备份失败")
		}
		metadata.BackupPaths = metadata.BackupPaths[1:]
	}
	m.saveConfigMetadata(configID, metadata)
	
	m.logger.WithFields(logrus.Fields{
		"config_id": configID,
		"backup":    backupPath,
	}).Info("配置备份成功")
	
	return nil
}

// RestoreConfig 恢复配置
func (m *Manager) RestoreConfig(configID string) error {
	// 加载元数据
	metadata, err := m.loadConfigMetadata(configID)
	if err != nil {
		return fmt.Errorf("加载配置元数据失败: %w", err)
	}
	
	if len(metadata.BackupPaths) == 0 {
		return fmt.Errorf("没有可用的备份")
	}
	
	// 获取最新的备份
	backupPath := metadata.BackupPaths[len(metadata.BackupPaths)-1]
	
	// 读取备份内容
	content, err := ioutil.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("读取备份文件失败: %w", err)
	}
	
	// 恢复到原配置文件
	configPath := m.GetConfigPath(configID)
	if err := ioutil.WriteFile(configPath, content, 0644); err != nil {
		return fmt.Errorf("恢复配置文件失败: %w", err)
	}
	
	// 从备份路径提取版本号
	// 格式: xxx.conf.backup.{version}
	parts := strings.Split(backupPath, ".")
	if len(parts) > 0 {
		if versionStr := parts[len(parts)-1]; versionStr != "" {
			if version, err := strconv.Atoi(versionStr); err == nil {
				// 更新元数据中的版本号
				metadata.Version = version
				m.saveConfigMetadata(configID, metadata)
			}
		}
	}
	
	// 清除缓存，强制重新加载
	m.configsMux.Lock()
	delete(m.configs, configID)
	m.configsMux.Unlock()
	
	m.logger.WithFields(logrus.Fields{
		"config_id": configID,
		"backup":    backupPath,
	}).Info("配置恢复成功")
	
	return nil
}

// 辅助方法

// loadMetadata 加载所有配置元数据
func (m *Manager) loadMetadata() error {
	data, err := ioutil.ReadFile(m.metadataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	
	var metadata map[string]*ConfigMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return err
	}
	
	// 验证配置文件是否存在
	for configID, meta := range metadata {
		if _, err := os.Stat(meta.FilePath); os.IsNotExist(err) {
			m.logger.WithField("config_id", configID).Warn("配置文件不存在，删除元数据")
			delete(metadata, configID)
		}
	}
	
	return nil
}

// saveConfigMetadata 保存单个配置的元数据
func (m *Manager) saveConfigMetadata(configID string, metadata *ConfigMetadata) error {
	// 保存到单独的元数据文件
	metadataPath := filepath.Join(m.config.ConfigDir, ".metadata", configID+".json")
	
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	
	// 同时保存到总的元数据文件（向后兼容）
	allMetadata := make(map[string]*ConfigMetadata)
	if data, err := ioutil.ReadFile(m.metadataFile); err == nil {
		json.Unmarshal(data, &allMetadata)
	}
	allMetadata[configID] = metadata
	
	if allData, err := json.MarshalIndent(allMetadata, "", "  "); err == nil {
		ioutil.WriteFile(m.metadataFile, allData, 0644)
	}
	
	return ioutil.WriteFile(metadataPath, data, 0644)
}

// loadConfigMetadata 加载单个配置的元数据
func (m *Manager) loadConfigMetadata(configID string) (*ConfigMetadata, error) {
	// 优先从单独的元数据文件读取
	metadataPath := filepath.Join(m.config.ConfigDir, ".metadata", configID+".json")
	if data, err := ioutil.ReadFile(metadataPath); err == nil {
		var metadata ConfigMetadata
		if err := json.Unmarshal(data, &metadata); err == nil {
			return &metadata, nil
		}
	}
	
	// 加载所有元数据
	allMetadata := make(map[string]*ConfigMetadata)
	if data, err := ioutil.ReadFile(m.metadataFile); err == nil {
		if err := json.Unmarshal(data, &allMetadata); err != nil {
			return nil, err
		}
	}
	
	metadata, ok := allMetadata[configID]
	if !ok {
		return nil, fmt.Errorf("配置元数据不存在")
	}
	
	return metadata, nil
}

// deleteConfigMetadata 删除配置元数据
func (m *Manager) deleteConfigMetadata(configID string) error {
	// 删除单独的元数据文件
	metadataPath := filepath.Join(m.config.ConfigDir, ".metadata", configID+".json")
	if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	
	// 同时更新总的元数据文件（向后兼容）
	allMetadata := make(map[string]*ConfigMetadata)
	if data, err := ioutil.ReadFile(m.metadataFile); err == nil {
		json.Unmarshal(data, &allMetadata)
	}
	
	// 删除元数据
	delete(allMetadata, configID)
	
	// 保存到文件
	if data, err := json.MarshalIndent(allMetadata, "", "  "); err == nil {
		ioutil.WriteFile(m.metadataFile, data, 0644)
	}
	
	return nil
}

// calculateHash 计算配置内容哈希
func (m *Manager) calculateHash(content string) string {
	// TODO: 实现哈希计算
	return ""
}

// isConfigFile 检查是否为配置文件
func isConfigFile(filename string) bool {
	return filepath.Ext(filename) == ".conf"
}

// extractConfigID 从文件名提取配置ID
func extractConfigID(filename string) string {
	// 移除扩展名
	name := filename[:len(filename)-len(filepath.Ext(filename))]
	// 移除备份后缀
	if idx := len(name) - 1; idx > 0 && name[idx] >= '0' && name[idx] <= '9' {
		// 可能是备份文件
		return ""
	}
	return name
}