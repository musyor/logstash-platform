package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// AgentConfig Agent配置
type AgentConfig struct {
	// 基础配置
	AgentID      string `yaml:"agent_id"`       // Agent唯一标识
	ServerURL    string `yaml:"server_url"`     // 管理平台地址
	Token        string `yaml:"token"`          // 认证令牌
	
	// Logstash配置
	LogstashPath    string `yaml:"logstash_path"`     // Logstash执行文件路径
	ConfigDir       string `yaml:"config_dir"`        // 配置文件目录
	DataDir         string `yaml:"data_dir"`          // 数据目录
	LogDir          string `yaml:"log_dir"`           // 日志目录
	PipelineWorkers int    `yaml:"pipeline_workers"`  // Pipeline工作线程数
	BatchSize       int    `yaml:"batch_size"`        // 批处理大小
	
	// 通信配置
	HeartbeatInterval   time.Duration `yaml:"heartbeat_interval"`    // 心跳间隔
	MetricsInterval     time.Duration `yaml:"metrics_interval"`      // 指标上报间隔
	ReconnectInterval   time.Duration `yaml:"reconnect_interval"`    // 重连间隔
	RequestTimeout      time.Duration `yaml:"request_timeout"`       // 请求超时
	MaxReconnectAttempts int          `yaml:"max_reconnect_attempts"` // 最大重连次数
	
	// WebSocket配置
	EnableWebSocket     bool          `yaml:"enable_websocket"`      // 是否启用WebSocket
	WebSocketPingInterval time.Duration `yaml:"websocket_ping_interval"` // WebSocket Ping间隔
	
	// 安全配置
	TLSEnabled     bool   `yaml:"tls_enabled"`      // 是否启用TLS
	TLSCertFile    string `yaml:"tls_cert_file"`    // TLS证书文件
	TLSKeyFile     string `yaml:"tls_key_file"`     // TLS密钥文件
	TLSCAFile      string `yaml:"tls_ca_file"`      // TLS CA文件
	TLSSkipVerify  bool   `yaml:"tls_skip_verify"`  // 是否跳过证书验证
	
	// 高级配置
	MaxConfigSize      int64  `yaml:"max_config_size"`       // 最大配置文件大小
	ConfigBackupCount  int    `yaml:"config_backup_count"`   // 配置备份数量
	EnableAutoReload   bool   `yaml:"enable_auto_reload"`    // 是否启用自动重载
	ReloadDebounceTime time.Duration `yaml:"reload_debounce_time"` // 重载防抖时间
}

// DefaultConfig 返回默认配置
func DefaultConfig() *AgentConfig {
	return &AgentConfig{
		AgentID:      "",
		ServerURL:    "http://localhost:8080",
		Token:        "",
		
		LogstashPath:    "/usr/share/logstash/bin/logstash",
		ConfigDir:       "/etc/logstash/conf.d",
		DataDir:         "/var/lib/logstash",
		LogDir:          "/var/log/logstash",
		PipelineWorkers: 2,
		BatchSize:       125,
		
		HeartbeatInterval:    30 * time.Second,
		MetricsInterval:      60 * time.Second,
		ReconnectInterval:    5 * time.Second,
		RequestTimeout:       30 * time.Second,
		MaxReconnectAttempts: 10,
		
		EnableWebSocket:       true,
		WebSocketPingInterval: 30 * time.Second,
		
		TLSEnabled:     false,
		TLSCertFile:    "",
		TLSKeyFile:     "",
		TLSCAFile:      "",
		TLSSkipVerify:  false,
		
		MaxConfigSize:      10 * 1024 * 1024, // 10MB
		ConfigBackupCount:  3,
		EnableAutoReload:   true,
		ReloadDebounceTime: 5 * time.Second,
	}
}

// LoadFromFile 从文件加载配置
func LoadFromFile(filename string) (*AgentConfig, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 先加载默认配置
	cfg := DefaultConfig()
	
	// 解析YAML覆盖默认值
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return cfg, nil
}

// SaveToFile 保存配置到文件
func (c *AgentConfig) SaveToFile(filename string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// Validate 验证配置
func (c *AgentConfig) Validate() error {
	if c.ServerURL == "" {
		return fmt.Errorf("server_url 不能为空")
	}

	if c.LogstashPath == "" {
		return fmt.Errorf("logstash_path 不能为空")
	}

	// 检查Logstash路径是否存在
	if _, err := os.Stat(c.LogstashPath); err != nil {
		return fmt.Errorf("logstash_path 无效: %w", err)
	}

	if c.ConfigDir == "" {
		return fmt.Errorf("config_dir 不能为空")
	}

	// 创建必要的目录
	dirs := []string{c.ConfigDir, c.DataDir, c.LogDir}
	for _, dir := range dirs {
		if dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("创建目录 %s 失败: %w", dir, err)
			}
		}
	}

	// 验证时间间隔
	if c.HeartbeatInterval < 10*time.Second {
		return fmt.Errorf("heartbeat_interval 不能小于10秒")
	}

	if c.MetricsInterval < 30*time.Second {
		return fmt.Errorf("metrics_interval 不能小于30秒")
	}

	// 验证TLS配置
	if c.TLSEnabled {
		if c.TLSCertFile == "" || c.TLSKeyFile == "" {
			return fmt.Errorf("启用TLS时必须提供证书和密钥文件")
		}
		
		// 检查证书文件是否存在
		if _, err := os.Stat(c.TLSCertFile); err != nil {
			return fmt.Errorf("TLS证书文件不存在: %w", err)
		}
		
		if _, err := os.Stat(c.TLSKeyFile); err != nil {
			return fmt.Errorf("TLS密钥文件不存在: %w", err)
		}
		
		if c.TLSCAFile != "" {
			if _, err := os.Stat(c.TLSCAFile); err != nil {
				return fmt.Errorf("TLS CA文件不存在: %w", err)
			}
		}
	}

	return nil
}

// GetLogstashConfigPath 获取Logstash配置文件完整路径
func (c *AgentConfig) GetLogstashConfigPath(configID string) string {
	return fmt.Sprintf("%s/%s.conf", c.ConfigDir, configID)
}

// GetConfigBackupPath 获取配置备份路径
func (c *AgentConfig) GetConfigBackupPath(configID string, version int) string {
	return fmt.Sprintf("%s/%s.conf.backup.%d", c.ConfigDir, configID, version)
}