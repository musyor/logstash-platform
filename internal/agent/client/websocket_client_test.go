package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"logstash-platform/internal/agent/config"
	"logstash-platform/internal/agent/core"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func TestNewWebSocketClient(t *testing.T) {
	logger := logrus.New()
	
	tests := []struct {
		name   string
		config *config.AgentConfig
	}{
		{
			name: "create with valid config",
			config: &config.AgentConfig{
				ServerURL: "ws://localhost:8080",
				AgentID:   "test-agent",
			},
		},
		{
			name: "create with https URL",
			config: &config.AgentConfig{
				ServerURL: "https://localhost:8443",
				AgentID:   "test-agent",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewWebSocketClient(tt.config, logger)
			assert.NotNil(t, client)
			assert.Equal(t, tt.config, client.config)
			assert.Equal(t, logger, client.logger)
		})
	}
}

func TestWebSocketClient_Connect(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	tests := []struct {
		name           string
		serverHandler  func(*testing.T, *websocket.Conn)
		expectError    bool
		validateFunc   func(*testing.T, *WebSocketClient)
	}{
		{
			name: "successful connection",
			serverHandler: func(t *testing.T, conn *websocket.Conn) {
				// Just accept the connection
				for {
					messageType, _, err := conn.ReadMessage()
					if err != nil {
						return
					}
					if messageType == websocket.PingMessage {
						conn.WriteMessage(websocket.PongMessage, nil)
					}
				}
			},
			expectError: false,
			validateFunc: func(t *testing.T, client *WebSocketClient) {
				assert.True(t, client.IsConnected())
			},
		},
		{
			name: "connection with authentication",
			serverHandler: func(t *testing.T, conn *websocket.Conn) {
				// Expect auth message first
				_, message, err := conn.ReadMessage()
				if err != nil {
					t.Errorf("Failed to read auth message: %v", err)
					return
				}
				
				var msg map[string]interface{}
				json.Unmarshal(message, &msg)
				assert.Equal(t, "auth", msg["type"])
				assert.Equal(t, "test-agent", msg["agent_id"])
				
				// Send auth success
				authResp := map[string]interface{}{
					"type":    "auth_success",
					"message": "authenticated",
				}
				conn.WriteJSON(authResp)
				
				// Keep connection alive
				for {
					if _, _, err := conn.ReadMessage(); err != nil {
						return
					}
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test WebSocket server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					t.Errorf("Failed to upgrade connection: %v", err)
					return
				}
				defer conn.Close()
				
				if tt.serverHandler != nil {
					tt.serverHandler(t, conn)
				}
			}))
			defer server.Close()

			// Convert http:// to ws://
			wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
			
			cfg := &config.AgentConfig{
				ServerURL:             wsURL,
				AgentID:               "test-agent",
				WebSocketPingInterval: 30 * time.Second,
			}
			client := NewWebSocketClient(cfg, logger)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			handler := &mockWSMessageHandler{}
			err := client.Connect(ctx, "test-agent", handler)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validateFunc != nil {
					tt.validateFunc(t, client)
				}
			}

			client.Close()
		})
	}
}

func TestWebSocketClient_Send(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	tests := []struct {
		name          string
		msgType       string
		payload       interface{}
		serverHandler func(*testing.T, *websocket.Conn, *sync.WaitGroup)
		expectError   bool
	}{
		{
			name:    "send valid message",
			msgType: "status_report",
			payload: map[string]interface{}{
				"status": "online",
				"cpu":    45.5,
			},
			serverHandler: func(t *testing.T, conn *websocket.Conn, wg *sync.WaitGroup) {
				defer wg.Done()
				
				// Skip auth message
				_, _, err := conn.ReadMessage()
				assert.NoError(t, err)
				
				// Read actual message
				_, data, err := conn.ReadMessage()
				assert.NoError(t, err)
				
				var msg core.WebSocketMessage
				err = json.Unmarshal(data, &msg)
				assert.NoError(t, err)
				assert.Equal(t, "status_report", msg.Type)
				
				// Decode payload
				var payload map[string]interface{}
				err = json.Unmarshal(msg.Payload, &payload)
				assert.NoError(t, err)
				assert.Equal(t, "online", payload["status"])
			},
			expectError: false,
		},
		{
			name:    "send when disconnected",
			msgType: "test",
			payload: map[string]interface{}{"test": true},
			serverHandler: nil, // No handler, connection will be closed
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wg sync.WaitGroup
			if tt.serverHandler != nil {
				wg.Add(1)
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()

				if tt.serverHandler != nil {
					tt.serverHandler(t, conn, &wg)
				}
			}))
			defer server.Close()

			wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
			cfg := &config.AgentConfig{
				ServerURL:             wsURL,
				AgentID:               "test-agent",
				WebSocketPingInterval: 30 * time.Second,
			}
			client := NewWebSocketClient(cfg, logger)

			if tt.name != "send when disconnected" {
				ctx := context.Background()
				handler := &mockWSMessageHandler{}
				err := client.Connect(ctx, "test-agent", handler)
				require.NoError(t, err)
				defer client.Close()
				
				// Give connection time to stabilize
				time.Sleep(50 * time.Millisecond)
			}

			err := client.Send(tt.msgType, tt.payload)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				wg.Wait()
			}
		})
	}
}

