package fixtures

import (
	"fmt"
	"logstash-platform/internal/platform/models"
	"time"
)

// GetValidFilterConfig returns a valid filter configuration for testing
func GetValidFilterConfig() *models.Config {
	return &models.Config{
		ID:          "test-filter-001",
		Name:        "nginx-access-log-parser",
		Type:        models.ConfigTypeFilter,
		Content:     "filter {\n  grok {\n    match => { \"message\" => \"%{COMBINEDAPACHELOG}\" }\n  }\n  date {\n    match => [ \"timestamp\", \"dd/MMM/yyyy:HH:mm:ss Z\" ]\n  }\n}",
		Description: "Parse nginx access logs",
		Tags:        []string{"nginx", "access-log", "parser"},
		Version:     1,
		Enabled:     true,
		TestStatus:  models.TestStatusPassed,
		CreatedBy:   "admin",
		UpdatedBy:   "admin",
		CreatedAt:   time.Now().Add(-24 * time.Hour),
		UpdatedAt:   time.Now().Add(-24 * time.Hour),
	}
}

// GetValidInputConfig returns a valid input configuration for testing
func GetValidInputConfig() *models.Config {
	return &models.Config{
		ID:          "test-input-001",
		Name:        "kafka-input",
		Type:        models.ConfigTypeInput,
		Content:     "input {\n  kafka {\n    bootstrap_servers => \"localhost:9092\"\n    topics => [\"app-logs\"]\n    codec => \"json\"\n  }\n}",
		Description: "Kafka input configuration",
		Tags:        []string{"kafka", "input"},
		Version:     1,
		Enabled:     true,
		TestStatus:  models.TestStatusUntested,
		CreatedBy:   "user1",
		UpdatedBy:   "user1",
		CreatedAt:   time.Now().Add(-48 * time.Hour),
		UpdatedAt:   time.Now().Add(-48 * time.Hour),
	}
}

// GetValidOutputConfig returns a valid output configuration for testing
func GetValidOutputConfig() *models.Config {
	return &models.Config{
		ID:          "test-output-001",
		Name:        "elasticsearch-output",
		Type:        models.ConfigTypeOutput,
		Content:     "output {\n  elasticsearch {\n    hosts => [\"localhost:9200\"]\n    index => \"logstash-%{+YYYY.MM.dd}\"\n  }\n}",
		Description: "Elasticsearch output configuration",
		Tags:        []string{"elasticsearch", "output"},
		Version:     2,
		Enabled:     false,
		TestStatus:  models.TestStatusFailed,
		// TestMessage field doesn't exist in model
		CreatedBy:   "admin",
		UpdatedBy:   "user2",
		CreatedAt:   time.Now().Add(-72 * time.Hour),
		UpdatedAt:   time.Now().Add(-12 * time.Hour),
	}
}

// GetInvalidConfigs returns various invalid configurations for negative testing
func GetInvalidConfigs() map[string]*models.Config {
	return map[string]*models.Config{
		"empty_content": {
			Name:    "empty-config",
			Type:    models.ConfigTypeFilter,
			Content: "",
		},
		"missing_keyword": {
			Name:    "missing-filter-keyword",
			Type:    models.ConfigTypeFilter,
			Content: "{ mutate { add_field => { \"test\" => \"value\" } } }",
		},
		"invalid_type": {
			Name:    "invalid-type",
			Type:    "invalid",
			Content: "filter { }",
		},
	}
}

// GetConfigList returns a list of configs for testing pagination
func GetConfigList(count int) []*models.Config {
	configs := make([]*models.Config, count)
	baseTime := time.Now().Add(-time.Duration(count) * 24 * time.Hour)
	
	for i := 0; i < count; i++ {
		configType := models.ConfigTypeFilter
		if i%3 == 1 {
			configType = models.ConfigTypeInput
		} else if i%3 == 2 {
			configType = models.ConfigTypeOutput
		}
		
		configs[i] = &models.Config{
			ID:          fmt.Sprintf("config-%03d", i+1),
			Name:        fmt.Sprintf("test-config-%d", i+1),
			Type:        configType,
			Content:     fmt.Sprintf("%s { # Config %d }", configType, i+1),
			Description: fmt.Sprintf("Test configuration number %d", i+1),
			Tags:        []string{"test", fmt.Sprintf("batch-%d", i/10)},
			Version:     1,
			Enabled:     i%2 == 0,
			TestStatus:  models.TestStatusUntested,
			CreatedBy:   fmt.Sprintf("user%d", i%3+1),
			UpdatedBy:   fmt.Sprintf("user%d", i%3+1),
			CreatedAt:   baseTime.Add(time.Duration(i) * time.Hour),
			UpdatedAt:   baseTime.Add(time.Duration(i) * time.Hour),
		}
	}
	
	return configs
}

