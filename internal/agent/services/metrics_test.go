package services

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"logstash-platform/internal/agent/core"
	"logstash-platform/internal/platform/models"
)

// Mock Logstash Controller
type MockLogstashController struct {
	mock.Mock
}

func (m *MockLogstashController) IsRunning() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockLogstashController) GetStatus() (*core.LogstashStatus, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.LogstashStatus), args.Error(1)
}

func (m *MockLogstashController) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockLogstashController) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockLogstashController) Restart(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockLogstashController) Reload(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockLogstashController) ValidateConfig(configPath string) error {
	args := m.Called(configPath)
	return args.Error(0)
}

// Extended Mock API Client for metrics
type MockMetricsAPIClient struct {
	mock.Mock
}

func (m *MockMetricsAPIClient) Register(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockMetricsAPIClient) SendHeartbeat(ctx context.Context, agentID string) error {
	args := m.Called(ctx, agentID)
	return args.Error(0)
}

func (m *MockMetricsAPIClient) ReportStatus(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockMetricsAPIClient) GetConfig(ctx context.Context, configID string) (*models.Config, error) {
	args := m.Called(ctx, configID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Config), args.Error(1)
}

func (m *MockMetricsAPIClient) ReportConfigApplied(ctx context.Context, agentID string, applied *models.AppliedConfig) error {
	args := m.Called(ctx, agentID, applied)
	return args.Error(0)
}

func (m *MockMetricsAPIClient) ConnectWebSocket(ctx context.Context, agentID string, handler core.MessageHandler) error {
	args := m.Called(ctx, agentID, handler)
	return args.Error(0)
}

func (m *MockMetricsAPIClient) ReportMetrics(ctx context.Context, agentID string, metrics *core.AgentMetrics) error {
	args := m.Called(ctx, agentID, metrics)
	return args.Error(0)
}

func (m *MockMetricsAPIClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func createTestMetricsCollector(t *testing.T) (*MetricsCollector, *MockMetricsAPIClient, *MockLogstashController) {
	mockAPI := new(MockMetricsAPIClient)
	mockLogstash := new(MockLogstashController)
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	collector := NewMetricsCollector("test-agent", mockAPI, mockLogstash, logger)
	return collector, mockAPI, mockLogstash
}

func TestNewMetricsCollector(t *testing.T) {
	mockAPI := new(MockMetricsAPIClient)
	mockLogstash := new(MockLogstashController)
	logger := logrus.New()

	collector := NewMetricsCollector("test-agent", mockAPI, mockLogstash, logger)
	assert.NotNil(t, collector)
	assert.Equal(t, "test-agent", collector.agentID)
	assert.Equal(t, 60*time.Second, collector.interval)
	assert.False(t, collector.running)
}

func TestMetricsCollector_Start(t *testing.T) {
	collector, mockAPI, mockLogstash := createTestMetricsCollector(t)

	// 设置较短的收集间隔
	collector.SetInterval(100 * time.Millisecond)

	// 设置mock期望
	mockLogstash.On("IsRunning").Return(true)
	mockLogstash.On("GetStatus").Return(&core.LogstashStatus{
		Running: true,
		PID:     12345,
	}, nil)

	callCount := int32(0)
	mockAPI.On("ReportMetrics", mock.Anything, "test-agent", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		atomic.AddInt32(&callCount, 1)
		metrics := args.Get(2).(*core.AgentMetrics)
		assert.NotZero(t, metrics.CPUUsage)
		assert.NotZero(t, metrics.MemoryUsage)
		assert.NotZero(t, metrics.DiskUsage)
	})

	ctx, cancel := context.WithCancel(context.Background())

	// 启动收集器
	err := collector.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, collector.running)

	// 等待几个收集周期
	time.Sleep(350 * time.Millisecond)

	// 停止收集器
	cancel()
	err = collector.Stop()
	assert.NoError(t, err)
	assert.False(t, collector.running)

	// 验证指标被上报
	count := atomic.LoadInt32(&callCount)
	assert.GreaterOrEqual(t, count, int32(3))
	mockAPI.AssertCalled(t, "ReportMetrics", mock.Anything, "test-agent", mock.Anything)
}

