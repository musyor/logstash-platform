package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"logstash-platform/internal/agent/config"
	"logstash-platform/internal/platform/models"
)

// Agent 实现AgentCore接口
type Agent struct {
	config      *config.AgentConfig
	logger      *logrus.Logger
	
	// 核心组件
	apiClient    APIClient
	configMgr    ConfigManager
	logstashCtrl LogstashController
	heartbeat    HeartbeatService
	metrics      MetricsCollector
	
	// 状态信息
	status       *models.Agent
	statusMutex  sync.RWMutex
	
	// 生命周期管理
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	
	// WebSocket消息通道
	msgChan      chan *WebSocketMessage
	
	// 启动时间
	startTime    time.Time
}

// NewAgent 创建新的Agent实例
func NewAgent(cfg *config.AgentConfig, logger *logrus.Logger) (*Agent, error) {
	// 获取本机IP
	ip, err := getLocalIP()
	if err != nil {
		logger.WithError(err).Warn("获取本机IP失败，使用127.0.0.1")
		ip = "127.0.0.1"
	}
	
	// 获取主机名
	hostname, err := getHostname()
	if err != nil {
		logger.WithError(err).Warn("获取主机名失败")
		hostname = "unknown"
	}
	
	// 初始化Agent状态
	agent := &Agent{
		config:    cfg,
		logger:    logger,
		msgChan:   make(chan *WebSocketMessage, 100),
		startTime: time.Now(),
		status: &models.Agent{
			AgentID:         cfg.AgentID,
			Hostname:        hostname,
			IP:              ip,
			LogstashVersion: "unknown", // 将在启动时获取
			Status:          "offline",
			LastHeartbeat:   time.Now(),
			AppliedConfigs:  []models.AppliedConfig{},
		},
	}
	
	return agent, nil
}

// WithAPIClient 设置API客户端
func (a *Agent) WithAPIClient(client APIClient) *Agent {
	a.apiClient = client
	return a
}

// WithConfigManager 设置配置管理器
func (a *Agent) WithConfigManager(mgr ConfigManager) *Agent {
	a.configMgr = mgr
	return a
}

// WithLogstashController 设置Logstash控制器
func (a *Agent) WithLogstashController(ctrl LogstashController) *Agent {
	a.logstashCtrl = ctrl
	return a
}

// WithHeartbeatService 设置心跳服务
func (a *Agent) WithHeartbeatService(service HeartbeatService) *Agent {
	a.heartbeat = service
	return a
}

// WithMetricsCollector 设置指标收集器
func (a *Agent) WithMetricsCollector(collector MetricsCollector) *Agent {
	a.metrics = collector
	return a
}

// Start 启动Agent
func (a *Agent) Start(ctx context.Context) error {
	a.logger.Info("正在启动Agent...")
	
	// 创建带取消的上下文
	a.ctx, a.cancel = context.WithCancel(ctx)
	
	// 验证组件
	if err := a.validateComponents(); err != nil {
		return fmt.Errorf("组件验证失败: %w", err)
	}
	
	// 注册到管理平台
	if err := a.Register(a.ctx); err != nil {
		return fmt.Errorf("注册到管理平台失败: %w", err)
	}
	
	// 启动Logstash
	if err := a.logstashCtrl.Start(a.ctx); err != nil {
		a.logger.WithError(err).Error("启动Logstash失败")
		// 不返回错误，允许Agent继续运行
	}
	
	// 获取Logstash版本
	if status, err := a.logstashCtrl.GetStatus(); err == nil {
		a.updateStatus(func(s *models.Agent) {
			s.LogstashVersion = status.Version
		})
	}
	
	// 启动心跳服务
	if err := a.heartbeat.Start(a.ctx); err != nil {
		return fmt.Errorf("启动心跳服务失败: %w", err)
	}
	
	// 启动指标收集
	if err := a.metrics.Start(a.ctx); err != nil {
		a.logger.WithError(err).Error("启动指标收集失败")
		// 不返回错误，指标收集是可选功能
	}
	
	// 启动WebSocket连接（如果启用）
	if a.config.EnableWebSocket {
		a.wg.Add(1)
		go a.connectWebSocket()
	}
	
	// 启动消息处理
	a.wg.Add(1)
	go a.processMessages()
	
	// 更新状态为在线
	a.updateStatus(func(s *models.Agent) {
		s.Status = "online"
		s.LastHeartbeat = time.Now()
	})
	
	a.logger.Info("Agent启动成功")
	return nil
}

