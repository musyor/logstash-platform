package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromFile(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() (string, error)
		expectError bool
		validate    func(*testing.T, *AgentConfig)
	}{
		{
			name: "load valid config",
			setupFunc: func() (string, error) {
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, "config.yaml")
				content := `
server_url: "http://localhost:8080"
agent_id: "test-agent-001"
logstash_path: "/usr/share/logstash/bin/logstash"
config_dir: "/etc/logstash/conf.d"
heartbeat_interval: 30s
reconnect_interval: 5s
max_reconnect_attempts: 3
tls_enabled: false
`
				return configPath, os.WriteFile(configPath, []byte(content), 0644)
			},
			expectError: false,
			validate: func(t *testing.T, cfg *AgentConfig) {
				assert.Equal(t, "http://localhost:8080", cfg.ServerURL)
				assert.Equal(t, "test-agent-001", cfg.AgentID)
				assert.Equal(t, "/usr/share/logstash/bin/logstash", cfg.LogstashPath)
				assert.Equal(t, 30*time.Second, cfg.HeartbeatInterval)
				assert.False(t, cfg.TLSEnabled)
			},
		},
		{
			name: "load config with defaults",
			setupFunc: func() (string, error) {
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, "config.yaml")
				content := `
server_url: "http://localhost:8080"
agent_id: "test-agent-002"
`
				return configPath, os.WriteFile(configPath, []byte(content), 0644)
			},
			expectError: false,
			validate: func(t *testing.T, cfg *AgentConfig) {
				assert.Equal(t, "http://localhost:8080", cfg.ServerURL)
				assert.Equal(t, "test-agent-002", cfg.AgentID)
				// Check defaults
				assert.Equal(t, "/usr/share/logstash/bin/logstash", cfg.LogstashPath)
				assert.Equal(t, "/etc/logstash/conf.d", cfg.ConfigDir)
				assert.Equal(t, 30*time.Second, cfg.HeartbeatInterval)
				assert.Equal(t, 5*time.Second, cfg.ReconnectInterval)
			},
		},
		{
			name: "file not found",
			setupFunc: func() (string, error) {
				return "/non/existent/path/config.yaml", nil
			},
			expectError: true,
		},
		{
			name: "invalid yaml format",
			setupFunc: func() (string, error) {
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, "config.yaml")
				content := `
server_url: "http://localhost:8080"
agent_id: [invalid array instead of string]
`
				return configPath, os.WriteFile(configPath, []byte(content), 0644)
			},
			expectError: true,
		},
		{
			name: "empty config file",
			setupFunc: func() (string, error) {
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, "config.yaml")
				return configPath, os.WriteFile(configPath, []byte(""), 0644)
			},
			expectError: false,
			validate: func(t *testing.T, cfg *AgentConfig) {
				// Should have defaults from DefaultConfig
				assert.Equal(t, "http://localhost:8080", cfg.ServerURL)
				assert.Equal(t, "", cfg.AgentID)
				assert.Equal(t, "/usr/share/logstash/bin/logstash", cfg.LogstashPath)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath, err := tt.setupFunc()
			require.NoError(t, err)

			cfg, err := LoadFromFile(configPath)
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)
			
			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestSaveToFile(t *testing.T) {
	tests := []struct {
		name        string
		config      *AgentConfig
		expectError bool
	}{
		{
			name: "save valid config",
			config: &AgentConfig{
				ServerURL:            "http://localhost:8080",
				AgentID:              "test-agent-save",
				LogstashPath:         "/usr/share/logstash/bin/logstash",
				ConfigDir:            "/etc/logstash/conf.d",
				HeartbeatInterval:    45 * time.Second,
				ReconnectInterval:    15 * time.Second,
				MaxReconnectAttempts: 5,
				TLSEnabled:           true,
				TLSCertFile:          "/path/to/cert.pem",
				TLSKeyFile:           "/path/to/key.pem",
				TLSCAFile:            "/path/to/ca.pem",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			err := tt.config.SaveToFile(configPath)
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify saved file
			loadedCfg, err := LoadFromFile(configPath)
			require.NoError(t, err)
			assert.Equal(t, tt.config.ServerURL, loadedCfg.ServerURL)
			assert.Equal(t, tt.config.AgentID, loadedCfg.AgentID)
			assert.Equal(t, tt.config.HeartbeatInterval, loadedCfg.HeartbeatInterval)
			assert.Equal(t, tt.config.TLSEnabled, loadedCfg.TLSEnabled)
		})
	}
}

