package logstash

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"logstash-platform/internal/agent/config"
	"logstash-platform/internal/agent/core"
)

// Controller Logstash控制器实现
type Controller struct {
	config        *config.AgentConfig
	logger        *logrus.Logger
	
	// 进程管理
	cmd           *exec.Cmd
	cmdMutex      sync.Mutex
	
	// 状态管理
	status        *core.LogstashStatus
	statusMutex   sync.RWMutex
	
	// 日志输出
	logChan       chan string
	errorChan     chan string
	
	// 控制通道
	stopChan      chan struct{}
	stoppedChan   chan struct{}
}

// NewController 创建Logstash控制器
func NewController(cfg *config.AgentConfig, logger *logrus.Logger) core.LogstashController {
	return &Controller{
		config:      cfg,
		logger:      logger,
		logChan:     make(chan string, 100),
		errorChan:   make(chan string, 100),
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
		status: &core.LogstashStatus{
			Running: false,
		},
	}
}

// Start 启动Logstash
func (c *Controller) Start(ctx context.Context) error {
	c.cmdMutex.Lock()
	defer c.cmdMutex.Unlock()
	
	// 检查是否已经运行
	if c.IsRunning() {
		return fmt.Errorf("Logstash已经在运行")
	}
	
	c.logger.Info("正在启动Logstash...")
	
	// 构建命令行参数
	args := c.buildArgs()
	
	// 创建命令
	cmd := exec.CommandContext(ctx, c.config.LogstashPath, args...)
	
	// 设置环境变量
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("LS_JAVA_OPTS=-Xmx1g -Xms1g"),
		fmt.Sprintf("LOGSTASH_PATH_CONF=%s", c.config.ConfigDir),
		fmt.Sprintf("LOGSTASH_PATH_DATA=%s", c.config.DataDir),
		fmt.Sprintf("LOGSTASH_PATH_LOGS=%s", c.config.LogDir),
	)
	
	// 设置工作目录
	cmd.Dir = filepath.Dir(c.config.LogstashPath)
	
	// 获取输出管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建stdout管道失败: %w", err)
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建stderr管道失败: %w", err)
	}
	
	// 启动进程
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动Logstash失败: %w", err)
	}
	
	c.cmd = cmd
	
	// 更新状态
	c.updateStatus(func(s *core.LogstashStatus) {
		s.Running = true
		s.PID = cmd.Process.Pid
		s.StartTime = time.Now()
		s.ConfigPath = c.config.ConfigDir
	})
	
	// 启动日志处理
	go c.handleOutput(stdout, c.logChan)
	go c.handleOutput(stderr, c.errorChan)
	go c.processLogs()
	
	// 等待进程退出
	go c.waitForExit()
	
	// 等待启动完成
	if err := c.waitForStartup(ctx); err != nil {
		c.Stop(context.Background())
		return err
	}
	
	// 获取版本信息
	c.detectVersion()
	
	c.logger.WithField("pid", cmd.Process.Pid).Info("Logstash启动成功")
	return nil
}

// Stop 停止Logstash
func (c *Controller) Stop(ctx context.Context) error {
	c.cmdMutex.Lock()
	defer c.cmdMutex.Unlock()
	
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	
	c.logger.Info("正在停止Logstash...")
	
	// 发送停止信号
	close(c.stopChan)
	
	// 发送SIGTERM信号
	if err := c.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		c.logger.WithError(err).Warn("发送SIGTERM信号失败")
	}
	
	// 等待进程退出
	done := make(chan error, 1)
	go func() {
		done <- c.cmd.Wait()
	}()
	
	select {
	case <-done:
		c.logger.Info("Logstash已正常停止")
	case <-ctx.Done():
		// 超时，强制终止
		c.logger.Warn("停止超时，强制终止Logstash")
		if err := c.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("强制终止失败: %w", err)
		}
	}
	
	// 更新状态
	c.updateStatus(func(s *core.LogstashStatus) {
		s.Running = false
		s.PID = 0
	})
	
	// 等待日志处理结束
	select {
	case <-c.stoppedChan:
	case <-time.After(5 * time.Second):
	}
	
	c.cmd = nil
	return nil
}

// Restart 重启Logstash
func (c *Controller) Restart(ctx context.Context) error {
	c.logger.Info("正在重启Logstash...")
	
	// 停止
	if err := c.Stop(ctx); err != nil {
		return fmt.Errorf("停止Logstash失败: %w", err)
	}
	
	// 等待一段时间
	time.Sleep(2 * time.Second)
	
	// 启动
	if err := c.Start(ctx); err != nil {
		return fmt.Errorf("启动Logstash失败: %w", err)
	}
	
	return nil
}

// Reload 重新加载配置
func (c *Controller) Reload(ctx context.Context) error {
	if !c.IsRunning() {
		return fmt.Errorf("Logstash未运行")
	}
	
	c.logger.Info("正在重载Logstash配置...")
	
	// 发送SIGHUP信号触发重载
	c.cmdMutex.Lock()
	cmd := c.cmd
	c.cmdMutex.Unlock()
	
	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("进程不存在")
	}
	
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("发送重载信号失败: %w", err)
	}
	
	// 更新状态
	c.updateStatus(func(s *core.LogstashStatus) {
		s.LastReloadTime = time.Now()
	})
	
	c.logger.Info("配置重载信号已发送")
	return nil
}

// IsRunning 检查是否运行中
func (c *Controller) IsRunning() bool {
	c.statusMutex.RLock()
	defer c.statusMutex.RUnlock()
	return c.status.Running
}