func TestMetricsCollector_GetMetrics(t *testing.T) {
	collector, _, mockLogstash := createTestMetricsCollector(t)

	// 设置mock
	mockLogstash.On("IsRunning").Return(true)
	mockLogstash.On("GetStatus").Return(&core.LogstashStatus{
		Running: true,
		PID:     12345,
	}, nil)

	// 获取指标
	metrics, err := collector.GetMetrics()
	assert.NoError(t, err)
	assert.NotNil(t, metrics)

	// 验证基本指标
	assert.NotZero(t, metrics.Timestamp)
	assert.GreaterOrEqual(t, metrics.CPUUsage, float64(0))
	assert.LessOrEqual(t, metrics.CPUUsage, float64(100))
	assert.GreaterOrEqual(t, metrics.MemoryUsage, float64(0))
	assert.LessOrEqual(t, metrics.MemoryUsage, float64(100))
	assert.GreaterOrEqual(t, metrics.DiskUsage, float64(0))
	assert.LessOrEqual(t, metrics.DiskUsage, float64(100))
	assert.Greater(t, metrics.Uptime, int64(0))
}

func TestMetricsCollector_CollectMetrics(t *testing.T) {
	collector, _, mockLogstash := createTestMetricsCollector(t)

	tests := []struct {
		name            string
		logstashRunning bool
		logstashStatus  *core.LogstashStatus
		logstashError   error
		wantErr         bool
	}{
		{
			name:            "logstash running",
			logstashRunning: true,
			logstashStatus: &core.LogstashStatus{
				Running: true,
				PID:     12345,
			},
			logstashError: nil,
			wantErr:       false,
		},
		{
			name:            "logstash not running",
			logstashRunning: false,
			logstashStatus:  nil,
			logstashError:   nil,
			wantErr:         false,
		},
		{
			name:            "logstash status error",
			logstashRunning: true,
			logstashStatus:  nil,
			logstashError:   errors.New("failed to get status"),
			wantErr:         false, // 错误被忽略
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 重置mock
			mockLogstash.ExpectedCalls = nil
			mockLogstash.On("IsRunning").Return(tt.logstashRunning)
			if tt.logstashRunning {
				mockLogstash.On("GetStatus").Return(tt.logstashStatus, tt.logstashError)
			}

			metrics, err := collector.collectMetrics()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, metrics)
			}
		})
	}
}

func TestMetricsCollector_SetInterval(t *testing.T) {
	collector, _, _ := createTestMetricsCollector(t)

	newInterval := 30 * time.Second
	collector.SetInterval(newInterval)

	collector.mu.Lock()
	assert.Equal(t, newInterval, collector.interval)
	collector.mu.Unlock()
}

func TestMetricsCollector_AlreadyRunning(t *testing.T) {
	collector, _, _ := createTestMetricsCollector(t)

	// 模拟已经运行
	collector.mu.Lock()
	collector.running = true
	collector.mu.Unlock()

	err := collector.Start(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "指标收集器已经在运行")
}

func TestMetricsCollector_ReportingError(t *testing.T) {
	collector, mockAPI, mockLogstash := createTestMetricsCollector(t)

	collector.SetInterval(100 * time.Millisecond)

	// 设置mock
	mockLogstash.On("IsRunning").Return(true)
	mockLogstash.On("GetStatus").Return(&core.LogstashStatus{
		Running: true,
		PID:     12345,
	}, nil)

	// 模拟上报失败
	mockAPI.On("ReportMetrics", mock.Anything, "test-agent", mock.Anything).Return(errors.New("network error"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := collector.Start(ctx)
	assert.NoError(t, err)

	// 等待几个周期
	time.Sleep(350 * time.Millisecond)

	err = collector.Stop()
	assert.NoError(t, err)

	// 验证即使上报失败，收集器仍然继续运行
	mockAPI.AssertCalled(t, "ReportMetrics", mock.Anything, "test-agent", mock.Anything)
}

func TestMetricsCollector_ContextCancellation(t *testing.T) {
	collector, mockAPI, mockLogstash := createTestMetricsCollector(t)

	collector.SetInterval(10 * time.Second) // 长间隔

	// 设置mock
	mockLogstash.On("IsRunning").Return(true).Maybe()
	mockLogstash.On("GetStatus").Return(&core.LogstashStatus{Running: true}, nil).Maybe()
	mockAPI.On("ReportMetrics", mock.Anything, "test-agent", mock.Anything).Return(nil).Maybe()

	ctx, cancel := context.WithCancel(context.Background())

	err := collector.Start(ctx)
	assert.NoError(t, err)

	// 立即取消
	cancel()

	time.Sleep(100 * time.Millisecond)

	err = collector.Stop()
	assert.NoError(t, err)

	// 只应该发送一次指标
	mockAPI.AssertNumberOfCalls(t, "ReportMetrics", 1)
}

func TestMetricsCollector_ProcessMetrics(t *testing.T) {
	collector, _, mockLogstash := createTestMetricsCollector(t)

	// 模拟不同的PID情况
	tests := []struct {
		name     string
		pid      int
		expected bool
	}{
		{"valid pid", 12345, true},
		{"invalid pid", 0, false},
		{"negative pid", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogstash.ExpectedCalls = nil
			mockLogstash.On("IsRunning").Return(true)
			mockLogstash.On("GetStatus").Return(&core.LogstashStatus{
				Running: true,
				PID:     tt.pid,
			}, nil)

			metrics, err := collector.collectMetrics()
			assert.NoError(t, err)
			assert.NotNil(t, metrics)

			// 注意：由于我们无法获取非存在进程的指标，
			// 这里只验证指标结构存在
		})
	}
}

// 并发测试
func TestMetricsCollector_ConcurrentOperations(t *testing.T) {
	collector, mockAPI, mockLogstash := createTestMetricsCollector(t)

	mockLogstash.On("IsRunning").Return(true)
	mockLogstash.On("GetStatus").Return(&core.LogstashStatus{Running: true}, nil)
	mockAPI.On("ReportMetrics", mock.Anything, "test-agent", mock.Anything).Return(nil).Maybe()

	// 并发获取指标
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = collector.GetMetrics()
			done <- true
		}()
	}

	// 等待完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// 基准测试