// GetConfigHistory returns test configuration history entries
func GetConfigHistory() []*models.ConfigHistory {
	baseTime := time.Now().Add(-7 * 24 * time.Hour)
	
	return []*models.ConfigHistory{
		{
			ID:         "history-001",
			ConfigID:   "test-filter-001",
			Version:    1,
			Content:    "filter { # Initial version }",
			ChangeType: "create",
			ChangeLog:  "Initial configuration",
			ModifiedBy: "admin",
			ModifiedAt: baseTime,
		},
		{
			ID:         "history-002",
			ConfigID:   "test-filter-001",
			Version:    2,
			Content:    "filter {\n  grok {\n    match => { \"message\" => \"%{COMBINEDAPACHELOG}\" }\n  }\n}",
			ChangeType: "update",
			ChangeLog:  "Added grok pattern for Apache logs",
			ModifiedBy: "user1",
			ModifiedAt: baseTime.Add(24 * time.Hour),
		},
		{
			ID:         "history-003",
			ConfigID:   "test-filter-001",
			Version:    3,
			Content:    "filter {\n  grok {\n    match => { \"message\" => \"%{COMBINEDAPACHELOG}\" }\n  }\n  date {\n    match => [ \"timestamp\", \"dd/MMM/yyyy:HH:mm:ss Z\" ]\n  }\n}",
			ChangeType: "update",
			ChangeLog:  "Added date filter for timestamp parsing",
			ModifiedBy: "admin",
			ModifiedAt: baseTime.Add(48 * time.Hour),
		},
	}
}

// GetCreateConfigRequests returns various create config requests for testing
func GetCreateConfigRequests() map[string]*models.CreateConfigRequest {
	return map[string]*models.CreateConfigRequest{
		"valid_filter": {
			Name:        "new-filter",
			Type:        models.ConfigTypeFilter,
			Content:     "filter { mutate { add_tag => [\"processed\"] } }",
			Description: "Add processed tag",
			Tags:        []string{"mutate", "tag"},
		},
		"valid_input": {
			Name:        "file-input",
			Type:        models.ConfigTypeInput,
			Content:     "input { file { path => \"/var/log/app.log\" } }",
			Description: "Read from application log file",
			Tags:        []string{"file", "log"},
		},
		"valid_output": {
			Name:        "stdout-output",
			Type:        models.ConfigTypeOutput,
			Content:     "output { stdout { codec => rubydebug } }",
			Description: "Output to stdout for debugging",
			Tags:        []string{"debug", "stdout"},
		},
		"with_special_chars": {
			Name:        "config-with-特殊字符",
			Type:        models.ConfigTypeFilter,
			Content:     "filter { # 中文注释\n  mutate { add_field => { \"测试\" => \"值\" } } }",
			Description: "Configuration with unicode characters",
			Tags:        []string{"unicode", "测试"},
		},
	}
}

// GetUpdateConfigRequests returns various update config requests for testing
func GetUpdateConfigRequests() map[string]*models.UpdateConfigRequest {
	enableTrue := true
	enableFalse := false
	
	return map[string]*models.UpdateConfigRequest{
		"enable_config": {
			Enabled: &enableTrue,
		},
		"disable_config": {
			Enabled: &enableFalse,
		},
		"update_content": {
			Content: "filter { mutate { add_tag => [\"updated\"] } }",
		},
		"update_description": {
			Description: "Updated description",
		},
		"update_tags": {
			Tags: []string{"new", "tags", "updated"},
		},
		"update_all": {
			Name:        "updated-name",
			Type:        models.ConfigTypeFilter,
			Content:     "filter { # Updated content }",
			Description: "Completely updated config",
			Tags:        []string{"updated"},
			Enabled:     &enableTrue,
		},
	}
}

// GetConfigListRequests returns various list requests for testing
func GetConfigListRequests() map[string]*models.ConfigListRequest {
	enableTrue := true
	
	return map[string]*models.ConfigListRequest{
		"default": {
			Page:     1,
			PageSize: 10,
		},
		"with_type_filter": {
			Type:     models.ConfigTypeFilter,
			Page:     1,
			PageSize: 20,
		},
		"with_tags": {
			Tags:     []string{"kafka", "input"},
			Page:     1,
			PageSize: 10,
		},
		"enabled_only": {
			Enabled:  &enableTrue,
			Page:     1,
			PageSize: 50,
		},
		"large_page": {
			Page:     5,
			PageSize: 100,
		},
	}
}