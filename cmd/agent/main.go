package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"logstash-platform/internal/agent/client"
	"logstash-platform/internal/agent/config"
	"logstash-platform/internal/agent/core"
	"logstash-platform/internal/agent/logstash"
	"logstash-platform/internal/agent/services"
	"logstash-platform/pkg/logger"
)

var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// 命令行参数
	var (
		configFile   = flag.String("config", "agent.yaml", "配置文件路径")
		showVersion  = flag.Bool("version", false, "显示版本信息")
		logLevel     = flag.String("log-level", "info", "日志级别 (debug, info, warn, error)")
		agentID      = flag.String("agent-id", "", "Agent ID (覆盖配置文件中的设置)")
		serverURL    = flag.String("server", "", "服务器地址 (覆盖配置文件中的设置)")
	)
	flag.Parse()

	// 显示版本信息
	if *showVersion {
		fmt.Printf("Logstash Agent\n")
		fmt.Printf("Version:    %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		os.Exit(0)
	}

	// 初始化日志
	log := logger.New(map[string]interface{}{
		"level":  *logLevel,
		"format": "text",
		"output": "stdout",
	})

	log.WithFields(logrus.Fields{
		"version":    version,
		"build_time": buildTime,
		"git_commit": gitCommit,
	}).Info("Logstash Agent 启动中...")

	// 加载配置
	cfg, err := loadConfig(*configFile, *agentID, *serverURL)
	if err != nil {
		log.WithError(err).Fatal("加载配置失败")
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		log.WithError(err).Fatal("配置验证失败")
	}

	// 创建Agent实例
	agent, err := createAgent(cfg, log)
	if err != nil {
		log.WithError(err).Fatal("创建Agent失败")
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动Agent
	if err := agent.Start(ctx); err != nil {
		log.WithError(err).Fatal("启动Agent失败")
	}

	log.Info("Agent启动成功，等待信号...")

	// 等待退出信号
	select {
	case sig := <-sigChan:
		log.WithField("signal", sig).Info("收到退出信号")
	case <-ctx.Done():
		log.Info("上下文取消")
	}

	// 优雅关闭
	log.Info("正在关闭Agent...")
	
	// 创建关闭超时上下文
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := agent.Stop(shutdownCtx); err != nil {
		log.WithError(err).Error("关闭Agent时出错")
	}

	log.Info("Agent已关闭")
}

// loadConfig 加载配置文件
func loadConfig(configFile, agentID, serverURL string) (*config.AgentConfig, error) {
	// 获取配置文件绝对路径
	absPath, err := filepath.Abs(configFile)
	if err != nil {
		return nil, fmt.Errorf("获取配置文件路径失败: %w", err)
	}

	// 加载配置文件
	cfg, err := config.LoadFromFile(absPath)
	if err != nil {
		// 如果配置文件不存在，使用默认配置
		if os.IsNotExist(err) {
			cfg = config.DefaultConfig()
		} else {
			return nil, fmt.Errorf("加载配置文件失败: %w", err)
		}
	}

	// 命令行参数覆盖配置文件
	if agentID != "" {
		cfg.AgentID = agentID
	}
	if serverURL != "" {
		cfg.ServerURL = serverURL
	}

	// 如果没有设置AgentID，使用主机名
	if cfg.AgentID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("获取主机名失败: %w", err)
		}
		cfg.AgentID = hostname
	}

	return cfg, nil
}

// createAgent 创建完整的Agent实例
func createAgent(cfg *config.AgentConfig, logger *logrus.Logger) (*core.Agent, error) {
	// 创建基本Agent
	agent, err := core.NewAgent(cfg, logger)
	if err != nil {
		return nil, err
	}

	// 创建API客户端
	apiClient, err := client.NewClient(cfg, logger)
	if err != nil {
		return nil, err
	}

	// 创建配置管理器
	configMgr, err := config.NewManager(cfg, logger)
	if err != nil {
		return nil, err
	}

	// 创建Logstash控制器
	logstashCtrl := logstash.NewController(cfg, logger)

	// 创建心跳服务
	heartbeat := services.NewHeartbeatService(cfg.AgentID, apiClient, logger)

	// 创建指标收集器
	metrics := services.NewMetricsCollector(cfg.AgentID, apiClient, logstashCtrl, logger)

	// 组装Agent
	agent.
		WithAPIClient(apiClient).
		WithConfigManager(configMgr).
		WithLogstashController(logstashCtrl).
		WithHeartbeatService(heartbeat).
		WithMetricsCollector(metrics)

	return agent, nil
}