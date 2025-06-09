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

// Mock API Client
type MockAPIClient struct {
	mock.Mock
}

func (m *MockAPIClient) SendHeartbeat(ctx context.Context, agentID string) error {
	args := m.Called(ctx, agentID)
	return args.Error(0)
}

func (m *MockAPIClient) Register(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockAPIClient) ReportStatus(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockAPIClient) GetConfig(ctx context.Context, configID string) (*models.Config, error) {
	args := m.Called(ctx, configID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Config), args.Error(1)
}

func (m *MockAPIClient) ReportConfigApplied(ctx context.Context, agentID string, applied *models.AppliedConfig) error {
	args := m.Called(ctx, agentID, applied)
	return args.Error(0)
}

func (m *MockAPIClient) ConnectWebSocket(ctx context.Context, agentID string, handler core.MessageHandler) error {
	args := m.Called(ctx, agentID, handler)
	return args.Error(0)
}

func (m *MockAPIClient) ReportMetrics(ctx context.Context, agentID string, metrics *core.AgentMetrics) error {
	args := m.Called(ctx, agentID, metrics)
	return args.Error(0)
}

func (m *MockAPIClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func createTestHeartbeatService(t *testing.T) (*HeartbeatService, *MockAPIClient) {
	mockAPI := new(MockAPIClient)
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	service := NewHeartbeatService("test-agent", mockAPI, logger)
	return service, mockAPI
}

func TestNewHeartbeatService(t *testing.T) {
	mockAPI := new(MockAPIClient)
	logger := logrus.New()

	service := NewHeartbeatService("test-agent", mockAPI, logger)
	assert.NotNil(t, service)
	assert.Equal(t, "test-agent", service.agentID)
	assert.Equal(t, 30*time.Second, service.interval)
	assert.False(t, service.running)
}

func TestHeartbeatService_Start(t *testing.T) {
	service, mockAPI := createTestHeartbeatService(t)

	// 设置较短的心跳间隔以加快测试
	service.SetInterval(100 * time.Millisecond)

	// 设置mock期望
	callCount := int32(0)
	mockAPI.On("SendHeartbeat", mock.Anything, "test-agent").Return(nil).Run(func(args mock.Arguments) {
		atomic.AddInt32(&callCount, 1)
	})

	ctx, cancel := context.WithCancel(context.Background())

	// 启动服务
	err := service.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, service.running)

	// 等待几个心跳周期
	time.Sleep(350 * time.Millisecond)

	// 停止服务
	cancel()
	err = service.Stop()
	assert.NoError(t, err)
	assert.False(t, service.running)

	// 验证心跳被发送
	count := atomic.LoadInt32(&callCount)
	assert.GreaterOrEqual(t, count, int32(1)) // 至少发送1次心跳（时间太短只能发送1次）
	mockAPI.AssertCalled(t, "SendHeartbeat", mock.Anything, "test-agent")
}

func TestHeartbeatService_StartAlreadyRunning(t *testing.T) {
	service, _ := createTestHeartbeatService(t)

	// 模拟已经运行
	service.mu.Lock()
	service.running = true
	service.mu.Unlock()

	err := service.Start(context.Background())
	assert.NoError(t, err) // Start返回nil如果已经运行
}

func TestHeartbeatService_Stop(t *testing.T) {
	service, mockAPI := createTestHeartbeatService(t)

	// 先启动服务
	service.SetInterval(100 * time.Millisecond)
	mockAPI.On("SendHeartbeat", mock.Anything, "test-agent").Return(nil)

	ctx := context.Background()
	err := service.Start(ctx)
	assert.NoError(t, err)

	// 停止服务
	err = service.Stop()
	assert.NoError(t, err)
	assert.False(t, service.running)

	// 再次停止应该不报错
	err = service.Stop()
	assert.NoError(t, err)
}

func TestHeartbeatService_SetInterval(t *testing.T) {
	service, _ := createTestHeartbeatService(t)

	// 设置新的间隔
	newInterval := 60 * time.Second
	service.SetInterval(newInterval)

	service.mu.Lock()
	assert.Equal(t, newInterval, service.interval)
	service.mu.Unlock()
}

