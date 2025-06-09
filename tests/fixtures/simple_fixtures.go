// Package fixtures provides test data for unit and integration tests
package fixtures

import (
	"logstash-platform/internal/platform/models"
)

// SimpleConfig returns a basic config for testing
func SimpleConfig() *models.Config {
	return &models.Config{
		ID:          "test-001",
		Name:        "simple-filter",
		Type:        models.ConfigTypeFilter,
		Content:     "filter { mutate { add_tag => [\"test\"] } }",
		Description: "Simple test filter",
		Tags:        []string{"test"},
		Version:     1,
		Enabled:     true,
		TestStatus:  models.TestStatusPassed,
		CreatedBy:   "test-user",
		UpdatedBy:   "test-user",
	}
}

// SimpleCreateRequest returns a basic create request
func SimpleCreateRequest() *models.CreateConfigRequest {
	return &models.CreateConfigRequest{
		Name:        "new-config",
		Type:        models.ConfigTypeFilter,
		Content:     "filter { }",
		Description: "New config",
		Tags:        []string{"new"},
	}
}

// SimpleUpdateRequest returns a basic update request
func SimpleUpdateRequest() *models.UpdateConfigRequest {
	enabled := true
	return &models.UpdateConfigRequest{
		Name:        "updated-config",
		Type:        models.ConfigTypeFilter,
		Content:     "filter { updated }",
		Description: "Updated config",
		Tags:        []string{"updated"},
		Enabled:     &enabled,
	}
}

// SimpleListRequest returns a basic list request
func SimpleListRequest() *models.ConfigListRequest {
	enabled := true
	return &models.ConfigListRequest{
		Type:     models.ConfigTypeFilter,
		Tags:     []string{"test"},
		Enabled:  &enabled,
		Page:     1,
		PageSize: 10,
	}
}

// SimpleHistory returns a basic history entry
func SimpleHistory() *models.ConfigHistory {
	return &models.ConfigHistory{
		ID:         "history-001",
		ConfigID:   "test-001",
		Version:    1,
		Content:    "filter { }",
		ChangeType: "create",
		ChangeLog:  "Initial creation",
		ModifiedBy: "test-user",
	}
}