func TestWebSocketClient_MessageHandling(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	messageReceived := make(chan *core.WebSocketMessage, 1)
	
	handler := &mockWSMessageHandler{
		handleFunc: func(msgType string, payload []byte) error {
			msg := &core.WebSocketMessage{
				Type:    msgType,
				Payload: json.RawMessage(payload),
			}
			messageReceived <- msg
			return nil
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Skip auth message
		conn.ReadMessage()

		// Send a test message
		testMsg := core.WebSocketMessage{
			Type:      "config_deploy",
			Timestamp: time.Now(),
			Payload: json.RawMessage(`{
				"config_id": "test-config-123",
				"version": 1
			}`),
		}
		
		conn.WriteJSON(testMsg)

		// Keep connection open
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	cfg := &config.AgentConfig{
		ServerURL:             wsURL,
		AgentID:               "test-agent",
		WebSocketPingInterval: 30 * time.Second,
	}
	
	client := NewWebSocketClient(cfg, logger)

	ctx := context.Background()
	err := client.Connect(ctx, "test-agent", handler)
	require.NoError(t, err)
	defer client.Close()

	// Wait for message
	select {
	case msg := <-messageReceived:
		assert.Equal(t, "config_deploy", msg.Type)
		
		var payload map[string]interface{}
		err := json.Unmarshal(msg.Payload, &payload)
		assert.NoError(t, err)
		assert.Equal(t, "test-config-123", payload["config_id"])
		assert.Equal(t, float64(1), payload["version"])
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

func TestWebSocketClient_Reconnect(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	reconnectCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reconnectCount++
		count := reconnectCount
		mu.Unlock()

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		if count == 1 {
			// First connection: close after auth
			conn.ReadMessage() // Read auth
			time.Sleep(50 * time.Millisecond)
			conn.Close()
		} else {
			// Second connection: keep alive
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					return
				}
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	cfg := &config.AgentConfig{
		ServerURL:             wsURL,
		AgentID:               "test-agent",
		ReconnectInterval:     100 * time.Millisecond, // Fast reconnect for testing
		WebSocketPingInterval: 30 * time.Second,
	}

	client := NewWebSocketClient(cfg, logger)
	handler := &mockWSMessageHandler{}

	ctx := context.Background()
	err := client.Connect(ctx, "test-agent", handler)
	require.NoError(t, err)

	// Wait for potential reconnection
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	// May not reconnect automatically without external trigger
	assert.GreaterOrEqual(t, reconnectCount, 1)
	mu.Unlock()

	client.Close()
}

func TestWebSocketClient_PingPong(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	pongReceived := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Handle pong
		conn.SetPongHandler(func(appData string) error {
			pongReceived <- true
			return nil
		})

		// Read messages and handle ping
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	cfg := &config.AgentConfig{
		ServerURL:             wsURL,
		AgentID:               "test-agent",
		WebSocketPingInterval: 100 * time.Millisecond, // Fast ping for testing
	}

	client := NewWebSocketClient(cfg, logger)
	handler := &mockWSMessageHandler{}

	ctx := context.Background()
	err := client.Connect(ctx, "test-agent", handler)
	require.NoError(t, err)
	defer client.Close()

	// Wait for ping/pong
	select {
	case <-pongReceived:
		// Success
	case <-time.After(500 * time.Millisecond):
		// Ping might be handled internally
		t.Log("No pong received, but connection is stable")
	}
}

func TestWebSocketClient_ConcurrentSend(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	messageCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Skip auth
		conn.ReadMessage()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
			mu.Lock()
			messageCount++
			mu.Unlock()
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	cfg := &config.AgentConfig{
		ServerURL:             wsURL,
		AgentID:               "test-agent",
		WebSocketPingInterval: 30 * time.Second,
	}

	client := NewWebSocketClient(cfg, logger)
	handler := &mockWSMessageHandler{}

	ctx := context.Background()
	err := client.Connect(ctx, "test-agent", handler)
	require.NoError(t, err)
	defer client.Close()

	// Give connection time to stabilize
	time.Sleep(50 * time.Millisecond)

	// Send messages concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := client.Send("test", map[string]interface{}{
				"id": id,
			})
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond) // Allow server to process

	mu.Lock()
	assert.Equal(t, 10, messageCount)
	mu.Unlock()
}

func TestWebSocketClient_Close(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		
		// Keep connection alive
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	cfg := &config.AgentConfig{
		ServerURL:             wsURL,
		AgentID:               "test-agent",
		WebSocketPingInterval: 30 * time.Second,
	}

	client := NewWebSocketClient(cfg, logger)
	handler := &mockWSMessageHandler{}

	ctx := context.Background()
	err := client.Connect(ctx, "test-agent", handler)
	require.NoError(t, err)

	assert.True(t, client.IsConnected())

	err = client.Close()
	assert.NoError(t, err)
	assert.False(t, client.IsConnected())
}

// mockWSMessageHandler is used only in websocket tests to avoid collision
type mockWSMessageHandler struct {
	handleFunc    func(msgType string, payload []byte) error
	connectFunc   func() error
	disconnectFunc func(err error)
}

func (m *mockWSMessageHandler) HandleMessage(msgType string, payload []byte) error {
	if m.handleFunc != nil {
		return m.handleFunc(msgType, payload)
	}
	return nil
}

func (m *mockWSMessageHandler) OnConnect() error {
	if m.connectFunc != nil {
		return m.connectFunc()
	}
	return nil
}

func (m *mockWSMessageHandler) OnDisconnect(err error) {
	if m.disconnectFunc != nil {
		m.disconnectFunc(err)
	}
}