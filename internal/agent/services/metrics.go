package services

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/sirupsen/logrus"
	"logstash-platform/internal/agent/core"
)

// MetricsCollector 指标收集器实现
type MetricsCollector struct {
	agentID         string
	apiClient       core.APIClient
	logstashCtrl    core.LogstashController
	logger          *logrus.Logger
	interval        time.Duration
	
	// 控制
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	running         bool
	mu              sync.Mutex
	
	// 缓存的指标
	lastMetrics     *core.AgentMetrics
	lastMetricsMu   sync.RWMutex
	
	// Logstash进程
	logstashProcess *process.Process
	
	// 统计
	collectCount    int64
	reportCount     int64
	errorCount      int64
	startTime       time.Time
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector(agentID string, apiClient core.APIClient, logstashCtrl core.LogstashController, logger *logrus.Logger) core.MetricsCollector {
	return &MetricsCollector{
		agentID:      agentID,
		apiClient:    apiClient,
		logstashCtrl: logstashCtrl,
		logger:       logger,
		interval:     60 * time.Second, // 默认60秒
		startTime:    time.Now(),
	}
}

// Start 启动指标收集
func (m *MetricsCollector) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.running {
		return nil
	}
	
	m.logger.Info("启动指标收集器")
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.running = true
	
	// 立即收集一次
	m.collectAndReport()
	
	// 启动收集循环
	m.wg.Add(1)
	go m.collectLoop()
	
	return nil
}

// Stop 停止指标收集
func (m *MetricsCollector) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.running {
		return nil
	}
	
	m.logger.Info("停止指标收集器")
	m.running = false
	
	// 取消上下文
	if m.cancel != nil {
		m.cancel()
	}
	
	// 等待goroutine结束
	m.wg.Wait()
	
	m.logger.WithFields(logrus.Fields{
		"collect_count": m.collectCount,
		"report_count":  m.reportCount,
		"error_count":   m.errorCount,
	}).Info("指标收集器已停止")
	
	return nil
}

// GetMetrics 获取当前指标
func (m *MetricsCollector) GetMetrics() (*core.AgentMetrics, error) {
	// 先尝试从缓存获取
	m.lastMetricsMu.RLock()
	if m.lastMetrics != nil && time.Since(m.lastMetrics.Timestamp) < 5*time.Second {
		metrics := *m.lastMetrics
		m.lastMetricsMu.RUnlock()
		return &metrics, nil
	}
	m.lastMetricsMu.RUnlock()
	
	// 收集新指标
	return m.collectMetrics()
}

// SetInterval 设置收集间隔
func (m *MetricsCollector) SetInterval(interval time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if interval < 30*time.Second {
		m.logger.Warn("指标收集间隔太短，使用最小值30秒")
		interval = 30 * time.Second
	}
	
	m.interval = interval
	m.logger.WithField("interval", interval).Info("指标收集间隔已更新")
}

// collectLoop 收集循环
func (m *MetricsCollector) collectLoop() {
	defer m.wg.Done()
	
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
			
		case <-ticker.C:
			m.collectAndReport()
		}
	}
}

// collectAndReport 收集并上报指标
func (m *MetricsCollector) collectAndReport() {
	// 收集指标
	metrics, err := m.collectMetrics()
	if err != nil {
		m.mu.Lock()
		m.errorCount++
		m.mu.Unlock()
		
		m.logger.WithError(err).Error("收集指标失败")
		return
	}
	
	m.mu.Lock()
	m.collectCount++
	m.mu.Unlock()
	
	// 缓存指标
	m.lastMetricsMu.Lock()
	m.lastMetrics = metrics
	m.lastMetricsMu.Unlock()
	
	// 上报指标
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()
	
	if err := m.apiClient.ReportMetrics(ctx, m.agentID, metrics); err != nil {
		m.mu.Lock()
		m.errorCount++
		m.mu.Unlock()
		
		m.logger.WithError(err).Error("上报指标失败")
	} else {
		m.mu.Lock()
		m.reportCount++
		m.mu.Unlock()
		
		m.logger.Debug("指标上报成功")
	}
}

