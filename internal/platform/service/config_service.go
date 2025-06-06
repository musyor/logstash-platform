package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"logstash-platform/internal/platform/models"
	"logstash-platform/internal/platform/repository"
)

// ConfigService 配置服务接口
type ConfigService interface {
	CreateConfig(ctx context.Context, req *models.CreateConfigRequest, userID string) (*models.Config, error)
	UpdateConfig(ctx context.Context, id string, req *models.UpdateConfigRequest, userID string) (*models.Config, error)
	DeleteConfig(ctx context.Context, id string) error
	GetConfig(ctx context.Context, id string) (*models.Config, error)
	ListConfigs(ctx context.Context, req *models.ConfigListRequest) (*models.ConfigListResponse, error)
	GetConfigHistory(ctx context.Context, configID string) ([]*models.ConfigHistory, error)
	RollbackConfig(ctx context.Context, configID string, version int, userID string) (*models.Config, error)
}

// configService 配置服务实现
type configService struct {
	configRepo repository.ConfigRepository
	logger     *logrus.Logger
}

// NewConfigService 创建配置服务
func NewConfigService(configRepo repository.ConfigRepository, logger *logrus.Logger) ConfigService {
	return &configService{
		configRepo: configRepo,
		logger:     logger,
	}
}

// CreateConfig 创建配置
func (s *configService) CreateConfig(ctx context.Context, req *models.CreateConfigRequest, userID string) (*models.Config, error) {
	// 验证配置内容
	if err := s.validateConfigContent(req.Type, req.Content); err != nil {
		return nil, fmt.Errorf("配置内容验证失败: %w", err)
	}

	// 创建配置对象
	config := &models.Config{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Content:     req.Content,
		Tags:        req.Tags,
		CreatedBy:   userID,
		UpdatedBy:   userID,
	}

	// 保存到仓库
	if err := s.configRepo.Create(ctx, config); err != nil {
		return nil, err
	}

	s.logger.WithFields(logrus.Fields{
		"config_id": config.ID,
		"name":      config.Name,
		"type":      config.Type,
		"user_id":   userID,
	}).Info("创建配置成功")

	return config, nil
}

// UpdateConfig 更新配置
func (s *configService) UpdateConfig(ctx context.Context, id string, req *models.UpdateConfigRequest, userID string) (*models.Config, error) {
	// 获取现有配置
	config, err := s.configRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("配置不存在: %w", err)
	}

	// 验证配置内容
	if err := s.validateConfigContent(req.Type, req.Content); err != nil {
		return nil, fmt.Errorf("配置内容验证失败: %w", err)
	}

	// 更新字段
	config.Name = req.Name
	config.Description = req.Description
	config.Type = req.Type
	config.Content = req.Content
	config.Tags = req.Tags
	config.UpdatedBy = userID

	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}

	// 保存更新
	if err := s.configRepo.Update(ctx, config); err != nil {
		return nil, err
	}

	s.logger.WithFields(logrus.Fields{
		"config_id": config.ID,
		"name":      config.Name,
		"version":   config.Version,
		"user_id":   userID,
	}).Info("更新配置成功")

	return config, nil
}

// DeleteConfig 删除配置
func (s *configService) DeleteConfig(ctx context.Context, id string) error {
	if err := s.configRepo.Delete(ctx, id); err != nil {
		return err
	}

	s.logger.WithField("config_id", id).Info("删除配置成功")
	return nil
}

// GetConfig 获取配置
func (s *configService) GetConfig(ctx context.Context, id string) (*models.Config, error) {
	return s.configRepo.GetByID(ctx, id)
}

// ListConfigs 获取配置列表
func (s *configService) ListConfigs(ctx context.Context, req *models.ConfigListRequest) (*models.ConfigListResponse, error) {
	// 参数验证
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 10
	}

	return s.configRepo.List(ctx, req)
}

// GetConfigHistory 获取配置历史
func (s *configService) GetConfigHistory(ctx context.Context, configID string) ([]*models.ConfigHistory, error) {
	// 验证配置是否存在
	if _, err := s.configRepo.GetByID(ctx, configID); err != nil {
		return nil, fmt.Errorf("配置不存在: %w", err)
	}

	return s.configRepo.GetHistory(ctx, configID)
}

// RollbackConfig 回滚配置
func (s *configService) RollbackConfig(ctx context.Context, configID string, version int, userID string) (*models.Config, error) {
	// 获取指定版本的历史记录
	history, err := s.configRepo.GetHistory(ctx, configID)
	if err != nil {
		return nil, err
	}

	// 查找指定版本
	var targetHistory *models.ConfigHistory
	for _, h := range history {
		if h.Version == version {
			targetHistory = h
			break
		}
	}

	if targetHistory == nil {
		return nil, fmt.Errorf("未找到版本 %d 的历史记录", version)
	}

	// 获取当前配置
	config, err := s.configRepo.GetByID(ctx, configID)
	if err != nil {
		return nil, err
	}

	// 更新配置内容
	config.Content = targetHistory.Content
	config.UpdatedBy = userID

	// 保存更新
	if err := s.configRepo.Update(ctx, config); err != nil {
		return nil, err
	}

	s.logger.WithFields(logrus.Fields{
		"config_id":     config.ID,
		"rollback_from": config.Version - 1,
		"rollback_to":   version,
		"user_id":       userID,
	}).Info("回滚配置成功")

	return config, nil
}

// validateConfigContent 验证配置内容
func (s *configService) validateConfigContent(configType models.ConfigType, content string) error {
	// TODO: 实现配置内容验证逻辑
	// 1. 检查语法是否正确
	// 2. 验证必要的字段
	// 3. 检查配置类型是否匹配

	if content == "" {
		return fmt.Errorf("配置内容不能为空")
	}

	// 基本的配置类型验证
	switch configType {
	case models.ConfigTypeInput:
		if !containsKeyword(content, "input") {
			return fmt.Errorf("输入配置必须包含 'input' 关键字")
		}
	case models.ConfigTypeFilter:
		if !containsKeyword(content, "filter") {
			return fmt.Errorf("过滤配置必须包含 'filter' 关键字")
		}
	case models.ConfigTypeOutput:
		if !containsKeyword(content, "output") {
			return fmt.Errorf("输出配置必须包含 'output' 关键字")
		}
	}

	return nil
}

// containsKeyword 检查内容是否包含关键字
func containsKeyword(content, keyword string) bool {
	// 简单的关键字检查，实际应该使用更复杂的解析
	return strings.Contains(content, keyword)
}