package fixtures

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// LoadJSONFixture loads a JSON fixture from the test_config.json file
func LoadJSONFixture(t *testing.T, fixtureName string) map[string]interface{} {
	t.Helper()
	
	// In a real implementation, this would read from test_config.json
	// For now, we'll return predefined data
	fixtures := map[string]map[string]interface{}{
		"valid_filter_config": {
			"name":        "nginx-access-log-parser",
			"type":        "filter",
			"content":     "filter {\n  grok {\n    match => { \"message\" => \"%{COMBINEDAPACHELOG}\" }\n  }\n}",
			"description": "Parse nginx access logs",
			"tags":        []string{"nginx", "access-log", "parser"},
		},
		"valid_input_config": {
			"name":        "kafka-input",
			"type":        "input",
			"content":     "input {\n  kafka {\n    bootstrap_servers => \"localhost:9092\"\n    topics => [\"app-logs\"]\n  }\n}",
			"description": "Kafka input configuration",
			"tags":        []string{"kafka", "input"},
		},
		"valid_output_config": {
			"name":        "elasticsearch-output",
			"type":        "output",
			"content":     "output {\n  elasticsearch {\n    hosts => [\"localhost:9200\"\n    index => \"logs-%{+YYYY.MM.dd}\"\n  }\n}",
			"description": "Elasticsearch output configuration",
			"tags":        []string{"elasticsearch", "output"},
		},
	}
	
	if fixture, ok := fixtures[fixtureName]; ok {
		return fixture
	}
	
	t.Fatalf("fixture %s not found", fixtureName)
	return nil
}

// CompareJSON compares two JSON strings for equality
func CompareJSON(t *testing.T, expected, actual string) bool {
	t.Helper()
	
	var expectedObj, actualObj interface{}
	
	if err := json.Unmarshal([]byte(expected), &expectedObj); err != nil {
		t.Errorf("failed to unmarshal expected JSON: %v", err)
		return false
	}
	
	if err := json.Unmarshal([]byte(actual), &actualObj); err != nil {
		t.Errorf("failed to unmarshal actual JSON: %v", err)
		return false
	}
	
	expectedBytes, _ := json.Marshal(expectedObj)
	actualBytes, _ := json.Marshal(actualObj)
	
	return string(expectedBytes) == string(actualBytes)
}

// GenerateTestID generates a unique test ID
func GenerateTestID(prefix string) string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s-%d", prefix, timestamp)
}

// CreateTestTags generates test tags based on count
func CreateTestTags(count int) []string {
	tags := make([]string, count)
	for i := 0; i < count; i++ {
		tags[i] = fmt.Sprintf("tag-%d", i+1)
	}
	return tags
}

// GetTimePointer returns a pointer to a time value
func GetTimePointer(t time.Time) *time.Time {
	return &t
}

// GetStringPointer returns a pointer to a string value
func GetStringPointer(s string) *string {
	return &s
}

// GetBoolPointer returns a pointer to a bool value
func GetBoolPointer(b bool) *bool {
	return &b
}

// GetIntPointer returns a pointer to an int value
func GetIntPointer(i int) *int {
	return &i
}

// AssertErrorContains checks if an error contains a specific message
func AssertErrorContains(t *testing.T, err error, contains string) {
	t.Helper()
	
	if err == nil {
		t.Errorf("expected error containing '%s', but got nil", contains)
		return
	}
	
	if !strings.Contains(err.Error(), contains) {
		t.Errorf("expected error containing '%s', but got '%s'", contains, err.Error())
	}
}

// AssertTimeWithinDuration checks if two times are within a specified duration
func AssertTimeWithinDuration(t *testing.T, expected, actual time.Time, delta time.Duration) {
	t.Helper()
	
	diff := expected.Sub(actual)
	if diff < 0 {
		diff = -diff
	}
	
	if diff > delta {
		t.Errorf("times differ by %v, which exceeds %v", diff, delta)
	}
}

// CleanupTestData provides a cleanup function for test data
func CleanupTestData(t *testing.T, cleanupFuncs ...func()) {
	t.Helper()
	
	t.Cleanup(func() {
		for _, fn := range cleanupFuncs {
			fn()
		}
	})
}