func BenchmarkMetricsCollector_CollectMetrics(b *testing.B) {
	collector, _, mockLogstash := createTestMetricsCollector(&testing.T{})

	mockLogstash.On("IsRunning").Return(true)
	mockLogstash.On("GetStatus").Return(&core.LogstashStatus{
		Running: true,
		PID:     12345,
	}, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = collector.collectMetrics()
	}
}

func BenchmarkMetricsCollector_GetMetrics(b *testing.B) {
	collector, _, mockLogstash := createTestMetricsCollector(&testing.T{})

	mockLogstash.On("IsRunning").Return(true)
	mockLogstash.On("GetStatus").Return(&core.LogstashStatus{
		Running: true,
		PID:     12345,
	}, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = collector.GetMetrics()
	}
}

// 集成测试
func TestMetricsCollector_LongRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long running test in short mode")
	}

	collector, mockAPI, mockLogstash := createTestMetricsCollector(t)

	collector.SetInterval(1 * time.Second)

	mockLogstash.On("IsRunning").Return(true)
	mockLogstash.On("GetStatus").Return(&core.LogstashStatus{
		Running: true,
		PID:     12345,
	}, nil)

	metricsCount := int32(0)
	mockAPI.On("ReportMetrics", mock.Anything, "test-agent", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		atomic.AddInt32(&metricsCount, 1)
		metrics := args.Get(2).(*core.AgentMetrics)
		t.Logf("Metrics: CPU=%.2f%%, Memory=%.2f%%, Disk=%.2f%%",
			metrics.CPUUsage, metrics.MemoryUsage, metrics.DiskUsage)
	})

	ctx, cancel := context.WithCancel(context.Background())

	err := collector.Start(ctx)
	assert.NoError(t, err)

	// 运行10秒
	time.Sleep(10 * time.Second)

	cancel()
	err = collector.Stop()
	assert.NoError(t, err)

	count := atomic.LoadInt32(&metricsCount)
	t.Logf("Total metrics collected: %d", count)
	assert.GreaterOrEqual(t, count, int32(9))
	assert.LessOrEqual(t, count, int32(11))
}

// 测试指标值的合理性
func TestMetricsCollector_MetricsValidity(t *testing.T) {
	collector, _, mockLogstash := createTestMetricsCollector(t)

	mockLogstash.On("IsRunning").Return(true)
	mockLogstash.On("GetStatus").Return(&core.LogstashStatus{
		Running: true,
		PID:     12345,
	}, nil)

	// 收集多次指标
	for i := 0; i < 5; i++ {
		metrics, err := collector.GetMetrics()
		assert.NoError(t, err)

		// 验证指标范围
		assert.GreaterOrEqual(t, metrics.CPUUsage, float64(0), "CPU usage should be >= 0")
		assert.LessOrEqual(t, metrics.CPUUsage, float64(100), "CPU usage should be <= 100")

		assert.GreaterOrEqual(t, metrics.MemoryUsage, float64(0), "Memory usage should be >= 0")
		assert.LessOrEqual(t, metrics.MemoryUsage, float64(100), "Memory usage should be <= 100")

		assert.GreaterOrEqual(t, metrics.DiskUsage, float64(0), "Disk usage should be >= 0")
		assert.LessOrEqual(t, metrics.DiskUsage, float64(100), "Disk usage should be <= 100")

		assert.Greater(t, metrics.Uptime, int64(0), "Uptime should be positive")

		time.Sleep(100 * time.Millisecond)
	}
}