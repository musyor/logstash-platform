package fixtures

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"logstash-platform/internal/platform/models"
)

func TestGetValidFilterConfig(t *testing.T) {
	config := GetValidFilterConfig()
	
	assert.NotNil(t, config)
	assert.Equal(t, "test-filter-001", config.ID)
	assert.Equal(t, "nginx-access-log-parser", config.Name)
	assert.Equal(t, models.ConfigTypeFilter, config.Type)
	assert.Contains(t, config.Content, "filter")
	assert.Contains(t, config.Content, "grok")
	assert.True(t, config.Enabled)
	assert.Equal(t, models.TestStatusPassed, config.TestStatus)
	assert.Len(t, config.Tags, 3)
}

func TestGetConfigList(t *testing.T) {
	configs := GetConfigList(30)
	
	assert.Len(t, configs, 30)
	
	// Check distribution of types
	filterCount := 0
	inputCount := 0
	outputCount := 0
	
	for _, config := range configs {
		switch config.Type {
		case models.ConfigTypeFilter:
			filterCount++
		case models.ConfigTypeInput:
			inputCount++
		case models.ConfigTypeOutput:
			outputCount++
		}
	}
	
	assert.Equal(t, 10, filterCount)
	assert.Equal(t, 10, inputCount)
	assert.Equal(t, 10, outputCount)
	
	// Check IDs are sequential
	for i, config := range configs {
		expectedID := fmt.Sprintf("config-%03d", i+1)
		assert.Equal(t, expectedID, config.ID)
	}
}

func TestGetInvalidConfigs(t *testing.T) {
	invalidConfigs := GetInvalidConfigs()
	
	assert.Len(t, invalidConfigs, 3)
	
	// Test empty content
	emptyConfig := invalidConfigs["empty_content"]
	assert.NotNil(t, emptyConfig)
	assert.Empty(t, emptyConfig.Content)
	
	// Test missing keyword
	missingKeyword := invalidConfigs["missing_keyword"]
	assert.NotNil(t, missingKeyword)
	assert.NotContains(t, missingKeyword.Content, "filter")
	
	// Test invalid type
	invalidType := invalidConfigs["invalid_type"]
	assert.NotNil(t, invalidType)
	assert.Equal(t, models.ConfigType("invalid"), invalidType.Type)
}

func TestGetConfigHistory(t *testing.T) {
	history := GetConfigHistory()
	
	assert.Len(t, history, 3)
	
	// Check versions are sequential
	for i, h := range history {
		assert.Equal(t, i+1, h.Version)
		assert.Equal(t, "test-filter-001", h.ConfigID)
	}
	
	// Check change types
	assert.Equal(t, "create", history[0].ChangeType)
	assert.Equal(t, "update", history[1].ChangeType)
	assert.Equal(t, "update", history[2].ChangeType)
	
	// Check chronological order
	for i := 1; i < len(history); i++ {
		assert.True(t, history[i].ModifiedAt.After(history[i-1].ModifiedAt))
	}
}

func TestGetTestAgent(t *testing.T) {
	agent := GetTestAgent()
	
	assert.NotNil(t, agent)
	assert.Equal(t, "agent-001", agent.ID)
	assert.Equal(t, "online", agent.Status)
	assert.Equal(t, "192.168.1.100", agent.IPAddress)
	assert.Equal(t, 5044, agent.Port)
	assert.Len(t, agent.ConfigFiles, 2)
	
	// Check LastSeen is recent
	assert.WithinDuration(t, time.Now(), agent.LastSeen, 1*time.Minute)
}

func TestGetAgentList(t *testing.T) {
	agents := GetAgentList()
	
	assert.Len(t, agents, 4)
	
	// Count statuses
	statusCount := map[string]int{}
	for _, agent := range agents {
		statusCount[agent.Status]++
	}
	
	assert.Equal(t, 2, statusCount["online"])
	assert.Equal(t, 1, statusCount["offline"])
	assert.Equal(t, 1, statusCount["error"])
	
	// Check error agent has error message
	for _, agent := range agents {
		if agent.Status == "error" {
			assert.NotEmpty(t, agent.ErrorMessage)
		}
	}
}

func TestHelpers(t *testing.T) {
	t.Run("GenerateTestID", func(t *testing.T) {
		id1 := GenerateTestID("test")
		id2 := GenerateTestID("test")
		
		assert.NotEqual(t, id1, id2)
		assert.Contains(t, id1, "test-")
	})
	
	t.Run("CreateTestTags", func(t *testing.T) {
		tags := CreateTestTags(5)
		
		assert.Len(t, tags, 5)
		for i, tag := range tags {
			assert.Equal(t, fmt.Sprintf("tag-%d", i+1), tag)
		}
	})
	
	t.Run("Pointer helpers", func(t *testing.T) {
		strPtr := GetStringPointer("test")
		assert.NotNil(t, strPtr)
		assert.Equal(t, "test", *strPtr)
		
		boolPtr := GetBoolPointer(true)
		assert.NotNil(t, boolPtr)
		assert.True(t, *boolPtr)
		
		intPtr := GetIntPointer(42)
		assert.NotNil(t, intPtr)
		assert.Equal(t, 42, *intPtr)
		
		now := time.Now()
		timePtr := GetTimePointer(now)
		assert.NotNil(t, timePtr)
		assert.Equal(t, now, *timePtr)
	})
	
	t.Run("CompareJSON", func(t *testing.T) {
		json1 := `{"name":"test","value":123,"tags":["a","b"]}`
		json2 := `{"tags":["a","b"],"name":"test","value":123}`
		json3 := `{"name":"test","value":456,"tags":["a","b"]}`
		
		assert.True(t, CompareJSON(t, json1, json2))
		assert.False(t, CompareJSON(t, json1, json3))
	})
}

func TestCreateConfigRequests(t *testing.T) {
	requests := GetCreateConfigRequests()
	
	assert.Len(t, requests, 4)
	
	// Test valid filter request
	filterReq := requests["valid_filter"]
	assert.NotNil(t, filterReq)
	assert.Equal(t, models.ConfigTypeFilter, filterReq.Type)
	assert.Contains(t, filterReq.Content, "filter")
	
	// Test unicode support
	unicodeReq := requests["with_special_chars"]
	assert.NotNil(t, unicodeReq)
	assert.Contains(t, unicodeReq.Name, "特殊字符")
	assert.Contains(t, unicodeReq.Content, "中文注释")
	assert.Contains(t, unicodeReq.Tags, "测试")
}

func TestUpdateConfigRequests(t *testing.T) {
	requests := GetUpdateConfigRequests()
	
	assert.Len(t, requests, 6)
	
	// Test enable/disable
	enableReq := requests["enable_config"]
	assert.NotNil(t, enableReq)
	assert.NotNil(t, enableReq.Enabled)
	assert.True(t, *enableReq.Enabled)
	
	disableReq := requests["disable_config"]
	assert.NotNil(t, disableReq)
	assert.NotNil(t, disableReq.Enabled)
	assert.False(t, *disableReq.Enabled)
	
	// Test full update
	fullReq := requests["update_all"]
	assert.NotNil(t, fullReq)
	assert.NotEmpty(t, fullReq.Name)
	assert.NotEmpty(t, fullReq.Content)
	assert.NotEmpty(t, fullReq.Description)
	assert.NotEmpty(t, fullReq.Tags)
}