package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Client ES客户端封装
type Client struct {
	es     *elasticsearch.Client
	logger *logrus.Logger
	config *Config
}

// Config ES配置
type Config struct {
	Addresses  []string
	Username   string
	Password   string
	MaxRetries int
	Timeout    time.Duration
	Indices    struct {
		Configs       string
		ConfigHistory string
		Agents        string
	}
}

// NewClient 创建新的ES客户端
func NewClient(logger *logrus.Logger) (*Client, error) {
	config := &Config{
		Addresses:  viper.GetStringSlice("elasticsearch.addresses"),
		Username:   viper.GetString("elasticsearch.username"),
		Password:   viper.GetString("elasticsearch.password"),
		MaxRetries: viper.GetInt("elasticsearch.max_retries"),
		Timeout:    viper.GetDuration("elasticsearch.timeout"),
	}
	
	config.Indices.Configs = viper.GetString("elasticsearch.indices.configs")
	config.Indices.ConfigHistory = viper.GetString("elasticsearch.indices.config_history")
	config.Indices.Agents = viper.GetString("elasticsearch.indices.agents")

	// 创建ES客户端配置
	esCfg := elasticsearch.Config{
		Addresses:  config.Addresses,
		MaxRetries: config.MaxRetries,
	}
	
	if config.Username != "" && config.Password != "" {
		esCfg.Username = config.Username
		esCfg.Password = config.Password
	}

	// 创建ES客户端
	es, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("创建ES客户端失败: %w", err)
	}

	// 测试连接
	res, err := es.Info()
	if err != nil {
		return nil, fmt.Errorf("连接ES失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("ES响应错误: %s", res.String())
	}

	logger.Info("成功连接到Elasticsearch")

	return &Client{
		es:     es,
		logger: logger,
		config: config,
	}, nil
}

// InitializeIndices 初始化索引
func (c *Client) InitializeIndices(ctx context.Context) error {
	indices := []struct {
		name    string
		mapping string
	}{
		{
			name:    c.config.Indices.Configs,
			mapping: configIndexMapping,
		},
		{
			name:    c.config.Indices.ConfigHistory,
			mapping: configHistoryIndexMapping,
		},
		{
			name:    c.config.Indices.Agents,
			mapping: agentIndexMapping,
		},
	}

	for _, index := range indices {
		exists, err := c.IndexExists(ctx, index.name)
		if err != nil {
			return fmt.Errorf("检查索引 %s 是否存在失败: %w", index.name, err)
		}

		if !exists {
			if err := c.CreateIndex(ctx, index.name, index.mapping); err != nil {
				return fmt.Errorf("创建索引 %s 失败: %w", index.name, err)
			}
			c.logger.Infof("创建索引: %s", index.name)
		} else {
			c.logger.Infof("索引已存在: %s", index.name)
		}
	}

	return nil
}

// IndexExists 检查索引是否存在
func (c *Client) IndexExists(ctx context.Context, index string) (bool, error) {
	res, err := c.es.Indices.Exists([]string{index})
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	return res.StatusCode == 200, nil
}

// CreateIndex 创建索引
func (c *Client) CreateIndex(ctx context.Context, index, mapping string) error {
	req := esapi.IndicesCreateRequest{
		Index: index,
		Body:  strings.NewReader(mapping),
	}

	res, err := req.Do(ctx, c.es)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("创建索引响应错误: %s", res.String())
	}

	return nil
}

// Index 索引文档
func (c *Client) Index(ctx context.Context, index, id string, doc interface{}) error {
	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("序列化文档失败: %w", err)
	}

	req := esapi.IndexRequest{
		Index:      index,
		DocumentID: id,
		Body:       strings.NewReader(string(data)),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, c.es)
	if err != nil {
		return fmt.Errorf("索引文档失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("索引文档响应错误: %s", res.String())
	}

	return nil
}

// Get 获取文档
func (c *Client) Get(ctx context.Context, index, id string, result interface{}) error {
	req := esapi.GetRequest{
		Index:      index,
		DocumentID: id,
	}

	res, err := req.Do(ctx, c.es)
	if err != nil {
		return fmt.Errorf("获取文档失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		if res.StatusCode == 404 {
			return fmt.Errorf("文档不存在")
		}
		return fmt.Errorf("获取文档响应错误: %s", res.String())
	}

	var response struct {
		Source json.RawMessage `json:"_source"`
		Found  bool            `json:"found"`
	}

	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if !response.Found {
		return fmt.Errorf("文档不存在")
	}

	if err := json.Unmarshal(response.Source, result); err != nil {
		return fmt.Errorf("解析文档失败: %w", err)
	}

	return nil
}

// Search 搜索文档
func (c *Client) Search(ctx context.Context, index string, query map[string]interface{}, results interface{}) error {
	data, err := json.Marshal(query)
	if err != nil {
		return fmt.Errorf("序列化查询失败: %w", err)
	}

	req := esapi.SearchRequest{
		Index: []string{index},
		Body:  strings.NewReader(string(data)),
	}

	res, err := req.Do(ctx, c.es)
	if err != nil {
		return fmt.Errorf("搜索失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("搜索响应错误: %s", res.String())
	}

	if err := json.NewDecoder(res.Body).Decode(results); err != nil {
		return fmt.Errorf("解析搜索结果失败: %w", err)
	}

	return nil
}

// Delete 删除文档
func (c *Client) Delete(ctx context.Context, index, id string) error {
	req := esapi.DeleteRequest{
		Index:      index,
		DocumentID: id,
		Refresh:    "true",
	}

	res, err := req.Do(ctx, c.es)
	if err != nil {
		return fmt.Errorf("删除文档失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("删除文档响应错误: %s", res.String())
	}

	return nil
}

// 索引映射定义
const (
	configIndexMapping = `{
		"mappings": {
			"properties": {
				"id": { "type": "keyword" },
				"name": { "type": "text" },
				"description": { "type": "text" },
				"type": { "type": "keyword" },
				"content": { "type": "text" },
				"tags": { "type": "keyword" },
				"version": { "type": "integer" },
				"enabled": { "type": "boolean" },
				"test_status": { "type": "keyword" },
				"created_at": { "type": "date" },
				"updated_at": { "type": "date" },
				"created_by": { "type": "keyword" },
				"updated_by": { "type": "keyword" }
			}
		}
	}`

	configHistoryIndexMapping = `{
		"mappings": {
			"properties": {
				"config_id": { "type": "keyword" },
				"version": { "type": "integer" },
				"content": { "type": "text" },
				"change_type": { "type": "keyword" },
				"change_log": { "type": "text" },
				"modified_by": { "type": "keyword" },
				"modified_at": { "type": "date" }
			}
		}
	}`

	agentIndexMapping = `{
		"mappings": {
			"properties": {
				"agent_id": { "type": "keyword" },
				"hostname": { "type": "keyword" },
				"ip": { "type": "ip" },
				"logstash_version": { "type": "keyword" },
				"status": { "type": "keyword" },
				"last_heartbeat": { "type": "date" },
				"applied_configs": {
					"type": "nested",
					"properties": {
						"config_id": { "type": "keyword" },
						"version": { "type": "integer" },
						"applied_at": { "type": "date" }
					}
				}
			}
		}
	}`
)