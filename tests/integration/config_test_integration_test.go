//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"logstash-platform/internal/platform/models"
)

// TestConfigTestIntegration 测试配置测试功能的集成
func TestConfigTestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 启动测试服务器
	server := NewTestPlatformServer()
	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	baseURL := server.GetURL()

	t.Run("完整的配置测试流程", func(t *testing.T) {
		// 1. 创建一个测试配置
		configID := "test-config-" + generateID()
		config := &models.Config{
			ID:      configID,
			Name:    "Integration Test Config",
			Type:    "filter",
			Content: `
filter {
  mutate {
    add_field => {
      "processed" => "true"
      "timestamp" => "%{+YYYY.MM.dd}"
    }
  }
  
  if [level] == "ERROR" {
    mutate {
      add_tag => ["error"]
    }
  }
}`,
			Version: 1,
		}
		server.configs[configID] = config

		// 2. 创建测试任务
		testReq := models.TestConfigRequest{
			ConfigID: configID,
			TestData: models.TestData{
				Type: "sample",
				Samples: []string{
					`{"message": "Application started", "level": "INFO"}`,
					`{"message": "Database connection failed", "level": "ERROR"}`,
					`{"message": "Request processed", "level": "DEBUG"}`,
				},
			},
		}

		body, _ := json.Marshal(testReq)
		resp, err := http.Post(baseURL+"/api/v1/test", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		var createResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&createResp)
		require.NoError(t, err)
		resp.Body.Close()

		testID := createResp["test_id"].(string)
		assert.NotEmpty(t, testID)
		assert.Equal(t, "running", createResp["status"])

		// 3. 等待测试完成
		time.Sleep(2 * time.Second)

		// 4. 获取测试结果
		resp, err = http.Get(baseURL + "/api/v1/test/" + testID)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result models.TestResult
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		resp.Body.Close()

		// 5. 验证测试结果
		assert.Equal(t, testID, result.TestID)
		assert.Equal(t, "completed", result.Status)
		assert.Equal(t, 3, result.InputCount)
		assert.Equal(t, 3, result.OutputCount)
		assert.Len(t, result.Results, 3)
		assert.Empty(t, result.Errors)

		// 验证处理结果
		for i, output := range result.Results {
			// 所有输出都应该有 processed 字段
			assert.Equal(t, "true", output.Output["processed"])
			assert.NotEmpty(t, output.Output["timestamp"])

			// ERROR 级别的日志应该有 error 标签
			if i == 1 { // 第二条是 ERROR 日志
				tags, ok := output.Output["tags"].([]interface{})
				assert.True(t, ok)
				assert.Contains(t, tags, "error")
			}
		}
	})

	t.Run("Kafka测试类型", func(t *testing.T) {
		configID := "kafka-config-" + generateID()
		config := &models.Config{
			ID:      configID,
			Name:    "Kafka Test Config",
			Type:    "input",
			Content: `
input {
  kafka {
    bootstrap_servers => "localhost:9092"
    topics => ["test-topic"]
    codec => json
  }
}`,
			Version: 1,
		}
		server.configs[configID] = config

		testReq := models.TestConfigRequest{
			ConfigID: configID,
			TestData: models.TestData{
				Type: "kafka",
				KafkaConfig: models.KafkaConfig{
					Brokers:       []string{"localhost:9092"},
					Topic:         "test-topic",
					ConsumerGroup: "test-group",
					MaxMessages:   10,
					Timeout:       30,
				},
			},
		}

		body, _ := json.Marshal(testReq)
		resp, err := http.Post(baseURL+"/api/v1/test", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		var createResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&createResp)
		require.NoError(t, err)
		resp.Body.Close()

		testID := createResp["test_id"].(string)

		// 等待一段时间
		time.Sleep(1 * time.Second)

		// 获取结果
		resp, err = http.Get(baseURL + "/api/v1/test/" + testID)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result models.TestResult
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		resp.Body.Close()

		// Kafka测试当前未实现，应该返回失败
		assert.Equal(t, "failed", result.Status)
		assert.NotEmpty(t, result.Errors)
		assert.Contains(t, result.Errors[0], "Kafka测试功能尚未实现")
	})

	t.Run("配置不存在的测试", func(t *testing.T) {
		testReq := models.TestConfigRequest{
			ConfigID: "non-existent-config",
			TestData: models.TestData{
				Type:    "sample",
				Samples: []string{"test"},
			},
		}

		body, _ := json.Marshal(testReq)
		resp, err := http.Post(baseURL+"/api/v1/test", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		var createResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&createResp)
		require.NoError(t, err)
		resp.Body.Close()

		testID := createResp["test_id"].(string)

		// 等待测试执行
		time.Sleep(500 * time.Millisecond)

		// 获取结果
		resp, err = http.Get(baseURL + "/api/v1/test/" + testID)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result models.TestResult
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		resp.Body.Close()

		assert.Equal(t, "failed", result.Status)
		assert.NotEmpty(t, result.Errors)
		assert.Contains(t, result.Errors[0], "获取配置失败")
	})

	t.Run("并发测试执行", func(t *testing.T) {
		// 创建共享配置
		configID := "concurrent-config-" + generateID()
		config := &models.Config{
			ID:      configID,
			Name:    "Concurrent Test Config",
			Type:    "filter",
			Content: `filter { mutate { add_field => { "processed" => "%{[test_id]}" } } }`,
			Version: 1,
		}
		server.configs[configID] = config

		// 并发创建多个测试
		numTests := 5
		testIDs := make([]string, numTests)

		for i := 0; i < numTests; i++ {
			testReq := models.TestConfigRequest{
				ConfigID: configID,
				TestData: models.TestData{
					Type: "sample",
					Samples: []string{
						`{"test_id": "` + string(rune(i)) + `", "message": "Test message"}`,
					},
				},
			}

			body, _ := json.Marshal(testReq)
			resp, err := http.Post(baseURL+"/api/v1/test", "application/json", bytes.NewBuffer(body))
			require.NoError(t, err)
			assert.Equal(t, http.StatusAccepted, resp.StatusCode)

			var createResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&createResp)
			require.NoError(t, err)
			resp.Body.Close()

			testIDs[i] = createResp["test_id"].(string)
		}

		// 等待所有测试完成
		time.Sleep(2 * time.Second)

		// 验证所有测试结果
		for i, testID := range testIDs {
			resp, err := http.Get(baseURL + "/api/v1/test/" + testID)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var result models.TestResult
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)
			resp.Body.Close()

			assert.Equal(t, "completed", result.Status)
			assert.Equal(t, 1, result.InputCount)
			assert.Equal(t, 1, result.OutputCount)
			
			// 验证每个测试的输出都包含正确的 test_id
			assert.Equal(t, string(rune(i)), result.Results[0].Output["processed"])
		}
	})

	t.Run("测试结果不存在", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/test/non-existent-test-id")
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)
		resp.Body.Close()

		assert.Equal(t, "TEST_NOT_FOUND", errorResp["code"])
	})

	t.Run("无效的测试请求", func(t *testing.T) {
		// 缺少必需字段
		invalidReq := map[string]interface{}{
			"config_id": "", // 空配置ID
		}

		body, _ := json.Marshal(invalidReq)
		resp, err := http.Post(baseURL+"/api/v1/test", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)
		resp.Body.Close()

		assert.Equal(t, "INVALID_REQUEST", errorResp["code"])
	})
}

