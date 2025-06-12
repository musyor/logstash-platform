package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"logstash-platform/internal/agent/config"
	"logstash-platform/internal/agent/core"
)

// WebSocketClient WebSocket客户端实现
type WebSocketClient struct {
	config    *config.AgentConfig
	logger    *logrus.Logger
	conn      *websocket.Conn
	handler   core.MessageHandler
	
	// 连接管理
	mu        sync.RWMutex
	connected bool
	closeChan chan struct{}
	
	// Ping/Pong管理
	pingTicker *time.Ticker
	lastPong   time.Time
	
	// 重连管理
	reconnectChan chan struct{}
}

// NewWebSocketClient 创建WebSocket客户端
func NewWebSocketClient(cfg *config.AgentConfig, logger *logrus.Logger) *WebSocketClient {
	return &WebSocketClient{
		config:        cfg,
		logger:        logger,
		closeChan:     make(chan struct{}),
		reconnectChan: make(chan struct{}, 1),
	}
}

// Connect 连接WebSocket
func (c *WebSocketClient) Connect(ctx context.Context, agentID string, handler core.MessageHandler) error {
	c.handler = handler
	
	// 构建WebSocket URL
	wsURL, err := c.buildWebSocketURL(agentID)
	if err != nil {
		return fmt.Errorf("构建WebSocket URL失败: %w", err)
	}
	
	// 创建WebSocket拨号器
	dialer := c.createDialer()
	
	// 设置请求头
	headers := http.Header{
		"User-Agent": []string{fmt.Sprintf("LogstashAgent/%s", agentID)},
	}
	if c.config.Token != "" {
		headers.Set("Authorization", "Bearer "+c.config.Token)
	}
	
	// 连接WebSocket
	c.logger.WithField("url", wsURL).Info("正在连接WebSocket...")
	conn, resp, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		if resp != nil {
			defer resp.Body.Close()
			return fmt.Errorf("WebSocket连接失败 (HTTP %d): %w", resp.StatusCode, err)
		}
		return fmt.Errorf("WebSocket连接失败: %w", err)
	}
	
	// 保存连接
	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.lastPong = time.Now()
	c.mu.Unlock()
	
	// 设置Pong处理器
	c.conn.SetPongHandler(func(string) error {
		c.mu.Lock()
		c.lastPong = time.Now()
		c.mu.Unlock()
		c.logger.Debug("收到Pong")
		return nil
	})
	
	// 设置读取超时
	c.conn.SetReadDeadline(time.Now().Add(c.config.WebSocketPingInterval * 2))
	
	// 通知连接成功
	if err := handler.OnConnect(); err != nil {
		c.logger.WithError(err).Error("处理连接事件失败")
	}
	
	// 启动读写循环
	go c.readLoop()
	go c.pingLoop()
	
	// 等待关闭
	select {
	case <-ctx.Done():
		return c.Close()
	case <-c.closeChan:
		return nil
	}
}

// Send 发送消息
func (c *WebSocketClient) Send(msgType string, payload interface{}) error {
	c.mu.RLock()
	if !c.connected || c.conn == nil {
		c.mu.RUnlock()
		return fmt.Errorf("WebSocket未连接")
	}
	conn := c.conn
	c.mu.RUnlock()
	
	// 构建消息
	msg := core.WebSocketMessage{
		Type:      msgType,
		Timestamp: time.Now(),
	}
	
	// 序列化payload
	if payload != nil {
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("序列化payload失败: %w", err)
		}
		msg.Payload = json.RawMessage(payloadBytes)
	}
	
	// 序列化消息
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}
	
	// 发送消息
	c.logger.WithFields(logrus.Fields{
		"type": msgType,
		"size": len(msgBytes),
	}).Debug("发送WebSocket消息")
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// 设置写入超时
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	
	if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		return fmt.Errorf("发送消息失败: %w", err)
	}
	
	return nil
}

// Close 关闭连接
func (c *WebSocketClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if !c.connected {
		return nil
	}
	
	c.connected = false
	close(c.closeChan)
	
	// 停止Ping
	if c.pingTicker != nil {
		c.pingTicker.Stop()
	}
	
	// 发送关闭消息
	if c.conn != nil {
		// 发送关闭帧
		deadline := time.Now().Add(5 * time.Second)
		c.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			deadline,
		)
		
		// 关闭连接
		if err := c.conn.Close(); err != nil {
			c.logger.WithError(err).Warn("关闭WebSocket连接失败")
		}
	}
	
	// 通知断开
	if c.handler != nil {
		c.handler.OnDisconnect(nil)
	}
	
	c.logger.Info("WebSocket连接已关闭")
	return nil
}