// Stop 停止Agent
func (a *Agent) Stop(ctx context.Context) error {
	a.logger.Info("正在停止Agent...")
	
	// 更新状态为离线
	a.updateStatus(func(s *models.Agent) {
		s.Status = "offline"
	})
	
	// 发送最后的状态更新
	if a.apiClient != nil {
		if err := a.apiClient.ReportStatus(ctx, a.GetStatus()); err != nil {
			a.logger.WithError(err).Error("发送离线状态失败")
		}
	}
	
	// 取消上下文
	if a.cancel != nil {
		a.cancel()
	}
	
	// 停止心跳服务
	if a.heartbeat != nil {
		if err := a.heartbeat.Stop(); err != nil {
			a.logger.WithError(err).Error("停止心跳服务失败")
		}
	}
	
	// 停止指标收集
	if a.metrics != nil {
		if err := a.metrics.Stop(); err != nil {
			a.logger.WithError(err).Error("停止指标收集失败")
		}
	}
	
	// 停止Logstash
	if a.logstashCtrl != nil {
		if err := a.logstashCtrl.Stop(ctx); err != nil {
			a.logger.WithError(err).Error("停止Logstash失败")
		}
	}
	
	// 关闭API客户端
	if a.apiClient != nil {
		if err := a.apiClient.Close(); err != nil {
			a.logger.WithError(err).Error("关闭API客户端失败")
		}
	}
	
	// 关闭消息通道
	close(a.msgChan)
	
	// 等待所有goroutine结束
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		a.logger.Info("Agent已停止")
		return nil
	case <-ctx.Done():
		a.logger.Warn("停止Agent超时")
		return ctx.Err()
	}
}

// Register 注册到管理平台
func (a *Agent) Register(ctx context.Context) error {
	a.logger.Info("正在注册到管理平台...")
	
	// 发送注册请求
	if err := a.apiClient.Register(ctx, a.GetStatus()); err != nil {
		return fmt.Errorf("注册请求失败: %w", err)
	}
	
	a.logger.WithField("agent_id", a.config.AgentID).Info("注册成功")
	return nil
}

// GetStatus 获取Agent状态
func (a *Agent) GetStatus() *models.Agent {
	a.statusMutex.RLock()
	defer a.statusMutex.RUnlock()
	
	// 创建状态副本
	status := *a.status
	
	// 复制切片
	status.AppliedConfigs = make([]models.AppliedConfig, len(a.status.AppliedConfigs))
	copy(status.AppliedConfigs, a.status.AppliedConfigs)
	
	return &status
}

// validateComponents 验证组件
func (a *Agent) validateComponents() error {
	if a.apiClient == nil {
		return fmt.Errorf("API客户端未设置")
	}
	if a.configMgr == nil {
		return fmt.Errorf("配置管理器未设置")
	}
	if a.logstashCtrl == nil {
		return fmt.Errorf("Logstash控制器未设置")
	}
	if a.heartbeat == nil {
		return fmt.Errorf("心跳服务未设置")
	}
	if a.metrics == nil {
		return fmt.Errorf("指标收集器未设置")
	}
	return nil
}

