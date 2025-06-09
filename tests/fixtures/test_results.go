package fixtures

import (
	"logstash-platform/internal/platform/models"
	"time"
)

// GetTestResult returns a test result for testing
func GetTestResult() *models.TestResult {
	endTime := time.Now()
	return &models.TestResult{
		TestID:      "test-001",
		Status:      "completed",
		InputCount:  10,
		OutputCount: 10,
		Results: []models.TestOutput{
			{
				Input: `{"message": "Test log entry"}`,
				Output: map[string]interface{}{
					"message": "Test log entry",
					"tags":    []string{"processed"},
				},
			},
		},
		Errors:    []string{},
		StartTime: endTime.Add(-5 * time.Second),
		EndTime:   &endTime,
	}
}

// GetTestResultList returns a list of test results
func GetTestResultList() []*models.TestResult {
	now := time.Now()
	endTime1 := now.Add(-1 * time.Hour)
	endTime2 := now.Add(-2 * time.Hour)
	
	return []*models.TestResult{
		{
			TestID:      "test-001",
			Status:      "completed",
			InputCount:  5,
			OutputCount: 5,
			Results: []models.TestOutput{
				{
					Input: `{"level": "INFO", "message": "Application started"}`,
					Output: map[string]interface{}{
						"level":   "INFO",
						"message": "Application started",
						"tags":    []string{"app", "startup"},
					},
				},
			},
			Errors:    []string{},
			StartTime: endTime1.Add(-10 * time.Second),
			EndTime:   &endTime1,
		},
		{
			TestID:      "test-002",
			Status:      "failed",
			InputCount:  3,
			OutputCount: 1,
			Results: []models.TestOutput{
				{
					Input: `{"data": "valid"}`,
					Output: map[string]interface{}{
						"data": "valid",
					},
				},
			},
			Errors: []string{
				"Failed to parse input at line 2",
				"Invalid JSON format",
			},
			StartTime: endTime2.Add(-30 * time.Second),
			EndTime:   &endTime2,
		},
		{
			TestID:      "test-003",
			Status:      "running",
			InputCount:  100,
			OutputCount: 45,
			Results:     []models.TestOutput{},
			Errors:      []string{},
			StartTime:   now.Add(-2 * time.Minute),
			EndTime:     nil,
		},
	}
}

// TestScenarioData represents test scenario data
type TestScenarioData struct {
	Name           string
	Description    string
	ConfigIDs      []string
	InputData      string
	ExpectedOutput map[string]interface{}
}

// GetTestScenarios returns various test scenarios
func GetTestScenarios() map[string]*TestScenarioData {
	return map[string]*TestScenarioData{
		"simple_filter": {
			Name:        "Simple Filter Test",
			Description: "Test basic filter functionality",
			ConfigIDs:   []string{"config-001"},
			InputData:   `{"message": "127.0.0.1 - - [06/Jan/2024:10:00:00 +0000] \"GET /api/v1/status HTTP/1.1\" 200 150"}`,
			ExpectedOutput: map[string]interface{}{
				"client_ip": "127.0.0.1",
				"method":    "GET",
				"path":      "/api/v1/status",
				"status":    200,
			},
		},
		"pipeline_test": {
			Name:        "Full Pipeline Test",
			Description: "Test complete input-filter-output pipeline",
			ConfigIDs:   []string{"config-input-001", "config-filter-001", "config-output-001"},
			InputData:   `{"source": "app-server-01", "level": "ERROR", "message": "Database connection failed"}`,
			ExpectedOutput: map[string]interface{}{
				"source":    "app-server-01",
				"level":     "ERROR",
				"message":   "Database connection failed",
				"processed": true,
				"timestamp": "2024-01-06T10:00:00Z",
			},
		},
		"error_handling": {
			Name:        "Error Handling Test",
			Description: "Test error handling in pipeline",
			ConfigIDs:   []string{"config-filter-error"},
			InputData:   `invalid json data`,
			ExpectedOutput: map[string]interface{}{
				"tags":          []string{"_jsonparsefailure"},
				"error_message": "Failed to parse JSON",
			},
		},
	}
}

// GetTestRequestData returns test request data
func GetTestRequestData() *models.TestConfigRequest {
	return &models.TestConfigRequest{
		ConfigID: "config-001",
		TestData: models.TestData{
			Type:    "sample",
			Samples: []string{
				`{"message": "Test log 1"}`,
				`{"message": "Test log 2"}`,
				`{"message": "Test log 3"}`,
			},
		},
	}
}

// GetKafkaTestData returns Kafka test data
func GetKafkaTestData() *models.TestConfigRequest {
	return &models.TestConfigRequest{
		ConfigID: "config-002",
		TestData: models.TestData{
			Type: "kafka",
			KafkaConfig: models.KafkaConfig{
				Brokers:       []string{"localhost:9092"},
				Topic:         "test-topic",
				ConsumerGroup: "test-group",
				MaxMessages:   100,
				Timeout:       30,
			},
		},
	}
}