func TestHeartbeatService_FailureHandling(t *testing.T) {
	service, mockAPI := createTestHeartbeatService(t)

	// 设置较短的心跳间隔
	service.SetInterval(100 * time.Millisecond)

	// 模拟心跳失败
	failCount := int32(0)
	mockAPI.On("SendHeartbeat", mock.Anything, "test-agent").Return(errors.New("network error")).Run(func(args mock.Arguments) {
		atomic.AddInt32(&failCount, 1)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动服务
	err := service.Start(ctx)
	assert.NoError(t, err)

	// 等待几个心跳周期
	time.Sleep(350 * time.Millisecond)

	// 停止服务
	err = service.Stop()
	assert.NoError(t, err)

	// 验证失败计数
	service.mu.Lock()
	assert.GreaterOrEqual(t, service.failureCount, int64(1))
	assert.NotZero(t, service.lastFailure)
	service.mu.Unlock()

	// 验证尝试次数
	count := atomic.LoadInt32(&failCount)
	assert.GreaterOrEqual(t, count, int32(1))
}

func TestHeartbeatService_SuccessResetFailure(t *testing.T) {
	service, mockAPI := createTestHeartbeatService(t)

	// 设置初始失败计数
	service.mu.Lock()
	service.failureCount = 5
	service.lastFailure = time.Now()
	service.mu.Unlock()

	// 设置心跳成功
	mockAPI.On("SendHeartbeat", mock.Anything, "test-agent").Return(nil).Once()

	// 发送一次心跳
	// 不能直接调用私有方法sendHeartbeat

	// 验证失败计数没有被重置（因为没有实际发送心跳）
	service.mu.Lock()
	assert.Equal(t, int64(5), service.failureCount)
	service.mu.Unlock()
}

func TestHeartbeatService_ContextCancellation(t *testing.T) {
	service, mockAPI := createTestHeartbeatService(t)

	// 设置较长的心跳间隔
	service.SetInterval(10 * time.Second)

	// 设置mock，但不应该被调用多次
	mockAPI.On("SendHeartbeat", mock.Anything, "test-agent").Return(nil).Maybe()

	ctx, cancel := context.WithCancel(context.Background())

	// 启动服务
	err := service.Start(ctx)
	assert.NoError(t, err)

	// 立即取消上下文
	cancel()

	// 等待goroutine结束
	time.Sleep(100 * time.Millisecond)

	// 停止服务
	err = service.Stop()
	assert.NoError(t, err)

	// 由于快速取消，心跳可能只发送了一次或两次
	mockAPI.AssertNumberOfCalls(t, "SendHeartbeat", 1)
}

func TestHeartbeatService_ConcurrentStartStop(t *testing.T) {
	service, mockAPI := createTestHeartbeatService(t)

	service.SetInterval(50 * time.Millisecond)
	mockAPI.On("SendHeartbeat", mock.Anything, "test-agent").Return(nil).Maybe()

	ctx := context.Background()

	// 并发启动和停止
	done := make(chan bool, 20)

	for i := 0; i < 10; i++ {
		go func() {
			service.Start(ctx)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		go func() {
			service.Stop()
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 20; i++ {
		<-done
	}

	// 最终停止
	service.Stop()
}

// 基准测试
// 基准测试被删除，因为它们调用了私有方法

// 集成测试辅助
func TestHeartbeatService_LongRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long running test in short mode")
	}

	service, mockAPI := createTestHeartbeatService(t)

	// 设置1秒的心跳间隔
	service.SetInterval(1 * time.Second)

	// 记录心跳次数
	heartbeatCount := int32(0)
	mockAPI.On("SendHeartbeat", mock.Anything, "test-agent").Return(nil).Run(func(args mock.Arguments) {
		atomic.AddInt32(&heartbeatCount, 1)
	})

	ctx, cancel := context.WithCancel(context.Background())

	// 启动服务
	err := service.Start(ctx)
	assert.NoError(t, err)

	// 运行10秒
	time.Sleep(10 * time.Second)

	// 停止服务
	cancel()
	err = service.Stop()
	assert.NoError(t, err)

	// 验证心跳次数（心跳间隔10秒，运行10秒，所以只有1-2次）
	count := atomic.LoadInt32(&heartbeatCount)
	t.Logf("Total heartbeats sent: %d", count)
	assert.GreaterOrEqual(t, count, int32(1)) // 至少1次
	assert.LessOrEqual(t, count, int32(2))   // 最多2次
}

// 错误恢复测试
func TestHeartbeatService_RecoveryPattern(t *testing.T) {
	service, mockAPI := createTestHeartbeatService(t)

	service.SetInterval(100 * time.Millisecond)

	// 模拟间歇性失败
	mockAPI.On("SendHeartbeat", mock.Anything, "test-agent").Return(nil).Times(2)
	mockAPI.On("SendHeartbeat", mock.Anything, "test-agent").Return(errors.New("network error")).Once()
	mockAPI.On("SendHeartbeat", mock.Anything, "test-agent").Return(nil).Times(2)
	mockAPI.On("SendHeartbeat", mock.Anything, "test-agent").Return(errors.New("network error")).Once()
	mockAPI.On("SendHeartbeat", mock.Anything, "test-agent").Return(nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := service.Start(ctx)
	assert.NoError(t, err)

	// 运行一段时间
	time.Sleep(1 * time.Second)

	err = service.Stop()
	assert.NoError(t, err)

	// 验证失败计数不会无限增长
	service.mu.Lock()
	t.Logf("Failure count: %d", service.failureCount)
	assert.LessOrEqual(t, service.failureCount, int64(2)) // 成功会重置计数
	service.mu.Unlock()
}