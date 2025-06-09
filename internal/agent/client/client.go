package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
	"logstash-platform/internal/agent/config"
	"logstash-platform/internal/agent/core"
	"logstash-platform/internal/platform/models"
)

// Client 统一的API客户端，实现core.APIClient接口
type Client struct {
	config      *config.AgentConfig
	logger      *logrus.Logger
	httpClient  *HTTPClient
	wsClient    *WebSocketClient
	
	// WebSocket状态
	wsConnected bool
	wsMutex     sync.RWMutex
	wsHandler   core.MessageHandler
}

// NewClient 创建统一的API客户端
func NewClient(cfg *config.AgentConfig, logger *logrus.Logger) (*Client, error) {
	// 创建HTTP客户端
	httpClient, err := NewHTTPClient(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP客户端失败: %w", err)
	}
	
	// 创建WebSocket客户端
	wsClient := NewWebSocketClient(cfg, logger)
	
	client := &Client{
		config:     cfg,
		logger:     logger,
		httpClient: httpClient,
		wsClient:   wsClient,
	}
	
	return client, nil
}

// Register 注册Agent
func (c *Client) Register(ctx context.Context, agent *models.Agent) error {
	return c.httpClient.Register(ctx, agent)
}

// SendHeartbeat 发送心跳
func (c *Client) SendHeartbeat(ctx context.Context, agentID string) error {
	// 优先使用WebSocket发送心跳
	if c.isWebSocketConnected() {
		err := c.wsClient.Send(core.MsgTypeHeartbeat, map[string]interface{}{
			"agent_id":  agentID,
			"timestamp": ctx.Value("timestamp"), // 如果上下文中有时间戳
		})
		if err == nil {
			return nil
		}
		c.logger.WithError(err).Debug("WebSocket发送心跳失败，降级到HTTP")
	}
	
	// 降级到HTTP
	return c.httpClient.SendHeartbeat(ctx, agentID)
}

// ReportStatus 上报状态
func (c *Client) ReportStatus(ctx context.Context, agent *models.Agent) error {
	// 优先使用WebSocket上报状态
	if c.isWebSocketConnected() {
		err := c.wsClient.Send(core.MsgTypeStatusReport, agent)
		if err == nil {
			return nil
		}
		c.logger.WithError(err).Debug("WebSocket上报状态失败，降级到HTTP")
	}
	
	// 降级到HTTP
	return c.httpClient.ReportStatus(ctx, agent)
}

// GetConfig 获取配置
func (c *Client) GetConfig(ctx context.Context, configID string) (*models.Config, error) {
	// 配置获取始终使用HTTP，因为需要返回值
	return c.httpClient.GetConfig(ctx, configID)
}

// ReportConfigApplied 上报配置应用结果
func (c *Client) ReportConfigApplied(ctx context.Context, agentID string, applied *models.AppliedConfig) error {
	// 优先使用WebSocket上报
	if c.isWebSocketConnected() {
		err := c.wsClient.Send(core.MsgTypeConfigApplied, map[string]interface{}{
			"agent_id":    agentID,
			"config_id":   applied.ConfigID,
			"version":     applied.Version,
			"applied_at":  applied.AppliedAt,
		})
		if err == nil {
			return nil
		}
		c.logger.WithError(err).Debug("WebSocket上报配置应用结果失败，降级到HTTP")
	}
	
	// 降级到HTTP
	return c.httpClient.ReportConfigApplied(ctx, agentID, applied)
}

// ConnectWebSocket 建立WebSocket连接
func (c *Client) ConnectWebSocket(ctx context.Context, agentID string, handler core.MessageHandler) error {
	c.wsHandler = handler
	
	// 包装handler以更新连接状态
	wrappedHandler := &wsHandlerWrapper{
		handler: handler,
		client:  c,
	}
	
	// 连接WebSocket
	err := c.wsClient.Connect(ctx, agentID, wrappedHandler)
	if err != nil {
		c.setWebSocketConnected(false)
		return err
	}
	
	return nil
}

// ReportMetrics 上报指标
func (c *Client) ReportMetrics(ctx context.Context, agentID string, metrics *core.AgentMetrics) error {
	// 优先使用WebSocket上报
	if c.isWebSocketConnected() {
		err := c.wsClient.Send(core.MsgTypeMetricsReport, map[string]interface{}{
			"agent_id": agentID,
			"metrics":  metrics,
		})
		if err == nil {
			return nil
		}
		c.logger.WithError(err).Debug("WebSocket上报指标失败，降级到HTTP")
	}
	
	// 降级到HTTP
	return c.httpClient.ReportMetrics(ctx, agentID, metrics)
}

// SendMessage 发送自定义消息（仅WebSocket）
func (c *Client) SendMessage(msgType string, payload interface{}) error {
	if !c.isWebSocketConnected() {
		return fmt.Errorf("WebSocket未连接")
	}
	
	return c.wsClient.Send(msgType, payload)
}

// Close 关闭客户端
func (c *Client) Close() error {
	var errs []error
	
	// 关闭WebSocket
	if c.wsClient != nil {
		if err := c.wsClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭WebSocket失败: %w", err))
		}
	}
	
	// 关闭HTTP客户端
	if c.httpClient != nil {
		if err := c.httpClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭HTTP客户端失败: %w", err))
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("关闭客户端时出错: %v", errs)
	}
	
	return nil
}

// isWebSocketConnected 检查WebSocket是否已连接
func (c *Client) isWebSocketConnected() bool {
	c.wsMutex.RLock()
	defer c.wsMutex.RUnlock()
	return c.wsConnected && c.wsClient.IsConnected()
}

// setWebSocketConnected 设置WebSocket连接状态
func (c *Client) setWebSocketConnected(connected bool) {
	c.wsMutex.Lock()
	defer c.wsMutex.Unlock()
	c.wsConnected = connected
}

// wsHandlerWrapper WebSocket处理器包装器
type wsHandlerWrapper struct {
	handler core.MessageHandler
	client  *Client
}

// HandleMessage 处理消息
func (w *wsHandlerWrapper) HandleMessage(msgType string, payload []byte) error {
	return w.handler.HandleMessage(msgType, payload)
}

// OnConnect 连接建立
func (w *wsHandlerWrapper) OnConnect() error {
	w.client.setWebSocketConnected(true)
	w.client.logger.Info("WebSocket连接已建立")
	return w.handler.OnConnect()
}

// OnDisconnect 连接断开
func (w *wsHandlerWrapper) OnDisconnect(err error) {
	w.client.setWebSocketConnected(false)
	if err != nil {
		w.client.logger.WithError(err).Warn("WebSocket连接已断开")
	} else {
		w.client.logger.Info("WebSocket连接已正常关闭")
	}
	w.handler.OnDisconnect(err)
}