// connectWebSocket 连接WebSocket
func (a *Agent) connectWebSocket() {
	defer a.wg.Done()
	
	retryCount := 0
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
		}
		
		a.logger.Info("正在连接WebSocket...")
		
		// 连接WebSocket
		err := a.apiClient.ConnectWebSocket(a.ctx, a.config.AgentID, a)
		if err == nil {
			// 连接成功，重置重试计数
			retryCount = 0
			a.logger.Info("WebSocket连接成功")
			
			// 等待连接断开
			select {
			case <-a.ctx.Done():
				return
			}
		} else {
			// 连接失败
			retryCount++
			a.logger.WithError(err).WithField("retry_count", retryCount).Error("WebSocket连接失败")
			
			// 检查是否超过最大重试次数
			if a.config.MaxReconnectAttempts > 0 && retryCount >= a.config.MaxReconnectAttempts {
				a.logger.Error("WebSocket重连次数超过限制，停止重连")
				return
			}
			
			// 等待一段时间后重试
			select {
			case <-time.After(a.config.ReconnectInterval):
			case <-a.ctx.Done():
				return
			}
		}
	}
}

// processMessages 处理消息
func (a *Agent) processMessages() {
	defer a.wg.Done()
	
	for {
		select {
		case msg, ok := <-a.msgChan:
			if !ok {
				return
			}
			
			if err := a.handleMessage(msg); err != nil {
				a.logger.WithError(err).WithField("msg_type", msg.Type).Error("处理消息失败")
			}
			
		case <-a.ctx.Done():
			return
		}
	}
}

// handleMessage 处理单个消息
func (a *Agent) handleMessage(msg *WebSocketMessage) error {
	a.logger.WithFields(logrus.Fields{
		"type":      msg.Type,
		"timestamp": msg.Timestamp,
	}).Debug("处理消息")
	
	switch msg.Type {
	case MsgTypeConfigDeploy:
		return a.handleConfigDeploy(msg.Payload)
	case MsgTypeConfigDelete:
		return a.handleConfigDelete(msg.Payload)
	case MsgTypeReloadRequest:
		return a.handleReloadRequest()
	case MsgTypeStatusRequest:
		return a.handleStatusRequest()
	case MsgTypeMetricsRequest:
		return a.handleMetricsRequest()
	default:
		return fmt.Errorf("未知消息类型: %s", msg.Type)
	}
}

// HandleMessage 实现MessageHandler接口
func (a *Agent) HandleMessage(msgType string, payload []byte) error {
	msg := &WebSocketMessage{
		Type:      msgType,
		Timestamp: time.Now(),
		Payload:   json.RawMessage(payload),
	}
	
	select {
	case a.msgChan <- msg:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("消息队列已满")
	}
}

// OnConnect 实现MessageHandler接口
func (a *Agent) OnConnect() error {
	a.logger.Info("WebSocket连接建立")
	
	// 更新状态
	a.updateStatus(func(s *models.Agent) {
		s.Status = "online"
		s.LastHeartbeat = time.Now()
	})
	
	// 发送初始状态
	return a.handleStatusRequest()
}

// OnDisconnect 实现MessageHandler接口
func (a *Agent) OnDisconnect(err error) {
	if err != nil {
		a.logger.WithError(err).Warn("WebSocket连接断开")
	} else {
		a.logger.Info("WebSocket连接正常关闭")
	}
}

