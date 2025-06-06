package api

import (
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"logstash-platform/internal/platform/api/handlers"
	"logstash-platform/internal/platform/api/middleware"
	"logstash-platform/internal/platform/repository"
	"logstash-platform/internal/platform/service"
	"logstash-platform/pkg/elasticsearch"
)

// Server API服务器
type Server struct {
	router         *gin.Engine
	logger         *logrus.Logger
	esClient       elasticsearch.ClientInterface
	configService  service.ConfigService
}

// NewServer 创建新的API服务器
func NewServer(logger *logrus.Logger, esClient elasticsearch.ClientInterface) *Server {
	// 创建仓库层
	configRepo := repository.NewConfigRepository(esClient, logger)
	
	// 创建服务层
	configService := service.NewConfigService(configRepo, logger)

	return &Server{
		logger:        logger,
		esClient:      esClient,
		configService: configService,
	}
}

// SetupRoutes 设置路由
func (s *Server) SetupRoutes() *gin.Engine {
	router := gin.New()

	// 全局中间件
	router.Use(gin.Recovery())
	router.Use(middleware.Logger(s.logger))
	router.Use(middleware.ErrorHandler())
	router.Use(middleware.CORS())

	// 健康检查
	router.GET("/health", handlers.HealthCheck)

	// API v1路由组
	v1 := router.Group("/api/v1")
	{
		// 配置管理路由
		configs := v1.Group("/configs")
		{
			configHandler := handlers.NewConfigHandler(s.configService, s.logger)
			
			configs.GET("", configHandler.ListConfigs)        // 获取配置列表
			configs.POST("", configHandler.CreateConfig)      // 创建配置
			configs.GET("/:id", configHandler.GetConfig)      // 获取单个配置
			configs.PUT("/:id", configHandler.UpdateConfig)   // 更新配置
			configs.DELETE("/:id", configHandler.DeleteConfig) // 删除配置
			configs.GET("/:id/history", configHandler.GetConfigHistory) // 获取配置历史
			configs.POST("/:id/rollback", configHandler.RollbackConfig) // 回滚配置
		}

		// 测试路由
		test := v1.Group("/test")
		{
			testHandler := handlers.NewTestHandler(s.configService, s.logger)
			
			test.POST("", testHandler.CreateTest)              // 创建测试任务
			test.GET("/:id/result", testHandler.GetTestResult) // 获取测试结果
		}

		// Agent管理路由
		agents := v1.Group("/agents")
		{
			agentHandler := handlers.NewAgentHandler(s.configService, s.logger)
			
			agents.GET("", agentHandler.ListAgents)           // 获取Agent列表
			agents.GET("/:id", agentHandler.GetAgent)         // 获取单个Agent
			agents.POST("/:id/deploy", agentHandler.DeployConfig) // 部署配置到Agent
		}

		// 批量操作路由
		v1.POST("/deploy", handlers.BatchDeploy(s.configService, s.logger)) // 批量部署
	}

	// WebSocket路由
	router.GET("/ws", middleware.AuthorizeWebSocket(), handlers.WebSocketHandler(s.logger))

	s.router = router
	return router
}

// GetRouter 获取路由
func (s *Server) GetRouter() *gin.Engine {
	if s.router == nil {
		return s.SetupRoutes()
	}
	return s.router
}