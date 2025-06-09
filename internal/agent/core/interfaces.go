package core

import (
	"context"
	"encoding/json"
	"time"

	"logstash-platform/internal/platform/models"
)

// AgentCore Agent核心功能接口
type AgentCore interface {
	// Start 启动Agent
	Start(ctx context.Context) error
	
	// Stop 停止Agent
	Stop(ctx context.Context) error
	
	// Register 注册到管理平台
	Register(ctx context.Context) error
	
	// GetStatus 获取Agent状态
	GetStatus() *models.Agent
}

// APIClient API通信客户端接口
type APIClient interface {
	// Register 注册Agent
	Register(ctx context.Context, agent *models.Agent) error
	
	// SendHeartbeat 发送心跳
	SendHeartbeat(ctx context.Context, agentID string) error
	
	// ReportStatus 上报状态
	ReportStatus(ctx context.Context, agent *models.Agent) error
	
	// GetConfig 获取配置
	GetConfig(ctx context.Context, configID string) (*models.Config, error)
	
	// ReportConfigApplied 上报配置应用结果
	ReportConfigApplied(ctx context.Context, agentID string, applied *models.AppliedConfig) error
	
	// ConnectWebSocket 建立WebSocket连接
	ConnectWebSocket(ctx context.Context, agentID string, handler MessageHandler) error
	
	// Close 关闭客户端
	Close() error
}

// ConfigManager 配置管理器接口
type ConfigManager interface {
	// SaveConfig 保存配置到本地
	SaveConfig(config *models.Config) error
	
	// LoadConfig 加载本地配置
	LoadConfig(configID string) (*models.Config, error)
	
	// DeleteConfig 删除本地配置
	DeleteConfig(configID string) error
	
	// ListConfigs 列出所有本地配置
	ListConfigs() ([]*models.Config, error)
	
	// GetConfigPath 获取配置文件路径
	GetConfigPath(configID string) string
	
	// BackupConfig 备份配置
	BackupConfig(configID string) error
	
	// RestoreConfig 恢复配置
	RestoreConfig(configID string) error
}

// LogstashController Logstash控制器接口
type LogstashController interface {
	// Start 启动Logstash
	Start(ctx context.Context) error
	
	// Stop 停止Logstash
	Stop(ctx context.Context) error
	
	// Restart 重启Logstash
	Restart(ctx context.Context) error
	
	// Reload 重新加载配置
	Reload(ctx context.Context) error
	
	// IsRunning 检查是否运行中
	IsRunning() bool
	
	// GetStatus 获取Logstash状态
	GetStatus() (*LogstashStatus, error)
	
	// ValidateConfig 验证配置文件
	ValidateConfig(configPath string) error
}

// HeartbeatService 心跳服务接口
type HeartbeatService interface {
	// Start 启动心跳服务
	Start(ctx context.Context) error
	
	// Stop 停止心跳服务
	Stop() error
	
	// SetInterval 设置心跳间隔
	SetInterval(interval time.Duration)
}

// MetricsCollector 指标收集器接口
type MetricsCollector interface {
	// Start 启动指标收集
	Start(ctx context.Context) error
	
	// Stop 停止指标收集
	Stop() error
	
	// GetMetrics 获取当前指标
	GetMetrics() (*AgentMetrics, error)
	
	// SetInterval 设置收集间隔
	SetInterval(interval time.Duration)
}

// MessageHandler WebSocket消息处理器
type MessageHandler interface {
	// HandleMessage 处理接收到的消息
	HandleMessage(msgType string, payload []byte) error
	
	// OnConnect 连接建立时调用
	OnConnect() error
	
	// OnDisconnect 连接断开时调用
	OnDisconnect(err error)
}

// LogstashStatus Logstash状态
type LogstashStatus struct {
	Running        bool      `json:"running"`
	PID            int       `json:"pid"`
	Version        string    `json:"version"`
	ConfigPath     string    `json:"config_path"`
	StartTime      time.Time `json:"start_time"`
	LastReloadTime time.Time `json:"last_reload_time"`
}

// AgentMetrics Agent指标
type AgentMetrics struct {
	Timestamp      time.Time `json:"timestamp"`
	CPUUsage       float64   `json:"cpu_usage"`        // CPU使用率 (%)
	MemoryUsage    float64   `json:"memory_usage"`     // 内存使用率 (%)
	DiskUsage      float64   `json:"disk_usage"`       // 磁盘使用率 (%)
	EventsReceived int64     `json:"events_received"`  // 接收事件数
	EventsSent     int64     `json:"events_sent"`      // 发送事件数
	EventsFailed   int64     `json:"events_failed"`    // 失败事件数
	Uptime         int64     `json:"uptime"`           // 运行时间 (秒)
}

// WebSocketMessage WebSocket消息
type WebSocketMessage struct {
	Type      string          `json:"type"`      // 消息类型
	Timestamp time.Time       `json:"timestamp"` // 时间戳
	Payload   json.RawMessage `json:"payload"`   // 消息内容
}

// 消息类型常量
const (
	// 服务器到Agent的消息类型
	MsgTypeConfigDeploy   = "config_deploy"    // 配置部署
	MsgTypeConfigDelete   = "config_delete"    // 配置删除
	MsgTypeReloadRequest  = "reload_request"   // 重载请求
	MsgTypeStatusRequest  = "status_request"   // 状态请求
	MsgTypeMetricsRequest = "metrics_request"  // 指标请求
	
	// Agent到服务器的消息类型
	MsgTypeHeartbeat      = "heartbeat"        // 心跳
	MsgTypeStatusReport   = "status_report"    // 状态上报
	MsgTypeMetricsReport  = "metrics_report"   // 指标上报
	MsgTypeConfigApplied  = "config_applied"   // 配置已应用
	MsgTypeError          = "error"            // 错误报告
)