// 消息处理方法
func (a *Agent) handleConfigDeploy(payload json.RawMessage) error {
	// 解析配置部署请求
	var req struct {
		ConfigID string `json:"config_id"`
		Version  int    `json:"version"`
	}
	
	if err := json.Unmarshal(payload, &req); err != nil {
		return fmt.Errorf("解析配置部署请求失败: %w", err)
	}
	
	a.logger.WithFields(logrus.Fields{
		"config_id": req.ConfigID,
		"version":   req.Version,
	}).Info("收到配置部署请求")
	
	// 获取配置内容
	config, err := a.apiClient.GetConfig(a.ctx, req.ConfigID)
	if err != nil {
		return fmt.Errorf("获取配置失败: %w", err)
	}
	
	// 保存配置
	if err := a.configMgr.SaveConfig(config); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}
	
	// 重载Logstash
	if a.config.EnableAutoReload && a.logstashCtrl.IsRunning() {
		if err := a.logstashCtrl.Reload(a.ctx); err != nil {
			a.logger.WithError(err).Error("重载Logstash失败")
			// 不返回错误，允许继续
		}
	}
	
	// 更新已应用配置
	applied := models.AppliedConfig{
		ConfigID:  req.ConfigID,
		Version:   req.Version,
		AppliedAt: time.Now(),
	}
	
	a.updateStatus(func(s *models.Agent) {
		// 检查是否已存在
		found := false
		for i, ac := range s.AppliedConfigs {
			if ac.ConfigID == req.ConfigID {
				s.AppliedConfigs[i] = applied
				found = true
				break
			}
		}
		if !found {
			s.AppliedConfigs = append(s.AppliedConfigs, applied)
		}
	})
	
	// 上报配置应用结果
	return a.apiClient.ReportConfigApplied(a.ctx, a.config.AgentID, &applied)
}

func (a *Agent) handleConfigDelete(payload json.RawMessage) error {
	// 解析配置删除请求
	var req struct {
		ConfigID string `json:"config_id"`
	}
	
	if err := json.Unmarshal(payload, &req); err != nil {
		return fmt.Errorf("解析配置删除请求失败: %w", err)
	}
	
	a.logger.WithField("config_id", req.ConfigID).Info("收到配置删除请求")
	
	// 删除配置
	if err := a.configMgr.DeleteConfig(req.ConfigID); err != nil {
		return fmt.Errorf("删除配置失败: %w", err)
	}
	
	// 更新状态
	a.updateStatus(func(s *models.Agent) {
		// 从已应用配置中移除
		newConfigs := make([]models.AppliedConfig, 0, len(s.AppliedConfigs))
		for _, ac := range s.AppliedConfigs {
			if ac.ConfigID != req.ConfigID {
				newConfigs = append(newConfigs, ac)
			}
		}
		s.AppliedConfigs = newConfigs
	})
	
	// 重载Logstash
	if a.config.EnableAutoReload && a.logstashCtrl.IsRunning() {
		if err := a.logstashCtrl.Reload(a.ctx); err != nil {
			a.logger.WithError(err).Error("重载Logstash失败")
		}
	}
	
	return nil
}

func (a *Agent) handleReloadRequest() error {
	a.logger.Info("收到重载请求")
	
	if !a.logstashCtrl.IsRunning() {
		return fmt.Errorf("Logstash未运行")
	}
	
	// 执行重载
	if err := a.logstashCtrl.Reload(a.ctx); err != nil {
		return fmt.Errorf("重载失败: %w", err)
	}
	
	a.logger.Info("Logstash重载成功")
	return nil
}

func (a *Agent) handleStatusRequest() error {
	a.logger.Debug("收到状态请求")
	
	// 获取当前状态
	status := a.GetStatus()
	
	// 直接上报状态
	if err := a.apiClient.ReportStatus(a.ctx, status); err != nil {
		return fmt.Errorf("上报状态失败: %w", err)
	}
	
	return nil
}

func (a *Agent) handleMetricsRequest() error {
	a.logger.Debug("收到指标请求")
	
	// 获取当前指标
	metrics, err := a.metrics.GetMetrics()
	if err != nil {
		return fmt.Errorf("获取指标失败: %w", err)
	}
	
	// 上报指标
	if err := a.apiClient.ReportMetrics(a.ctx, a.config.AgentID, metrics); err != nil {
		return fmt.Errorf("上报指标失败: %w", err)
	}
	
	return nil
}

// updateStatus 更新Agent状态
func (a *Agent) updateStatus(updater func(*models.Agent)) {
	a.statusMutex.Lock()
	defer a.statusMutex.Unlock()
	updater(a.status)
}

// 辅助函数

// getLocalIP 获取本机IP
func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	
	return "", fmt.Errorf("未找到有效的IP地址")
}

// getHostname 获取主机名
func getHostname() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return hostname, nil
}