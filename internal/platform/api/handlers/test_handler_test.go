package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"logstash-platform/internal/platform/models"
)

func setupTestHandlerRouter() (*gin.Engine, *TestHandler, *MockConfigService) {
	gin.SetMode(gin.TestMode)
	
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	mockService := new(MockConfigService)
	handler := NewTestHandler(mockService, logger)
	
	router := gin.New()
	return router, handler, mockService
}

func TestCreateTest(t *testing.T) {
	router, handler, mockService := setupTestHandlerRouter()
	router.POST("/test", handler.CreateTest)

	t.Run("成功创建样本数据测试", func(t *testing.T) {
		// 准备测试数据
		reqBody := models.TestConfigRequest{
			ConfigID: "test-config-123",
			TestData: models.TestData{
				Type: "sample",
				Samples: []string{
					"log line 1",
					"log line 2",
					"log line 3",
				},
			},
		}

		testConfig := &models.Config{
			ID:      "test-config-123",
			Name:    "Test Config",
			Content: "input { stdin {} } output { stdout {} }",
			Version: 1,
		}

		// 设置mock期望
		mockService.On("GetConfig", mock.Anything, "test-config-123").Return(testConfig, nil)

		// 发送请求
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 验证响应
		assert.Equal(t, http.StatusAccepted, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.NotEmpty(t, response["test_id"])
		assert.Equal(t, "running", response["status"])
		assert.Equal(t, "测试任务已创建", response["message"])

		// 等待一点时间让异步任务执行
		time.Sleep(500 * time.Millisecond)

		// 验证测试结果已存储
		testID := response["test_id"].(string)
		handler.mu.RLock()
		result, exists := handler.testResults[testID]
		handler.mu.RUnlock()
		
		assert.True(t, exists)
		assert.Equal(t, "completed", result.Status)
		assert.Equal(t, 3, result.InputCount)
		assert.Equal(t, 3, result.OutputCount)
		assert.Len(t, result.Results, 3)
		assert.Empty(t, result.Errors)
	})

	t.Run("缺少必需字段", func(t *testing.T) {
		// 发送空请求体
		req, _ := http.NewRequest("POST", "/test", bytes.NewBuffer([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 验证响应
		assert.Equal(t, http.StatusBadRequest, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, "INVALID_REQUEST", response["code"])
	})

	t.Run("配置不存在", func(t *testing.T) {
		reqBody := models.TestConfigRequest{
			ConfigID: "non-existent-config",
			TestData: models.TestData{
				Type:    "sample",
				Samples: []string{"test"},
			},
		}

		// 设置mock期望
		mockService.On("GetConfig", mock.Anything, "non-existent-config").Return(nil, errors.New("config not found"))

		// 发送请求
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 验证响应
		assert.Equal(t, http.StatusAccepted, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		testID := response["test_id"].(string)
		
		// 等待异步任务执行
		time.Sleep(100 * time.Millisecond)

		// 验证测试失败
		handler.mu.RLock()
		result, exists := handler.testResults[testID]
		handler.mu.RUnlock()
		
		assert.True(t, exists)
		assert.Equal(t, "failed", result.Status)
		assert.NotEmpty(t, result.Errors)
		assert.Contains(t, result.Errors[0], "获取配置失败")
	})

	t.Run("不支持的测试类型", func(t *testing.T) {
		// 发送一个包含未知类型的请求，这会在绑定时失败
		reqBody := map[string]interface{}{
			"config_id": "test-config-123",
			"test_data": map[string]interface{}{
				"type": "unknown",
			},
		}

		// 发送请求
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 验证响应 - 应该是验证错误
		assert.Equal(t, http.StatusBadRequest, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, "INVALID_REQUEST", response["code"])
		assert.Contains(t, response["message"], "oneof")
	})
}

func TestGetTestResult(t *testing.T) {
	router, handler, _ := setupTestHandlerRouter()
	router.GET("/test/:id", handler.GetTestResult)

	t.Run("成功获取测试结果", func(t *testing.T) {
		// 准备测试数据
		testID := "test-123"
		testResult := &models.TestResult{
			TestID:      testID,
			Status:      "completed",
			InputCount:  5,
			OutputCount: 5,
			Results: []models.TestOutput{
				{
					Input: "log line 1",
					Output: map[string]interface{}{
						"message": "log line 1",
					},
				},
			},
			Errors:    []string{},
			StartTime: time.Now().Add(-1 * time.Minute),
		}
		endTime := time.Now()
		testResult.EndTime = &endTime

		// 存储测试结果
		handler.storeTestResult(testID, testResult)

		// 发送请求
		req, _ := http.NewRequest("GET", "/test/"+testID, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 验证响应
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response models.TestResult
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, testID, response.TestID)
		assert.Equal(t, "completed", response.Status)
		assert.Equal(t, 5, response.InputCount)
		assert.Equal(t, 5, response.OutputCount)
		assert.Len(t, response.Results, 1)
	})

	t.Run("测试结果不存在", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/test/non-existent-id", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 验证响应
		assert.Equal(t, http.StatusNotFound, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, "TEST_NOT_FOUND", response["code"])
		assert.Equal(t, "测试任务不存在", response["message"])
	})

	t.Run("缺少测试ID", func(t *testing.T) {
		// 直接访问空ID
		w := httptest.NewRecorder()
		
		// 需要手动处理空参数情况
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: ""}}
		c.Request = httptest.NewRequest("GET", "/test/", nil)
		handler.GetTestResult(c)

		// 验证响应
		assert.Equal(t, http.StatusBadRequest, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, "INVALID_REQUEST", response["code"])
		assert.Equal(t, "测试ID不能为空", response["message"])
	})
}

func TestExecuteSampleTest(t *testing.T) {
	_, handler, _ := setupTestHandlerRouter()

	t.Run("样本测试成功执行", func(t *testing.T) {
		testID := "sample-test-123"
		config := &models.Config{
			ID:      "config-123",
			Name:    "Sample Config",
			Content: "filter { mutate { add_field => { \"processed\" => \"true\" } } }",
		}
		samples := []string{
			"2024-01-01 INFO Application started",
			"2024-01-01 ERROR Database connection failed",
			"2024-01-01 WARN Memory usage high",
		}

		// 初始化测试结果
		testResult := &models.TestResult{
			TestID:      testID,
			Status:      "running",
			InputCount:  0,
			OutputCount: 0,
			Results:     []models.TestOutput{},
			Errors:      []string{},
			StartTime:   time.Now(),
		}
		handler.storeTestResult(testID, testResult)

		// 执行测试
		handler.executeSampleTest(testID, config, samples)

		// 验证结果
		handler.mu.RLock()
		result := handler.testResults[testID]
		handler.mu.RUnlock()

		assert.Equal(t, "completed", result.Status)
		assert.Equal(t, 3, result.InputCount)
		assert.Equal(t, 3, result.OutputCount)
		assert.Len(t, result.Results, 3)
		assert.NotNil(t, result.EndTime)
		
		// 验证输出内容
		for i, output := range result.Results {
			assert.Equal(t, samples[i], output.Input)
			assert.NotNil(t, output.Output["message"])
			assert.NotNil(t, output.Output["@timestamp"])
			assert.NotNil(t, output.Output["test_field"])
		}
	})

	t.Run("空样本列表", func(t *testing.T) {
		testID := "empty-sample-test"
		config := &models.Config{
			ID:      "config-123",
			Content: "filter {}",
		}
		samples := []string{}

		// 初始化测试结果
		testResult := &models.TestResult{
			TestID:    testID,
			Status:    "running",
			StartTime: time.Now(),
			Results:   []models.TestOutput{},
			Errors:    []string{},
		}
		handler.storeTestResult(testID, testResult)

		// 执行测试
		handler.executeSampleTest(testID, config, samples)

		// 验证结果
		handler.mu.RLock()
		result := handler.testResults[testID]
		handler.mu.RUnlock()

		assert.Equal(t, "completed", result.Status)
		assert.Equal(t, 0, result.InputCount)
		assert.Equal(t, 0, result.OutputCount)
		assert.Len(t, result.Results, 0)
		assert.NotNil(t, result.EndTime)
	})
}

func TestExecuteKafkaTest(t *testing.T) {
	_, handler, _ := setupTestHandlerRouter()

	t.Run("Kafka测试尚未实现", func(t *testing.T) {
		testID := "kafka-test-123"
		config := &models.Config{
			ID:      "config-123",
			Content: "input { kafka {} }",
		}
		kafkaConfig := &models.KafkaConfig{
			Brokers:       []string{"localhost:9092"},
			Topic:         "test-topic",
			ConsumerGroup: "test-group",
		}

		// 初始化测试结果
		testResult := &models.TestResult{
			TestID:    testID,
			Status:    "running",
			StartTime: time.Now(),
			Results:   []models.TestOutput{},
			Errors:    []string{},
		}
		handler.storeTestResult(testID, testResult)

		// 执行测试
		handler.executeKafkaTest(testID, config, kafkaConfig)

		// 验证结果
		handler.mu.RLock()
		result := handler.testResults[testID]
		handler.mu.RUnlock()

		assert.Equal(t, "failed", result.Status)
		assert.NotEmpty(t, result.Errors)
		assert.Contains(t, result.Errors[0], "Kafka测试功能尚未实现")
		assert.NotNil(t, result.EndTime)
	})
}

func TestConcurrentTestExecution(t *testing.T) {
	router, handler, mockService := setupTestHandlerRouter()
	router.POST("/test", handler.CreateTest)
	router.GET("/test/:id", handler.GetTestResult)

	// 准备测试配置
	testConfig := &models.Config{
		ID:      "concurrent-config",
		Name:    "Concurrent Test Config",
		Content: "filter { }",
	}
	mockService.On("GetConfig", mock.Anything, "concurrent-config").Return(testConfig, nil)

	// 并发创建多个测试
	numTests := 10
	testIDs := make([]string, 0, numTests)
	
	for i := 0; i < numTests; i++ {
		reqBody := models.TestConfigRequest{
			ConfigID: "concurrent-config",
			TestData: models.TestData{
				Type: "sample",
				Samples: []string{
					"concurrent test line 1",
					"concurrent test line 2",
				},
			},
		}

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusAccepted, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		testIDs = append(testIDs, response["test_id"].(string))
	}

	// 等待所有测试完成
	time.Sleep(1 * time.Second)

	// 验证所有测试结果
	for _, testID := range testIDs {
		req, _ := http.NewRequest("GET", "/test/"+testID, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var result models.TestResult
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		
		assert.Equal(t, "completed", result.Status)
		assert.Equal(t, 2, result.InputCount)
		assert.Equal(t, 2, result.OutputCount)
		assert.Len(t, result.Results, 2)
	}
}

func TestTestResultCleanup(t *testing.T) {
	_, handler, _ := setupTestHandlerRouter()

	// 创建多个测试结果
	oldTestID := "old-test-123"
	oldResult := &models.TestResult{
		TestID:    oldTestID,
		Status:    "completed",
		StartTime: time.Now().Add(-2 * time.Hour),
	}
	endTime := time.Now().Add(-1 * time.Hour)
	oldResult.EndTime = &endTime

	recentTestID := "recent-test-456"
	recentResult := &models.TestResult{
		TestID:    recentTestID,
		Status:    "completed",
		StartTime: time.Now().Add(-5 * time.Minute),
	}
	recentEndTime := time.Now()
	recentResult.EndTime = &recentEndTime

	// 存储测试结果
	handler.storeTestResult(oldTestID, oldResult)
	handler.storeTestResult(recentTestID, recentResult)

	// 验证两个结果都存在
	handler.mu.RLock()
	_, oldExists := handler.testResults[oldTestID]
	_, recentExists := handler.testResults[recentTestID]
	handler.mu.RUnlock()

	assert.True(t, oldExists)
	assert.True(t, recentExists)

	// TODO: 实现定期清理旧测试结果的功能
	// 这里可以添加清理逻辑的测试
}

func TestGenerateTestID(t *testing.T) {
	// 测试ID生成的唯一性
	ids := make(map[string]bool)
	
	for i := 0; i < 100; i++ {
		id := generateTestID()
		assert.NotEmpty(t, id)
		assert.False(t, ids[id], "生成了重复的测试ID: %s", id)
		ids[id] = true
	}
}

func TestUpdateTestResult(t *testing.T) {
	_, handler, _ := setupTestHandlerRouter()

	testID := "update-test-123"
	testResult := &models.TestResult{
		TestID:      testID,
		Status:      "running",
		InputCount:  0,
		OutputCount: 0,
		StartTime:   time.Now(),
	}

	// 存储初始结果
	handler.storeTestResult(testID, testResult)

	// 更新结果
	handler.updateTestResult(testID, func(result *models.TestResult) {
		result.Status = "completed"
		result.InputCount = 10
		result.OutputCount = 10
		endTime := time.Now()
		result.EndTime = &endTime
	})

	// 验证更新
	handler.mu.RLock()
	updatedResult := handler.testResults[testID]
	handler.mu.RUnlock()

	assert.Equal(t, "completed", updatedResult.Status)
	assert.Equal(t, 10, updatedResult.InputCount)
	assert.Equal(t, 10, updatedResult.OutputCount)
	assert.NotNil(t, updatedResult.EndTime)
}

func TestUpdateNonExistentTestResult(t *testing.T) {
	_, handler, _ := setupTestHandlerRouter()

	// 尝试更新不存在的测试结果
	handler.updateTestResult("non-existent-id", func(result *models.TestResult) {
		result.Status = "completed"
	})

	// 验证结果仍然不存在
	handler.mu.RLock()
	_, exists := handler.testResults["non-existent-id"]
	handler.mu.RUnlock()

	assert.False(t, exists)
}