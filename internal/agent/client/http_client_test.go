package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"logstash-platform/internal/agent/config"
	"logstash-platform/internal/platform/models"
)

func TestNewHTTPClient(t *testing.T) {
	logger := logrus.New()
	
	tests := []struct {
		name        string
		config      *config.AgentConfig
		expectError bool
	}{
		{
			name: "create with valid config",
			config: &config.AgentConfig{
				ServerURL: "http://localhost:8080",
				AgentID:   "test-agent",
				TLSEnabled: false,
			},
			expectError: false,
		},
		{
			name: "create with TLS config",
			config: &config.AgentConfig{
				ServerURL: "https://localhost:8443",
				AgentID:   "test-agent-tls",
				TLSEnabled: true,
				TLSCertFile: "testdata/cert.pem",
				TLSKeyFile:  "testdata/key.pem",
				TLSCAFile:   "testdata/ca.pem",
			},
			expectError: true, // Will fail with test certs
		},
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
		},
		{
			name: "invalid server URL",
			config: &config.AgentConfig{
				ServerURL: "://invalid-url",
				AgentID:   "test-agent",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewHTTPClient(tt.config, logger)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestHTTPClient_Register(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	tests := []struct {
		name           string
		agent          *models.Agent
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectError    bool
	}{
		{
			name: "successful registration",
			agent: &models.Agent{
				AgentID:         "test-agent",
				Hostname:        "test-host",
				IP:              "192.168.1.100",
				LogstashVersion: "7.17.0",
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/agents/register", r.URL.Path)
				assert.Equal(t, "POST", r.Method)

				var req map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&req)
				assert.NoError(t, err)
				assert.Equal(t, "test-agent", req["agent_id"])
				assert.Equal(t, "test-host", req["hostname"])

				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name: "registration conflict",
			agent: &models.Agent{
				AgentID:  "test-agent",
				Hostname: "test-host",
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "agent already exists",
				})
			},
			expectError: true,
		},
		{
			name: "server error",
			agent: &models.Agent{
				AgentID: "test-agent",
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			cfg := &config.AgentConfig{
				ServerURL: server.URL,
				AgentID:   "test-agent",
			}
			client, err := NewHTTPClient(cfg, logger)
			require.NoError(t, err)

			ctx := context.Background()
			err = client.Register(ctx, tt.agent)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPClient_SendHeartbeat(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	tests := []struct {
		name           string
		agentID        string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectError    bool
	}{
		{
			name:    "successful heartbeat",
			agentID: "test-agent",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/agents/test-agent/heartbeat", r.URL.Path)
				assert.Equal(t, "POST", r.Method)
				
				var req map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&req)
				assert.NoError(t, err)
				assert.NotNil(t, req["timestamp"])
				
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name:    "agent not found",
			agentID: "unknown-agent",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
		{
			name:    "server error",
			agentID: "test-agent",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			cfg := &config.AgentConfig{
				ServerURL: server.URL,
				AgentID:   tt.agentID,
			}
			client, err := NewHTTPClient(cfg, logger)
			require.NoError(t, err)

			ctx := context.Background()
			err = client.SendHeartbeat(ctx, tt.agentID)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPClient_ReportStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	tests := []struct {
		name           string
		agent          *models.Agent
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectError    bool
	}{
		{
			name: "successful status update",
			agent: &models.Agent{
				AgentID:  "test-agent",
				Status:   "online",
				Hostname: "test-host",
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/agents/test-agent/status", r.URL.Path)
				assert.Equal(t, "PUT", r.Method)

				var agent models.Agent
				err := json.NewDecoder(r.Body).Decode(&agent)
				assert.NoError(t, err)
				assert.Equal(t, "online", agent.Status)

				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name: "nil agent",
			agent: nil,
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Should not be called
				t.Error("Server should not be called with nil agent")
			},
			expectError: true,
		},
		{
			name: "server error",
			agent: &models.Agent{
				AgentID: "test-agent",
				Status:  "error",
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			cfg := &config.AgentConfig{
				ServerURL: server.URL,
				AgentID:   "test-agent",
			}
			client, err := NewHTTPClient(cfg, logger)
			require.NoError(t, err)

			ctx := context.Background()
			err = client.ReportStatus(ctx, tt.agent)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPClient_GetConfig(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	tests := []struct {
		name           string
		configID       string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectError    bool
		validateFunc   func(*testing.T, *models.Config)
	}{
		{
			name:     "successful get config",
			configID: "config-123",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/configs/config-123", r.URL.Path)
				assert.Equal(t, "GET", r.Method)

				config := &models.Config{
					ID:      "config-123",
					Name:    "Test Config",
					Content: "input { stdin {} } output { stdout {} }",
					Version: 1,
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(config)
			},
			expectError: false,
			validateFunc: func(t *testing.T, cfg *models.Config) {
				assert.Equal(t, "config-123", cfg.ID)
				assert.Equal(t, "Test Config", cfg.Name)
				assert.Equal(t, 1, cfg.Version)
			},
		},
		{
			name:     "config not found",
			configID: "non-existent",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
		{
			name:     "empty config ID",
			configID: "",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Should not be called
				t.Error("Server should not be called with empty config ID")
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			cfg := &config.AgentConfig{
				ServerURL: server.URL,
				AgentID:   "test-agent",
			}
			client, err := NewHTTPClient(cfg, logger)
			require.NoError(t, err)

			ctx := context.Background()
			config, err := client.GetConfig(ctx, tt.configID)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				require.NoError(t, err)
				require.NotNil(t, config)
				if tt.validateFunc != nil {
					tt.validateFunc(t, config)
				}
			}
		})
	}
}

func TestHTTPClient_ReportConfigApplied(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	tests := []struct {
		name           string
		agentID        string
		applied        *models.AppliedConfig
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectError    bool
	}{
		{
			name:    "successful report",
			agentID: "test-agent",
			applied: &models.AppliedConfig{
				ConfigID:  "config-123",
				Version:   1,
				AppliedAt: time.Now(),
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/agents/test-agent/configs/applied", r.URL.Path)
				assert.Equal(t, "POST", r.Method)

				var applied models.AppliedConfig
				err := json.NewDecoder(r.Body).Decode(&applied)
				assert.NoError(t, err)
				assert.Equal(t, "config-123", applied.ConfigID)
				assert.Equal(t, 1, applied.Version)

				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name:    "failed deployment report",
			agentID: "test-agent",
			applied: &models.AppliedConfig{
				ConfigID:  "config-456",
				Version:   2,
				AppliedAt: time.Now(),
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				var applied models.AppliedConfig
				err := json.NewDecoder(r.Body).Decode(&applied)
				assert.NoError(t, err)
				assert.Equal(t, "config-456", applied.ConfigID)
				assert.Equal(t, 2, applied.Version)

				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name:    "nil applied config",
			agentID: "test-agent",
			applied: nil,
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Should not be called
				t.Error("Server should not be called with nil applied config")
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			cfg := &config.AgentConfig{
				ServerURL: server.URL,
				AgentID:   tt.agentID,
			}
			client, err := NewHTTPClient(cfg, logger)
			require.NoError(t, err)

			ctx := context.Background()
			err = client.ReportConfigApplied(ctx, tt.agentID, tt.applied)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPClient_Timeout(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.AgentConfig{
		ServerURL:      server.URL,
		AgentID:        "test-agent",
		RequestTimeout: 100 * time.Millisecond, // Short timeout for test
	}
	client, err := NewHTTPClient(cfg, logger)
	require.NoError(t, err)

	// Test timeout
	ctx := context.Background()
	err = client.SendHeartbeat(ctx, "test-agent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestHTTPClient_Headers(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-agent", r.Header.Get("X-Agent-ID"))
		assert.NotEmpty(t, r.Header.Get("User-Agent"))
		
		// Check auth token if configured
		if token := r.Header.Get("Authorization"); token != "" {
			assert.Equal(t, "Bearer test-token", token)
		}
		
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.AgentConfig{
		ServerURL: server.URL,
		AgentID:   "test-agent",
		Token:     "test-token",
	}
	client, err := NewHTTPClient(cfg, logger)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.SendHeartbeat(ctx, "test-agent")
	assert.NoError(t, err)
}

func TestHTTPClient_ContextCancellation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.AgentConfig{
		ServerURL: server.URL,
		AgentID:   "test-agent",
	}
	client, err := NewHTTPClient(cfg, logger)
	require.NoError(t, err)

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = client.SendHeartbeat(ctx, "test-agent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestHTTPClient_Close(t *testing.T) {
	logger := logrus.New()
	
	cfg := &config.AgentConfig{
		ServerURL: "http://localhost:8080",
		AgentID:   "test-agent",
	}
	client, err := NewHTTPClient(cfg, logger)
	require.NoError(t, err)

	err = client.Close()
	assert.NoError(t, err)
}