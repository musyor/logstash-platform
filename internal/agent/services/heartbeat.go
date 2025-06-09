package services

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"logstash-platform/internal/agent/core"
)

// HeartbeatService 心跳服务实现
type HeartbeatService struct {
	agentID   string
	apiClient core.APIClient
	logger    *logrus.Logger
	interval  time.Duration
	
	// 控制
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   bool
	mu        sync.Mutex
	
	// 回调
	onSuccess func()
	onFailure func(error)
	
	// 统计
	successCount int64
	failureCount int64
	lastSuccess  time.Time
	lastFailure  time.Time
}

// NewHeartbeatService 创建心跳服务
func NewHeartbeatService(agentID string, apiClient core.APIClient, logger *logrus.Logger) core.HeartbeatService {
	return &HeartbeatService{
		agentID:   agentID,
		apiClient: apiClient,
		logger:    logger,
		interval:  30 * time.Second, // 默认30秒
	}
}

// Start 启动心跳服务
func (h *HeartbeatService) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.running {
		return nil
	}
	
	h.logger.Info("启动心跳服务")
	h.ctx, h.cancel = context.WithCancel(ctx)
	h.running = true
	
	// 立即发送一次心跳
	h.sendHeartbeat()
	
	// 启动心跳循环
	h.wg.Add(1)
	go h.heartbeatLoop()
	
	return nil
}

// Stop 停止心跳服务
func (h *HeartbeatService) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if !h.running {
		return nil
	}
	
	h.logger.Info("停止心跳服务")
	h.running = false
	
	// 取消上下文
	if h.cancel != nil {
		h.cancel()
	}
	
	// 等待goroutine结束
	h.wg.Wait()
	
	h.logger.WithFields(logrus.Fields{
		"success_count": h.successCount,
		"failure_count": h.failureCount,
	}).Info("心跳服务已停止")
	
	return nil
}

// SetInterval 设置心跳间隔
func (h *HeartbeatService) SetInterval(interval time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if interval < 10*time.Second {
		h.logger.Warn("心跳间隔太短，使用最小值10秒")
		interval = 10 * time.Second
	}
	
	h.interval = interval
	h.logger.WithField("interval", interval).Info("心跳间隔已更新")
}

// SetCallbacks 设置回调函数
func (h *HeartbeatService) SetCallbacks(onSuccess func(), onFailure func(error)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.onSuccess = onSuccess
	h.onFailure = onFailure
}

// GetStats 获取统计信息
func (h *HeartbeatService) GetStats() (successCount, failureCount int64, lastSuccess, lastFailure time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	return h.successCount, h.failureCount, h.lastSuccess, h.lastFailure
}

// heartbeatLoop 心跳循环
func (h *HeartbeatService) heartbeatLoop() {
	defer h.wg.Done()
	
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-h.ctx.Done():
			return
			
		case <-ticker.C:
			h.sendHeartbeat()
		}
	}
}

// sendHeartbeat 发送心跳
func (h *HeartbeatService) sendHeartbeat() {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(h.ctx, 10*time.Second)
	defer cancel()
	
	// 添加时间戳到上下文
	ctx = context.WithValue(ctx, "timestamp", time.Now().Unix())
	
	// 发送心跳
	start := time.Now()
	err := h.apiClient.SendHeartbeat(ctx, h.agentID)
	duration := time.Since(start)
	
	h.mu.Lock()
	if err != nil {
		h.failureCount++
		h.lastFailure = time.Now()
		h.mu.Unlock()
		
		h.logger.WithError(err).WithField("duration", duration).Error("发送心跳失败")
		
		// 调用失败回调
		if h.onFailure != nil {
			h.onFailure(err)
		}
	} else {
		h.successCount++
		h.lastSuccess = time.Now()
		h.mu.Unlock()
		
		h.logger.WithField("duration", duration).Debug("心跳发送成功")
		
		// 调用成功回调
		if h.onSuccess != nil {
			h.onSuccess()
		}
	}
}

// IsHealthy 检查心跳服务是否健康
func (h *HeartbeatService) IsHealthy() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if !h.running {
		return false
	}
	
	// 如果最近3个心跳周期内有成功，认为是健康的
	if !h.lastSuccess.IsZero() && time.Since(h.lastSuccess) < h.interval*3 {
		return true
	}
	
	// 如果从未成功过，但服务刚启动不久，也认为是健康的
	if h.successCount == 0 && h.failureCount < 3 {
		return true
	}
	
	return false
}

// GetInterval 获取当前心跳间隔
func (h *HeartbeatService) GetInterval() time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.interval
}