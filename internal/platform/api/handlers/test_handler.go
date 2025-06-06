package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"logstash-platform/internal/platform/api/middleware"
	"logstash-platform/internal/platform/service"
)

// TestHandler 测试处理器
type TestHandler struct {
	configService service.ConfigService
	logger        *logrus.Logger
}

// NewTestHandler 创建测试处理器
func NewTestHandler(configService service.ConfigService, logger *logrus.Logger) *TestHandler {
	return &TestHandler{
		configService: configService,
		logger:        logger,
	}
}

// CreateTest 创建测试任务
func (h *TestHandler) CreateTest(c *gin.Context) {
	// TODO: 实现测试逻辑
	c.JSON(http.StatusOK, gin.H{
		"message": "测试功能待实现",
	})
}

// GetTestResult 获取测试结果
func (h *TestHandler) GetTestResult(c *gin.Context) {
	testID := c.Param("id")
	if testID == "" {
		middleware.HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "测试ID不能为空")
		return
	}

	// TODO: 实现获取测试结果逻辑
	c.JSON(http.StatusOK, gin.H{
		"test_id": testID,
		"status":  "pending",
		"message": "测试结果功能待实现",
	})
}