// collectMetrics 收集系统和Logstash指标
func (m *MetricsCollector) collectMetrics() (*core.AgentMetrics, error) {
	metrics := &core.AgentMetrics{
		Timestamp: time.Now(),
		Uptime:    int64(time.Since(m.startTime).Seconds()),
	}
	
	// 收集CPU使用率
	if cpuPercent, err := cpu.Percent(1*time.Second, false); err == nil && len(cpuPercent) > 0 {
		metrics.CPUUsage = cpuPercent[0]
	} else if err != nil {
		m.logger.WithError(err).Warn("获取CPU使用率失败")
	}
	
	// 收集内存使用率
	if memInfo, err := mem.VirtualMemory(); err == nil {
		metrics.MemoryUsage = memInfo.UsedPercent
	} else {
		m.logger.WithError(err).Warn("获取内存使用率失败")
	}
	
	// 收集磁盘使用率（配置目录所在磁盘）
	if diskInfo, err := disk.Usage("/"); err == nil {
		metrics.DiskUsage = diskInfo.UsedPercent
	} else {
		m.logger.WithError(err).Warn("获取磁盘使用率失败")
	}
	
	// 收集Logstash进程指标
	if m.logstashCtrl.IsRunning() {
		status, err := m.logstashCtrl.GetStatus()
		if err == nil && status.PID > 0 {
			m.collectLogstashMetrics(metrics, status.PID)
		}
	}
	
	return metrics, nil
}

// collectLogstashMetrics 收集Logstash进程指标
func (m *MetricsCollector) collectLogstashMetrics(metrics *core.AgentMetrics, pid int) {
	// 获取或创建进程对象
	if m.logstashProcess == nil || m.logstashProcess.Pid != int32(pid) {
		var err error
		m.logstashProcess, err = process.NewProcess(int32(pid))
		if err != nil {
			m.logger.WithError(err).Warn("获取Logstash进程失败")
			return
		}
	}
	
	// 获取进程CPU使用率
	if cpuPercent, err := m.logstashProcess.CPUPercent(); err == nil {
		// 如果Logstash CPU使用率更高，使用它
		if cpuPercent > metrics.CPUUsage {
			metrics.CPUUsage = cpuPercent
		}
	}
	
	// 获取进程内存使用
	if memInfo, err := m.logstashProcess.MemoryInfo(); err == nil {
		// 计算Logstash内存占系统内存的百分比
		if totalMem, err := mem.VirtualMemory(); err == nil {
			logstashMemPercent := float64(memInfo.RSS) / float64(totalMem.Total) * 100
			// 如果Logstash内存使用率更高，使用它
			if logstashMemPercent > metrics.MemoryUsage {
				metrics.MemoryUsage = logstashMemPercent
			}
		}
	}
	
	// TODO: 从Logstash API获取事件处理统计
	// 这里需要Logstash启用监控API
	m.getLogstashEventStats(metrics)
}

// getLogstashEventStats 获取Logstash事件统计
func (m *MetricsCollector) getLogstashEventStats(metrics *core.AgentMetrics) {
	// TODO: 实现从Logstash监控API获取统计信息
	// 默认值
	metrics.EventsReceived = 0
	metrics.EventsSent = 0
	metrics.EventsFailed = 0
	
	// 这里可以通过HTTP请求Logstash的监控API
	// 例如: http://localhost:9600/_node/stats
}

// GetStats 获取收集器统计信息
func (m *MetricsCollector) GetStats() (collectCount, reportCount, errorCount int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.collectCount, m.reportCount, m.errorCount
}

// GetSystemInfo 获取系统信息（用于调试）
func (m *MetricsCollector) GetSystemInfo() map[string]interface{} {
	info := make(map[string]interface{})
	
	// 基本信息
	info["os"] = runtime.GOOS
	info["arch"] = runtime.GOARCH
	info["cpu_count"] = runtime.NumCPU()
	info["go_version"] = runtime.Version()
	
	// CPU信息
	if cpuInfo, err := cpu.Info(); err == nil && len(cpuInfo) > 0 {
		info["cpu_model"] = cpuInfo[0].ModelName
		info["cpu_cores"] = cpuInfo[0].Cores
	}
	
	// 内存信息
	if memInfo, err := mem.VirtualMemory(); err == nil {
		info["total_memory"] = memInfo.Total
		info["available_memory"] = memInfo.Available
	}
	
	// 磁盘信息
	if diskInfo, err := disk.Usage("/"); err == nil {
		info["disk_total"] = diskInfo.Total
		info["disk_free"] = diskInfo.Free
	}
	
	return info
}