package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"logstash-platform/internal/platform/api/middleware"
	"logstash-platform/internal/platform/models"
	"logstash-platform/internal/platform/service"
)

// ConfigHandler 配置处理器
type ConfigHandler struct {
	configService service.ConfigService
	logger        *logrus.Logger
}

// NewConfigHandler 创建配置处理器
func NewConfigHandler(configService service.ConfigService, logger *logrus.Logger) *ConfigHandler {
	return &ConfigHandler{
		configService: configService,
		logger:        logger,
	}
}

// ListConfigs 获取配置列表
func (h *ConfigHandler) ListConfigs(c *gin.Context) {
	var req models.ConfigListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数无效")
		return
	}

	// 处理标签参数
	if tags := c.QueryArray("tags[]"); len(tags) > 0 {
		req.Tags = tags
	}

	resp, err := h.configService.ListConfigs(c.Request.Context(), &req)
	if err != nil {
		h.logger.Errorf("获取配置列表失败: %v", err)
		middleware.HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "获取配置列表失败")
		return
	}

	c.JSON(http.StatusOK, resp)
}

// CreateConfig 创建配置
func (h *ConfigHandler) CreateConfig(c *gin.Context) {
	var req models.CreateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数无效")
		return
	}

	// TODO: 从JWT或会话中获取用户ID
	userID := "admin"

	config, err := h.configService.CreateConfig(c.Request.Context(), &req, userID)
	if err != nil {
		h.logger.Errorf("创建配置失败: %v", err)
		middleware.HandleError(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	c.JSON(http.StatusCreated, config)
}

// GetConfig 获取单个配置
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		middleware.HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "配置ID不能为空")
		return
	}

	config, err := h.configService.GetConfig(c.Request.Context(), id)
	if err != nil {
		if err.Error() == "文档不存在" {
			middleware.HandleError(c, http.StatusNotFound, "NOT_FOUND", "配置不存在")
			return
		}
		h.logger.Errorf("获取配置失败: %v", err)
		middleware.HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "获取配置失败")
		return
	}

	c.JSON(http.StatusOK, config)
}

// UpdateConfig 更新配置
func (h *ConfigHandler) UpdateConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		middleware.HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "配置ID不能为空")
		return
	}

	var req models.UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数无效")
		return
	}

	// TODO: 从JWT或会话中获取用户ID
	userID := "admin"

	config, err := h.configService.UpdateConfig(c.Request.Context(), id, &req, userID)
	if err != nil {
		if err.Error() == "配置不存在" {
			middleware.HandleError(c, http.StatusNotFound, "NOT_FOUND", "配置不存在")
			return
		}
		h.logger.Errorf("更新配置失败: %v", err)
		middleware.HandleError(c, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}

	c.JSON(http.StatusOK, config)
}

// DeleteConfig 删除配置
func (h *ConfigHandler) DeleteConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		middleware.HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "配置ID不能为空")
		return
	}

	if err := h.configService.DeleteConfig(c.Request.Context(), id); err != nil {
		if err.Error() == "配置不存在" {
			middleware.HandleError(c, http.StatusNotFound, "NOT_FOUND", "配置不存在")
			return
		}
		h.logger.Errorf("删除配置失败: %v", err)
		middleware.HandleError(c, http.StatusInternalServerError, "DELETE_FAILED", "删除配置失败")
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// GetConfigHistory 获取配置历史
func (h *ConfigHandler) GetConfigHistory(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		middleware.HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "配置ID不能为空")
		return
	}

	history, err := h.configService.GetConfigHistory(c.Request.Context(), id)
	if err != nil {
		if err.Error() == "配置不存在" {
			middleware.HandleError(c, http.StatusNotFound, "NOT_FOUND", "配置不存在")
			return
		}
		h.logger.Errorf("获取配置历史失败: %v", err)
		middleware.HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "获取配置历史失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": history,
		"total": len(history),
	})
}

// RollbackConfig 回滚配置
func (h *ConfigHandler) RollbackConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		middleware.HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "配置ID不能为空")
		return
	}

	var req struct {
		Version int `json:"version" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数无效")
		return
	}

	// TODO: 从JWT或会话中获取用户ID
	userID := "admin"

	config, err := h.configService.RollbackConfig(c.Request.Context(), id, req.Version, userID)
	if err != nil {
		h.logger.Errorf("回滚配置失败: %v", err)
		middleware.HandleError(c, http.StatusInternalServerError, "ROLLBACK_FAILED", err.Error())
		return
	}

	c.JSON(http.StatusOK, config)
}