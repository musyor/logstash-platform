package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"logstash-platform/internal/platform/api"
	"logstash-platform/pkg/elasticsearch"
)

func main() {
	// 初始化日志
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	// 加载配置
	if err := loadConfig(); err != nil {
		logger.Fatalf("加载配置失败: %v", err)
	}

	// 设置Gin模式
	if viper.GetString("server.mode") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化Elasticsearch客户端
	esClient, err := elasticsearch.NewClient(logger)
	if err != nil {
		logger.Fatalf("初始化Elasticsearch客户端失败: %v", err)
	}

	// 初始化索引
	if err := esClient.InitializeIndices(context.Background()); err != nil {
		logger.Errorf("初始化索引失败: %v", err)
	}

	// 创建API服务器
	apiServer := api.NewServer(logger, esClient)
	router := apiServer.SetupRoutes()

	// 创建HTTP服务器
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", viper.GetString("server.port")),
		Handler: router,
	}

	// 启动服务器
	go func() {
		logger.Infof("启动Logstash管理平台，监听端口: %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("启动服务器失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("收到关闭信号，正在优雅关闭服务器...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Errorf("服务器关闭错误: %v", err)
	}

	logger.Info("服务器已关闭")
}

// loadConfig 加载配置文件
func loadConfig() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// 设置默认值
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("elasticsearch.addresses", []string{"http://localhost:9200"})

	// 环境变量覆盖
	viper.AutomaticEnv()
	viper.SetEnvPrefix("LOGSTASH_PLATFORM")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// 配置文件不存在，使用默认值
			log.Println("配置文件不存在，使用默认配置")
			return nil
		}
		return err
	}

	return nil
}
