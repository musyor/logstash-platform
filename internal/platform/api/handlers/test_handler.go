package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"logstash-platform/internal/platform/api/middleware"
	"logstash-platform/internal/platform/models"
	"logstash-platform/internal/platform/service"
)

// TestHandler 测试处理器
type TestHandler struct {
	configService service.ConfigService
	logger        *logrus.Logger
	
	// 临时存储测试结果
	testResults map[string]*models.TestResult
	mu          sync.RWMutex
}

// NewTestHandler 创建测试处理器
func NewTestHandler(configService service.ConfigService, logger *logrus.Logger) *TestHandler {
	return &TestHandler{
		configService: configService,
		logger:        logger,
		testResults:   make(map[string]*models.TestResult),
	}
}

// CreateTest 创建测试任务
func (h *TestHandler) CreateTest(c *gin.Context) {
	var req models.TestConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// 生成测试ID
	testID := generateTestID()

	// 创建测试任务
	testResult := &models.TestResult{
		TestID:      testID,
		Status:      "running",
		InputCount:  0,
		OutputCount: 0,
		Results:     []models.TestOutput{},
		Errors:      []string{},
		StartTime:   time.Now(),
	}

	// TODO: 将测试任务保存到存储中
	h.storeTestResult(testID, testResult)

	// 异步执行测试
	go h.executeTest(testID, &req)

	c.JSON(http.StatusAccepted, gin.H{
		"test_id": testID,
		"status":  "running",
		"message": "测试任务已创建",
	})
}

// GetTestResult 获取测试结果
func (h *TestHandler) GetTestResult(c *gin.Context) {
	testID := c.Param("id")
	if testID == "" {
		middleware.HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "测试ID不能为空")
		return
	}

	h.mu.RLock()
	result, exists := h.testResults[testID]
	h.mu.RUnlock()

	if !exists {
		middleware.HandleError(c, http.StatusNotFound, "TEST_NOT_FOUND", "测试任务不存在")
		return
	}

	c.JSON(http.StatusOK, result)
}

// 辅助方法

// generateTestID 生成测试ID
func generateTestID() string {
	return uuid.New().String()
}

// storeTestResult 存储测试结果
func (h *TestHandler) storeTestResult(testID string, result *models.TestResult) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.testResults[testID] = result
}

// updateTestResult 更新测试结果
func (h *TestHandler) updateTestResult(testID string, update func(*models.TestResult)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if result, exists := h.testResults[testID]; exists {
		update(result)
	}
}

// executeTest 执行测试
func (h *TestHandler) executeTest(testID string, req *models.TestConfigRequest) {
	h.logger.WithField("test_id", testID).Info("开始执行配置测试")

	// 获取配置
	config, err := h.configService.GetConfig(context.Background(), req.ConfigID)
	if err != nil {
		h.updateTestResult(testID, func(result *models.TestResult) {
			result.Status = "failed"
			result.Errors = append(result.Errors, fmt.Sprintf("获取配置失败: %v", err))
			endTime := time.Now()
			result.EndTime = &endTime
		})
		return
	}

	// 根据测试数据类型执行测试
	switch req.TestData.Type {
	case "sample":
		h.executeSampleTest(testID, config, req.TestData.Samples)
	case "kafka":
		h.executeKafkaTest(testID, config, &req.TestData.KafkaConfig)
	default:
		h.updateTestResult(testID, func(result *models.TestResult) {
			result.Status = "failed"
			result.Errors = append(result.Errors, fmt.Sprintf("不支持的测试类型: %s", req.TestData.Type))
			endTime := time.Now()
			result.EndTime = &endTime
		})
	}
}

// executeSampleTest 执行样本数据测试
func (h *TestHandler) executeSampleTest(testID string, config *models.Config, samples []string) {
	h.logger.WithField("test_id", testID).Info("执行样本数据测试")

	// 更新输入计数
	h.updateTestResult(testID, func(result *models.TestResult) {
		result.InputCount = len(samples)
	})

	// TODO: 实际的Logstash测试逻辑
	// 这里简化处理，假设所有样本都成功处理
	for i, sample := range samples {
		output := models.TestOutput{
			Input: sample,
			Output: map[string]interface{}{
				"message": sample,
				"@timestamp": time.Now().Format(time.RFC3339),
				"test_field": fmt.Sprintf("processed_%d", i),
			},
		}

		h.updateTestResult(testID, func(result *models.TestResult) {
			result.Results = append(result.Results, output)
			result.OutputCount++
		})

		// 模拟处理延迟
		time.Sleep(100 * time.Millisecond)
	}

	// 标记测试完成
	h.updateTestResult(testID, func(result *models.TestResult) {
		result.Status = "completed"
		endTime := time.Now()
		result.EndTime = &endTime
	})

	h.logger.WithField("test_id", testID).Info("样本数据测试完成")
}

// executeKafkaTest 执行Kafka数据测试
func (h *TestHandler) executeKafkaTest(testID string, config *models.Config, kafkaConfig *models.KafkaConfig) {
	h.logger.WithField("test_id", testID).Info("执行Kafka数据测试")

	// TODO: 实际的Kafka测试逻辑
	// 这里简化处理
	h.updateTestResult(testID, func(result *models.TestResult) {
		result.Status = "failed"
		result.Errors = append(result.Errors, "Kafka测试功能尚未实现")
		endTime := time.Now()
		result.EndTime = &endTime
	})
}