func TestValidate(t *testing.T) {
	// Create a temporary logstash executable for testing
	tmpDir := t.TempDir()
	logstashPath := filepath.Join(tmpDir, "logstash")
	err := os.WriteFile(logstashPath, []byte("#!/bin/bash\necho test"), 0755)
	require.NoError(t, err)

	tests := []struct {
		name        string
		config      *AgentConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &AgentConfig{
				ServerURL:            "http://localhost:8080",
				AgentID:              "valid-agent",
				LogstashPath:         logstashPath,
				ConfigDir:            filepath.Join(tmpDir, "conf.d"),
				DataDir:              filepath.Join(tmpDir, "data"),
				LogDir:               filepath.Join(tmpDir, "logs"),
				HeartbeatInterval:    30 * time.Second,
				MetricsInterval:      60 * time.Second,
				ReconnectInterval:    10 * time.Second,
				MaxReconnectAttempts: 3,
			},
			expectError: false,
		},
		{
			name: "missing server URL",
			config: &AgentConfig{
				AgentID:           "test-agent",
				LogstashPath:      logstashPath,
				ConfigDir:         filepath.Join(tmpDir, "conf.d"),
				HeartbeatInterval: 30 * time.Second,
				MetricsInterval:   60 * time.Second,
			},
			expectError: true,
			errorMsg:    "server_url 不能为空",
		},
		{
			name: "missing logstash path",
			config: &AgentConfig{
				ServerURL:         "http://localhost:8080",
				AgentID:           "test-agent",
				ConfigDir:         filepath.Join(tmpDir, "conf.d"),
				HeartbeatInterval: 30 * time.Second,
				MetricsInterval:   60 * time.Second,
			},
			expectError: true,
			errorMsg:    "logstash_path 不能为空",
		},
		{
			name: "invalid logstash path",
			config: &AgentConfig{
				ServerURL:         "http://localhost:8080",
				AgentID:           "test-agent",
				LogstashPath:      "/non/existent/logstash",
				ConfigDir:         filepath.Join(tmpDir, "conf.d"),
				HeartbeatInterval: 30 * time.Second,
				MetricsInterval:   60 * time.Second,
			},
			expectError: true,
			errorMsg:    "logstash_path 无效",
		},
		{
			name: "heartbeat interval too small",
			config: &AgentConfig{
				ServerURL:         "http://localhost:8080",
				AgentID:           "test-agent",
				LogstashPath:      logstashPath,
				ConfigDir:         filepath.Join(tmpDir, "conf.d"),
				HeartbeatInterval: 5 * time.Second, // Less than 10 seconds
				MetricsInterval:   60 * time.Second,
			},
			expectError: true,
			errorMsg:    "heartbeat_interval 不能小于10秒",
		},
		{
			name: "metrics interval too small",
			config: &AgentConfig{
				ServerURL:         "http://localhost:8080",
				AgentID:           "test-agent",
				LogstashPath:      logstashPath,
				ConfigDir:         filepath.Join(tmpDir, "conf.d"),
				HeartbeatInterval: 30 * time.Second,
				MetricsInterval:   20 * time.Second, // Less than 30 seconds
			},
			expectError: true,
			errorMsg:    "metrics_interval 不能小于30秒",
		},
		{
			name: "TLS enabled but missing cert file",
			config: &AgentConfig{
				ServerURL:         "https://localhost:8443",
				AgentID:           "test-agent",
				LogstashPath:      logstashPath,
				ConfigDir:         filepath.Join(tmpDir, "conf.d"),
				HeartbeatInterval: 30 * time.Second,
				MetricsInterval:   60 * time.Second,
				TLSEnabled:        true,
				TLSKeyFile:        "/path/to/key.pem",
			},
			expectError: true,
			errorMsg:    "启用TLS时必须提供证书和密钥文件",
		},
		{
			name: "TLS enabled with non-existent cert files",
			config: &AgentConfig{
				ServerURL:         "https://localhost:8443",
				AgentID:           "test-agent",
				LogstashPath:      logstashPath,
				ConfigDir:         filepath.Join(tmpDir, "conf.d"),
				HeartbeatInterval: 30 * time.Second,
				MetricsInterval:   60 * time.Second,
				TLSEnabled:        true,
				TLSCertFile:       "/non/existent/cert.pem",
				TLSKeyFile:        "/non/existent/key.pem",
			},
			expectError: true,
			errorMsg:    "TLS证书文件不存在",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	
	assert.NotNil(t, cfg)
	assert.Equal(t, "/usr/share/logstash/bin/logstash", cfg.LogstashPath)
	assert.Equal(t, "/etc/logstash/conf.d", cfg.ConfigDir)
	assert.Equal(t, 30*time.Second, cfg.HeartbeatInterval)
	assert.Equal(t, 5*time.Second, cfg.ReconnectInterval)
	assert.Equal(t, 10, cfg.MaxReconnectAttempts)
	assert.False(t, cfg.TLSEnabled)
	assert.Equal(t, 3, cfg.ConfigBackupCount)
	assert.True(t, cfg.EnableAutoReload)
}

func TestGetLogstashConfigPath(t *testing.T) {
	cfg := &AgentConfig{
		ConfigDir: "/etc/logstash/conf.d",
	}
	
	path := cfg.GetLogstashConfigPath("test-config-123")
	assert.Equal(t, "/etc/logstash/conf.d/test-config-123.conf", path)
}

func TestGetConfigBackupPath(t *testing.T) {
	cfg := &AgentConfig{
		ConfigDir: "/etc/logstash/conf.d",
	}
	
	path := cfg.GetConfigBackupPath("test-config-123", 1)
	assert.Equal(t, "/etc/logstash/conf.d/.backup/test-config-123.conf.backup.1", path)
}

func TestConfigWithEnvironmentVariables(t *testing.T) {
	// Test environment variable substitution
	os.Setenv("TEST_SERVER_URL", "http://env-server:8080")
	os.Setenv("TEST_AGENT_ID", "env-agent-001")
	defer func() {
		os.Unsetenv("TEST_SERVER_URL")
		os.Unsetenv("TEST_AGENT_ID")
	}()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
server_url: "${TEST_SERVER_URL}"
agent_id: "${TEST_AGENT_ID}"
heartbeat_interval: 30s
`
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadFromFile(configPath)
	require.NoError(t, err)
	
	// Note: YAML parser doesn't expand environment variables by default
	// This test verifies the actual behavior
	assert.Contains(t, cfg.ServerURL, "TEST_SERVER_URL")
	assert.Contains(t, cfg.AgentID, "TEST_AGENT_ID")
}

func TestConfigPermissions(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	
	// Create config file
	content := `server_url: "http://localhost:8080"`
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)
	
	// Make file unreadable
	err = os.Chmod(configPath, 0000)
	require.NoError(t, err)
	defer os.Chmod(configPath, 0644) // Restore permissions
	
	_, err = LoadFromFile(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestConfigDurationParsing(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
server_url: "http://localhost:8080"
agent_id: "test-agent"
heartbeat_interval: "45s"
metrics_interval: "2m"
reconnect_interval: "10s"
request_timeout: "1m30s"
websocket_ping_interval: "30s"
reload_debounce_time: "5s"
`
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadFromFile(configPath)
	require.NoError(t, err)
	
	assert.Equal(t, 45*time.Second, cfg.HeartbeatInterval)
	assert.Equal(t, 2*time.Minute, cfg.MetricsInterval)
	assert.Equal(t, 10*time.Second, cfg.ReconnectInterval)
	assert.Equal(t, 90*time.Second, cfg.RequestTimeout)
	assert.Equal(t, 30*time.Second, cfg.WebSocketPingInterval)
	assert.Equal(t, 5*time.Second, cfg.ReloadDebounceTime)
}