// IsConnected 检查是否已连接
func (c *WebSocketClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// readLoop 读取消息循环
func (c *WebSocketClient) readLoop() {
	defer func() {
		c.handleDisconnect(fmt.Errorf("读取循环结束"))
	}()
	
	for {
		// 检查是否已关闭
		select {
		case <-c.closeChan:
			return
		default:
		}
		
		// 读取消息
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.WithError(err).Error("读取WebSocket消息失败")
			}
			return
		}
		
		// 更新读取超时
		c.conn.SetReadDeadline(time.Now().Add(c.config.WebSocketPingInterval * 2))
		
		// 处理文本消息
		if messageType == websocket.TextMessage {
			c.handleMessage(message)
		}
	}
}

// pingLoop Ping循环
func (c *WebSocketClient) pingLoop() {
	c.pingTicker = time.NewTicker(c.config.WebSocketPingInterval)
	defer c.pingTicker.Stop()
	
	for {
		select {
		case <-c.closeChan:
			return
		case <-c.pingTicker.C:
			c.mu.Lock()
			if !c.connected || c.conn == nil {
				c.mu.Unlock()
				return
			}
			
			// 检查最后Pong时间
			if time.Since(c.lastPong) > c.config.WebSocketPingInterval*2 {
				c.mu.Unlock()
				c.logger.Warn("Pong超时，断开连接")
				c.handleDisconnect(fmt.Errorf("pong超时"))
				return
			}
			
			// 发送Ping
			c.logger.Debug("发送Ping")
			deadline := time.Now().Add(5 * time.Second)
			if err := c.conn.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				c.mu.Unlock()
				c.logger.WithError(err).Error("发送Ping失败")
				c.handleDisconnect(err)
				return
			}
			c.mu.Unlock()
		}
	}
}

// handleMessage 处理消息
func (c *WebSocketClient) handleMessage(data []byte) {
	// 解析消息
	var msg core.WebSocketMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.logger.WithError(err).Error("解析WebSocket消息失败")
		return
	}
	
	// 记录消息
	c.logger.WithFields(logrus.Fields{
		"type":      msg.Type,
		"timestamp": msg.Timestamp,
	}).Debug("收到WebSocket消息")
	
	// 处理消息
	if c.handler != nil {
		if err := c.handler.HandleMessage(msg.Type, msg.Payload); err != nil {
			c.logger.WithError(err).WithField("type", msg.Type).Error("处理消息失败")
			
			// 发送错误响应
			c.Send(core.MsgTypeError, map[string]interface{}{
				"error":    err.Error(),
				"msg_type": msg.Type,
			})
		}
	}
}

// handleDisconnect 处理断开连接
func (c *WebSocketClient) handleDisconnect(err error) {
	c.mu.Lock()
	wasConnected := c.connected
	c.connected = false
	c.mu.Unlock()
	
	if wasConnected {
		c.logger.WithError(err).Warn("WebSocket连接断开")
		
		// 通知处理器
		if c.handler != nil {
			c.handler.OnDisconnect(err)
		}
		
		// 触发重连
		select {
		case c.reconnectChan <- struct{}{}:
		default:
		}
	}
}

// buildWebSocketURL 构建WebSocket URL
func (c *WebSocketClient) buildWebSocketURL(agentID string) (string, error) {
	// 解析服务器URL
	u, err := url.Parse(c.config.ServerURL)
	if err != nil {
		return "", fmt.Errorf("解析服务器URL失败: %w", err)
	}
	
	// 转换协议
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
		// 已经是WebSocket协议，保持不变
	default:
		return "", fmt.Errorf("不支持的协议: %s", u.Scheme)
	}
	
	// 设置路径
	u.Path = "/ws"
	
	// 添加查询参数
	q := u.Query()
	q.Set("agent_id", agentID)
	u.RawQuery = q.Encode()
	
	return u.String(), nil
}

// createDialer 创建WebSocket拨号器
func (c *WebSocketClient) createDialer() *websocket.Dialer {
	dialer := &websocket.Dialer{
		HandshakeTimeout: c.config.RequestTimeout,
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
	}
	
	// 配置TLS
	if c.config.TLSEnabled {
		tlsConfig, err := createTLSConfig(c.config)
		if err != nil {
			c.logger.WithError(err).Warn("创建TLS配置失败，使用默认配置")
			tlsConfig = &tls.Config{
				InsecureSkipVerify: c.config.TLSSkipVerify,
			}
		}
		dialer.TLSClientConfig = tlsConfig
	}
	
	return dialer
}