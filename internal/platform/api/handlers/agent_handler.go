package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"logstash-platform/internal/platform/api/middleware"
	"logstash-platform/internal/platform/service"
)

// AgentHandler Agent处理器
type AgentHandler struct {
	configService service.ConfigService
	logger        *logrus.Logger
}

// NewAgentHandler 创建Agent处理器
func NewAgentHandler(configService service.ConfigService, logger *logrus.Logger) *AgentHandler {
	return &AgentHandler{
		configService: configService,
		logger:        logger,
	}
}

// ListAgents 获取Agent列表
func (h *AgentHandler) ListAgents(c *gin.Context) {
	// TODO: 实现Agent列表逻辑
	c.JSON(http.StatusOK, gin.H{
		"items": []interface{}{},
		"total": 0,
		"message": "Agent管理功能待实现",
	})
}

// GetAgent 获取单个Agent
func (h *AgentHandler) GetAgent(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		middleware.HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "Agent ID不能为空")
		return
	}

	// TODO: 实现获取Agent逻辑
	c.JSON(http.StatusOK, gin.H{
		"agent_id": agentID,
		"status":   "offline",
		"message":  "Agent详情功能待实现",
	})
}

// DeployConfig 部署配置到Agent
func (h *AgentHandler) DeployConfig(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		middleware.HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "Agent ID不能为空")
		return
	}

	// TODO: 实现部署逻辑
	c.JSON(http.StatusOK, gin.H{
		"agent_id": agentID,
		"status":   "pending",
		"message":  "部署功能待实现",
	})
}

// BatchDeploy 批量部署
func BatchDeploy(configService service.ConfigService, logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: 实现批量部署逻辑
		c.JSON(http.StatusOK, gin.H{
			"status":  "pending",
			"message": "批量部署功能待实现",
		})
	}
}