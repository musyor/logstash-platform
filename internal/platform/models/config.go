package models

import (
	"time"
)

// ConfigType 配置类型
type ConfigType string

const (
	ConfigTypeInput  ConfigType = "input"
	ConfigTypeFilter ConfigType = "filter"
	ConfigTypeOutput ConfigType = "output"
)

// TestStatus 测试状态
type TestStatus string

const (
	TestStatusUntested TestStatus = "untested"
	TestStatusTesting  TestStatus = "testing"
	TestStatusPassed   TestStatus = "passed"
	TestStatusFailed   TestStatus = "failed"
)

// Config 配置模型
type Config struct {
	ID          string     `json:"id"`
	Name        string     `json:"name" binding:"required,min=1,max=100"`
	Description string     `json:"description"`
	Type        ConfigType `json:"type" binding:"required,oneof=input filter output"`
	Content     string     `json:"content" binding:"required"`
	Tags        []string   `json:"tags"`
	Version     int        `json:"version"`
	Enabled     bool       `json:"enabled"`
	TestStatus  TestStatus `json:"test_status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CreatedBy   string     `json:"created_by"`
	UpdatedBy   string     `json:"updated_by"`
}

// ConfigHistory 配置历史记录
type ConfigHistory struct {
	ID         string     `json:"id"`
	ConfigID   string     `json:"config_id"`
	Version    int        `json:"version"`
	Content    string     `json:"content"`
	ChangeType string     `json:"change_type"` // create, update, delete
	ChangeLog  string     `json:"change_log"`
	ModifiedBy string     `json:"modified_by"`
	ModifiedAt time.Time  `json:"modified_at"`
}

// ConfigListRequest 配置列表请求
type ConfigListRequest struct {
	Type     ConfigType `form:"type"`
	Tags     []string   `form:"tags"`
	Enabled  *bool      `form:"enabled"`
	Page     int        `form:"page,default=1"`
	PageSize int        `form:"size,default=10"`
}

// ConfigListResponse 配置列表响应
type ConfigListResponse struct {
	Total int64     `json:"total"`
	Page  int       `json:"page"`
	Size  int       `json:"size"`
	Items []*Config `json:"items"`
}

// CreateConfigRequest 创建配置请求
type CreateConfigRequest struct {
	Name        string     `json:"name" binding:"required,min=1,max=100"`
	Description string     `json:"description"`
	Type        ConfigType `json:"type" binding:"required,oneof=input filter output"`
	Content     string     `json:"content" binding:"required"`
	Tags        []string   `json:"tags"`
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	Name        string     `json:"name" binding:"required,min=1,max=100"`
	Description string     `json:"description"`
	Type        ConfigType `json:"type" binding:"required,oneof=input filter output"`
	Content     string     `json:"content" binding:"required"`
	Tags        []string   `json:"tags"`
	Enabled     *bool      `json:"enabled"`
}

// TestConfigRequest 测试配置请求
type TestConfigRequest struct {
	ConfigID string   `json:"config_id" binding:"required"`
	TestData TestData `json:"test_data" binding:"required"`
}

// TestData 测试数据
type TestData struct {
	Type        string      `json:"type" binding:"required,oneof=sample kafka"`
	Samples     []string    `json:"samples,omitempty"`
	KafkaConfig KafkaConfig `json:"kafka_config,omitempty"`
}

// KafkaConfig Kafka配置
type KafkaConfig struct {
	Brokers       []string `json:"brokers"`
	Topic         string   `json:"topic"`
	ConsumerGroup string   `json:"consumer_group"`
	MaxMessages   int      `json:"max_messages"`
	Timeout       int      `json:"timeout_seconds"`
}

// TestResult 测试结果
type TestResult struct {
	TestID      string        `json:"test_id"`
	Status      string        `json:"status"` // running, completed, failed
	InputCount  int           `json:"input_count"`
	OutputCount int           `json:"output_count"`
	Results     []TestOutput  `json:"results"`
	Errors      []string      `json:"errors"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     *time.Time    `json:"end_time"`
}

// TestOutput 测试输出
type TestOutput struct {
	Input  string                 `json:"input"`
	Output map[string]interface{} `json:"output"`
	Error  string                 `json:"error,omitempty"`
}

// Agent 代理信息
type Agent struct {
	AgentID         string          `json:"agent_id"`
	Hostname        string          `json:"hostname"`
	IP              string          `json:"ip"`
	LogstashVersion string          `json:"logstash_version"`
	Status          string          `json:"status"` // online, offline, error
	LastHeartbeat   time.Time       `json:"last_heartbeat"`
	AppliedConfigs  []AppliedConfig `json:"applied_configs"`
}

// AppliedConfig 已应用的配置
type AppliedConfig struct {
	ConfigID  string    `json:"config_id"`
	Version   int       `json:"version"`
	AppliedAt time.Time `json:"applied_at"`
}

// DeployRequest 部署请求
type DeployRequest struct {
	ConfigID string   `json:"config_id" binding:"required"`
	AgentIDs []string `json:"agent_ids" binding:"required,min=1"`
}