// GetStatus 获取Logstash状态
func (c *Controller) GetStatus() (*core.LogstashStatus, error) {
	c.statusMutex.RLock()
	defer c.statusMutex.RUnlock()
	
	// 创建状态副本
	status := *c.status
	return &status, nil
}

// ValidateConfig 验证配置文件
func (c *Controller) ValidateConfig(configPath string) error {
	c.logger.WithField("path", configPath).Info("验证配置文件")
	
	// 构建验证命令
	args := []string{
		"--config.test_and_exit",
		"--path.config", configPath,
	}
	
	// 执行验证
	cmd := exec.Command(c.config.LogstashPath, args...)
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		return fmt.Errorf("配置验证失败: %s\n%s", err, string(output))
	}
	
	// 检查输出中是否包含错误
	outputStr := string(output)
	if strings.Contains(outputStr, "ERROR") || strings.Contains(outputStr, "error") {
		return fmt.Errorf("配置包含错误:\n%s", outputStr)
	}
	
	c.logger.Info("配置验证通过")
	return nil
}

// 内部方法

// buildArgs 构建命令行参数
func (c *Controller) buildArgs() []string {
	args := []string{
		"--path.config", c.config.ConfigDir,
		"--path.data", c.config.DataDir,
		"--path.logs", c.config.LogDir,
	}
	
	// 设置工作线程数
	if c.config.PipelineWorkers > 0 {
		args = append(args, "--pipeline.workers", fmt.Sprintf("%d", c.config.PipelineWorkers))
	}
	
	// 设置批处理大小
	if c.config.BatchSize > 0 {
		args = append(args, "--pipeline.batch.size", fmt.Sprintf("%d", c.config.BatchSize))
	}
	
	// 启用配置重载
	if c.config.EnableAutoReload {
		args = append(args, "--config.reload.automatic")
		args = append(args, "--config.reload.interval", "3s")
	}
	
	return args
}

// handleOutput 处理输出
func (c *Controller) handleOutput(pipe io.ReadCloser, output chan<- string) {
	defer pipe.Close()
	
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		select {
		case output <- scanner.Text():
		case <-c.stopChan:
			return
		default:
			// 通道满了，丢弃旧日志
			select {
			case <-output:
			default:
			}
			output <- scanner.Text()
		}
	}
}

// processLogs 处理日志
func (c *Controller) processLogs() {
	defer close(c.stoppedChan)
	
	for {
		select {
		case line := <-c.logChan:
			c.logger.WithField("source", "logstash").Debug(line)
			// 检测启动完成
			if strings.Contains(line, "Pipelines running") {
				c.logger.Info("Logstash管道已启动")
			}
			
		case line := <-c.errorChan:
			c.logger.WithField("source", "logstash").Error(line)
			
		case <-c.stopChan:
			// 清空剩余日志
			for len(c.logChan) > 0 {
				<-c.logChan
			}
			for len(c.errorChan) > 0 {
				<-c.errorChan
			}
			return
		}
	}
}

// waitForExit 等待进程退出
func (c *Controller) waitForExit() {
	c.cmdMutex.Lock()
	cmd := c.cmd
	c.cmdMutex.Unlock()
	
	if cmd == nil {
		return
	}
	
	// 等待进程退出
	err := cmd.Wait()
	
	c.logger.WithError(err).Info("Logstash进程已退出")
	
	// 更新状态
	c.updateStatus(func(s *core.LogstashStatus) {
		s.Running = false
		s.PID = 0
	})
}

// waitForStartup 等待启动完成
func (c *Controller) waitForStartup(ctx context.Context) error {
	startupTimeout := 60 * time.Second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	deadline := time.Now().Add(startupTimeout)
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// 检查进程是否还在运行
			if !c.IsRunning() {
				return fmt.Errorf("Logstash启动失败")
			}
			
			// TODO: 检查Logstash API或日志确认启动完成
			// 暂时使用简单的时间等待
			if time.Now().After(deadline) {
				return fmt.Errorf("启动超时")
			}
			
			// 假设5秒后启动完成
			if time.Since(c.status.StartTime) > 5*time.Second {
				return nil
			}
		}
	}
}

// detectVersion 检测Logstash版本
func (c *Controller) detectVersion() {
	// 执行版本命令
	cmd := exec.Command(c.config.LogstashPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		c.logger.WithError(err).Warn("获取Logstash版本失败")
		return
	}
	
	// 解析版本
	version := strings.TrimSpace(string(output))
	if parts := strings.Fields(version); len(parts) >= 2 {
		version = parts[1]
	}
	
	c.updateStatus(func(s *core.LogstashStatus) {
		s.Version = version
	})
	
	c.logger.WithField("version", version).Info("检测到Logstash版本")
}

// updateStatus 更新状态
func (c *Controller) updateStatus(updater func(*core.LogstashStatus)) {
	c.statusMutex.Lock()
	defer c.statusMutex.Unlock()
	updater(c.status)
}

// GetLogContent 获取日志内容（用于调试）
func (c *Controller) GetLogContent(lines int) ([]string, error) {
	logFile := filepath.Join(c.config.LogDir, "logstash-plain.log")
	
	file, err := os.Open(logFile)
	if err != nil {
		return nil, fmt.Errorf("打开日志文件失败: %w", err)
	}
	defer file.Close()
	
	// 读取最后N行
	var result []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		result = append(result, scanner.Text())
		if len(result) > lines {
			result = result[1:]
		}
	}
	
	return result, scanner.Err()
}