// TestLargeScaleConfigTest 测试大规模配置测试
func TestLargeScaleConfigTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large scale test in short mode")
	}

	server := NewTestPlatformServer()
	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	baseURL := server.GetURL()

	// 创建配置
	configID := "large-scale-config"
	config := &models.Config{
		ID:   configID,
		Name: "Large Scale Test Config",
		Type: "filter",
		Content: `
filter {
  mutate {
    add_field => { "processed_at" => "%{+YYYY.MM.dd HH:mm:ss}" }
  }
  
  grok {
    match => { "message" => "%{TIMESTAMP_ISO8601:timestamp} %{LOGLEVEL:level} %{GREEDYDATA:content}" }
  }
  
  if [level] == "ERROR" {
    mutate {
      add_tag => ["error", "alert"]
    }
  }
}`,
		Version: 1,
	}
	server.configs[configID] = config

	// 生成大量测试样本
	numSamples := 100
	samples := make([]string, numSamples)
	for i := 0; i < numSamples; i++ {
		level := "INFO"
		if i%10 == 0 {
			level = "ERROR"
		} else if i%5 == 0 {
			level = "WARN"
		}
		samples[i] = `"2024-01-01T12:00:00Z ` + level + ` Sample log message ` + string(rune(i)) + `"`
	}

	// 创建测试
	testReq := models.TestConfigRequest{
		ConfigID: configID,
		TestData: models.TestData{
			Type:    "sample",
			Samples: samples,
		},
	}

	body, _ := json.Marshal(testReq)
	resp, err := http.Post(baseURL+"/api/v1/test", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var createResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&createResp)
	require.NoError(t, err)
	resp.Body.Close()

	testID := createResp["test_id"].(string)

	// 等待处理完成
	time.Sleep(15 * time.Second) // 给更多时间处理大量数据

	// 获取结果
	resp, err = http.Get(baseURL + "/api/v1/test/" + testID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result models.TestResult
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	resp.Body.Close()

	// 验证结果
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, numSamples, result.InputCount)
	assert.Equal(t, numSamples, result.OutputCount)
	assert.Len(t, result.Results, numSamples)

	// 验证处理的正确性
	errorCount := 0
	for i, output := range result.Results {
		assert.NotEmpty(t, output.Output["processed_at"])
		assert.NotEmpty(t, output.Output["timestamp"])
		assert.NotEmpty(t, output.Output["level"])
		assert.NotEmpty(t, output.Output["content"])

		// 验证ERROR级别的处理
		if i%10 == 0 {
			errorCount++
			tags, ok := output.Output["tags"].([]interface{})
			assert.True(t, ok)
			assert.Contains(t, tags, "error")
			assert.Contains(t, tags, "alert")
		}
	}

	assert.Equal(t, 10, errorCount) // 应该有10个ERROR日志
}