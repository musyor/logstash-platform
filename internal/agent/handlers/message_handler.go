package handlers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"logstash-platform/internal/agent/client"
	"logstash-platform/internal/agent/core"
	"logstash-platform/internal/platform/models"
)

// MessageHandler WebSocket消息处理器实现
type MessageHandler struct {
	agent            core.AgentCore
	apiClient        core.APIClient
	configManager    core.ConfigManager
	logstashCtrl     core.LogstashController
	metricsCollector core.MetricsCollector
	logger           *logrus.Logger
	agentID          string
}

// NewMessageHandler 创建消息处理器
func NewMessageHandler(
	agent core.AgentCore,
	apiClient core.APIClient,
	configManager core.ConfigManager,
	logstashCtrl core.LogstashController,
	metricsCollector core.MetricsCollector,
	logger *logrus.Logger,
	agentID string,
) *MessageHandler {
	return &MessageHandler{
		agent:            agent,
		apiClient:        apiClient,
		configManager:    configManager,
		logstashCtrl:     logstashCtrl,
		metricsCollector: metricsCollector,
		logger:           logger,
		agentID:          agentID,
	}
}

// HandleMessage 处理接收到的消息
func (h *MessageHandler) HandleMessage(msgType string, payload []byte) error {
	h.logger.WithFields(logrus.Fields{
		"type": msgType,
		"size": len(payload),
	}).Debug("处理WebSocket消息")

	switch msgType {
	case core.MsgTypeConfigDeploy:
		return h.handleConfigDeploy(payload)
	case core.MsgTypeConfigDelete:
		return h.handleConfigDelete(payload)
	case core.MsgTypeReloadRequest:
		return h.handleReloadRequest(payload)
	case core.MsgTypeStatusRequest:
		return h.handleStatusRequest(payload)
	case core.MsgTypeMetricsRequest:
		return h.handleMetricsRequest(payload)
	default:
		h.logger.WithField("type", msgType).Warn("未知的消息类型")
		return fmt.Errorf("未知的消息类型: %s", msgType)
	}
}

// OnConnect 连接建立时调用
func (h *MessageHandler) OnConnect() error {
	h.logger.Info("WebSocket连接已建立")
	
	// 立即上报状态
	status := h.agent.GetStatus()
	return h.apiClient.ReportStatus(nil, status)
}

// OnDisconnect 连接断开时调用
func (h *MessageHandler) OnDisconnect(err error) {
	if err != nil {
		h.logger.WithError(err).Warn("WebSocket连接断开")
	} else {
		h.logger.Info("WebSocket连接已关闭")
	}
}

// handleConfigDeploy 处理配置部署
func (h *MessageHandler) handleConfigDeploy(payload []byte) error {
	var req struct {
		ConfigID string `json:"config_id"`
		Version  int    `json:"version"`
		Force    bool   `json:"force,omitempty"`
	}
	
	if err := json.Unmarshal(payload, &req); err != nil {
		return fmt.Errorf("解析配置部署请求失败: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"config_id": req.ConfigID,
		"version":   req.Version,
		"force":     req.Force,
	}).Info("收到配置部署请求")

	// 获取配置
	config, err := h.apiClient.GetConfig(nil, req.ConfigID)
	if err != nil {
		return fmt.Errorf("获取配置失败: %w", err)
	}

	// 验证版本
	if config.Version != req.Version && !req.Force {
		return fmt.Errorf("配置版本不匹配: 期望 %d, 实际 %d", req.Version, config.Version)
	}

	// 保存配置
	if err := h.configManager.SaveConfig(config); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	// 验证配置
	configPath := h.configManager.GetConfigPath(config.ID)
	if err := h.logstashCtrl.ValidateConfig(configPath); err != nil {
		// 回滚配置
		h.configManager.RestoreConfig(config.ID)
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 重新加载Logstash
	if h.logstashCtrl.IsRunning() {
		if err := h.logstashCtrl.Reload(nil); err != nil {
			// 回滚配置
			h.configManager.RestoreConfig(config.ID)
			return fmt.Errorf("重载配置失败: %w", err)
		}
	}

	// 上报配置应用成功
	applied := &models.AppliedConfig{
		ConfigID:  config.ID,
		Version:   config.Version,
		AppliedAt: time.Now(),
	}
	
	if err := h.apiClient.ReportConfigApplied(nil, h.agentID, applied); err != nil {
		h.logger.WithError(err).Warn("上报配置应用结果失败")
	}

	h.logger.WithField("config_id", config.ID).Info("配置部署成功")
	return nil
}

// handleConfigDelete 处理配置删除
func (h *MessageHandler) handleConfigDelete(payload []byte) error {
	var req struct {
		ConfigID string `json:"config_id"`
	}
	
	if err := json.Unmarshal(payload, &req); err != nil {
		return fmt.Errorf("解析配置删除请求失败: %w", err)
	}

	h.logger.WithField("config_id", req.ConfigID).Info("收到配置删除请求")

	// 删除配置
	if err := h.configManager.DeleteConfig(req.ConfigID); err != nil {
		return fmt.Errorf("删除配置失败: %w", err)
	}

	// 重新加载Logstash
	if h.logstashCtrl.IsRunning() {
		if err := h.logstashCtrl.Reload(nil); err != nil {
			h.logger.WithError(err).Warn("重载配置失败")
		}
	}

	h.logger.WithField("config_id", req.ConfigID).Info("配置删除成功")
	return nil
}

// handleReloadRequest 处理重载请求
func (h *MessageHandler) handleReloadRequest(payload []byte) error {
	h.logger.Info("收到重载请求")

	if !h.logstashCtrl.IsRunning() {
		return fmt.Errorf("Logstash未运行")
	}

	if err := h.logstashCtrl.Reload(nil); err != nil {
		return fmt.Errorf("重载失败: %w", err)
	}

	h.logger.Info("重载成功")
	return nil
}

// handleStatusRequest 处理状态请求
func (h *MessageHandler) handleStatusRequest(payload []byte) error {
	h.logger.Debug("收到状态请求")

	// 获取Agent状态
	status := h.agent.GetStatus()
	
	// 获取Logstash状态
	logstashStatus, err := h.logstashCtrl.GetStatus()
	if err != nil {
		h.logger.WithError(err).Warn("获取Logstash状态失败")
	}

	// 构建响应
	response := map[string]interface{}{
		"agent":    status,
		"logstash": logstashStatus,
	}

	// 通过WebSocket发送状态
	if client, ok := h.apiClient.(*client.Client); ok {
		if err := client.SendMessage(core.MsgTypeStatusReport, response); err != nil {
			return fmt.Errorf("发送状态失败: %w", err)
		}
	}

	return nil
}

// handleMetricsRequest 处理指标请求
func (h *MessageHandler) handleMetricsRequest(payload []byte) error {
	h.logger.Debug("收到指标请求")

	// 获取指标
	metrics, err := h.metricsCollector.GetMetrics()
	if err != nil {
		return fmt.Errorf("获取指标失败: %w", err)
	}

	// 通过WebSocket发送指标
	if client, ok := h.apiClient.(*client.Client); ok {
		if err := client.SendMessage(core.MsgTypeMetricsReport, map[string]interface{}{
			"metrics": metrics,
		}); err != nil {
			return fmt.Errorf("发送指标失败: %w", err)
		}
	}

	return nil
}