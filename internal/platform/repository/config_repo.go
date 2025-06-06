package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"logstash-platform/internal/platform/models"
	"logstash-platform/pkg/elasticsearch"
)

// ConfigRepository 配置仓库接口
type ConfigRepository interface {
	Create(ctx context.Context, config *models.Config) error
	Update(ctx context.Context, config *models.Config) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*models.Config, error)
	List(ctx context.Context, req *models.ConfigListRequest) (*models.ConfigListResponse, error)
	SaveHistory(ctx context.Context, history *models.ConfigHistory) error
	GetHistory(ctx context.Context, configID string) ([]*models.ConfigHistory, error)
}

// configRepository 配置仓库实现
type configRepository struct {
	esClient *elasticsearch.Client
	logger   *logrus.Logger
}

// NewConfigRepository 创建配置仓库
func NewConfigRepository(esClient *elasticsearch.Client, logger *logrus.Logger) ConfigRepository {
	return &configRepository{
		esClient: esClient,
		logger:   logger,
	}
}

// Create 创建配置
func (r *configRepository) Create(ctx context.Context, config *models.Config) error {
	// 生成ID
	if config.ID == "" {
		config.ID = uuid.New().String()
	}

	// 设置时间戳
	now := time.Now()
	config.CreatedAt = now
	config.UpdatedAt = now
	config.Version = 1
	config.Enabled = true
	config.TestStatus = models.TestStatusUntested

	// 索引文档
	if err := r.esClient.Index(ctx, "logstash_configs", config.ID, config); err != nil {
		return fmt.Errorf("创建配置失败: %w", err)
	}

	// 保存历史记录
	history := &models.ConfigHistory{
		ID:         uuid.New().String(),
		ConfigID:   config.ID,
		Version:    config.Version,
		Content:    config.Content,
		ChangeType: "create",
		ChangeLog:  fmt.Sprintf("创建配置: %s", config.Name),
		ModifiedBy: config.CreatedBy,
		ModifiedAt: now,
	}

	if err := r.SaveHistory(ctx, history); err != nil {
		r.logger.Errorf("保存配置历史失败: %v", err)
	}

	return nil
}

// Update 更新配置
func (r *configRepository) Update(ctx context.Context, config *models.Config) error {
	// 获取现有配置
	existing, err := r.GetByID(ctx, config.ID)
	if err != nil {
		return fmt.Errorf("获取现有配置失败: %w", err)
	}

	// 更新版本和时间戳
	config.Version = existing.Version + 1
	config.UpdatedAt = time.Now()
	config.CreatedAt = existing.CreatedAt
	config.CreatedBy = existing.CreatedBy

	// 如果内容变更，重置测试状态
	if config.Content != existing.Content {
		config.TestStatus = models.TestStatusUntested
	}

	// 更新文档
	if err := r.esClient.Index(ctx, "logstash_configs", config.ID, config); err != nil {
		return fmt.Errorf("更新配置失败: %w", err)
	}

	// 保存历史记录
	history := &models.ConfigHistory{
		ID:         uuid.New().String(),
		ConfigID:   config.ID,
		Version:    config.Version,
		Content:    config.Content,
		ChangeType: "update",
		ChangeLog:  fmt.Sprintf("更新配置: %s (版本 %d -> %d)", config.Name, existing.Version, config.Version),
		ModifiedBy: config.UpdatedBy,
		ModifiedAt: config.UpdatedAt,
	}

	if err := r.SaveHistory(ctx, history); err != nil {
		r.logger.Errorf("保存配置历史失败: %v", err)
	}

	return nil
}

// Delete 删除配置
func (r *configRepository) Delete(ctx context.Context, id string) error {
	// 获取配置信息用于历史记录
	config, err := r.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("获取配置失败: %w", err)
	}

	// 删除文档
	if err := r.esClient.Delete(ctx, "logstash_configs", id); err != nil {
		return fmt.Errorf("删除配置失败: %w", err)
	}

	// 保存删除历史
	history := &models.ConfigHistory{
		ID:         uuid.New().String(),
		ConfigID:   id,
		Version:    config.Version,
		Content:    config.Content,
		ChangeType: "delete",
		ChangeLog:  fmt.Sprintf("删除配置: %s", config.Name),
		ModifiedBy: config.UpdatedBy,
		ModifiedAt: time.Now(),
	}

	if err := r.SaveHistory(ctx, history); err != nil {
		r.logger.Errorf("保存配置历史失败: %v", err)
	}

	return nil
}

// GetByID 根据ID获取配置
func (r *configRepository) GetByID(ctx context.Context, id string) (*models.Config, error) {
	var config models.Config
	if err := r.esClient.Get(ctx, "logstash_configs", id, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// List 获取配置列表
func (r *configRepository) List(ctx context.Context, req *models.ConfigListRequest) (*models.ConfigListResponse, error) {
	// 构建查询
	query := map[string]interface{}{
		"from": (req.Page - 1) * req.PageSize,
		"size": req.PageSize,
		"sort": []map[string]interface{}{
			{"updated_at": map[string]string{"order": "desc"}},
		},
	}

	// 构建过滤条件
	var must []map[string]interface{}

	if req.Type != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{"type": req.Type},
		})
	}

	if req.Enabled != nil {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{"enabled": *req.Enabled},
		})
	}

	if len(req.Tags) > 0 {
		must = append(must, map[string]interface{}{
			"terms": map[string]interface{}{"tags": req.Tags},
		})
	}

	if len(must) > 0 {
		query["query"] = map[string]interface{}{
			"bool": map[string]interface{}{
				"must": must,
			},
		}
	}

	// 执行搜索
	var result struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source models.Config `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := r.esClient.Search(ctx, "logstash_configs", query, &result); err != nil {
		return nil, fmt.Errorf("搜索配置失败: %w", err)
	}

	// 构建响应
	response := &models.ConfigListResponse{
		Total: result.Hits.Total.Value,
		Page:  req.Page,
		Size:  req.PageSize,
		Items: make([]*models.Config, 0, len(result.Hits.Hits)),
	}

	for _, hit := range result.Hits.Hits {
		config := hit.Source
		response.Items = append(response.Items, &config)
	}

	return response, nil
}

// SaveHistory 保存历史记录
func (r *configRepository) SaveHistory(ctx context.Context, history *models.ConfigHistory) error {
	if history.ID == "" {
		history.ID = uuid.New().String()
	}

	return r.esClient.Index(ctx, "logstash_config_history", history.ID, history)
}

// GetHistory 获取配置历史
func (r *configRepository) GetHistory(ctx context.Context, configID string) ([]*models.ConfigHistory, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"config_id": configID,
			},
		},
		"sort": []map[string]interface{}{
			{"modified_at": map[string]string{"order": "desc"}},
		},
		"size": 100, // 最多返回100条历史记录
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source models.ConfigHistory `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := r.esClient.Search(ctx, "logstash_config_history", query, &result); err != nil {
		return nil, fmt.Errorf("搜索配置历史失败: %w", err)
	}

	history := make([]*models.ConfigHistory, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		h := hit.Source
		history = append(history, &h)
	}

	return history, nil
}