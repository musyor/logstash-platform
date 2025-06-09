package fixtures

import (
	"time"
)

// AgentTestData represents test data for agents (placeholder for future implementation)
type AgentTestData struct {
	ID           string
	Name         string
	Version      string
	Status       string
	IPAddress    string
	Port         int
	ConfigFiles  []string
	LastSeen     time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ErrorMessage string
}

// GetTestAgent returns test agent data
func GetTestAgent() *AgentTestData {
	return &AgentTestData{
		ID:          "agent-001",
		Name:        "test-agent-01",
		Version:     "8.11.0",
		Status:      "online",
		IPAddress:   "192.168.1.100",
		Port:        5044,
		ConfigFiles: []string{"config-001", "config-002"},
		LastSeen:    time.Now().Add(-30 * time.Second),
		CreatedAt:   time.Now().Add(-7 * 24 * time.Hour),
		UpdatedAt:   time.Now().Add(-30 * time.Second),
	}
}

// GetAgentList returns a list of test agents
func GetAgentList() []*AgentTestData {
	baseTime := time.Now()
	
	return []*AgentTestData{
		{
			ID:          "agent-001",
			Name:        "production-agent-01",
			Version:     "8.11.0",
			Status:      "online",
			IPAddress:   "192.168.1.100",
			Port:        5044,
			ConfigFiles: []string{"config-001", "config-002", "config-003"},
			LastSeen:    baseTime.Add(-30 * time.Second),
			CreatedAt:   baseTime.Add(-30 * 24 * time.Hour),
			UpdatedAt:   baseTime.Add(-30 * time.Second),
		},
		{
			ID:          "agent-002",
			Name:        "production-agent-02",
			Version:     "8.11.0",
			Status:      "online",
			IPAddress:   "192.168.1.101",
			Port:        5044,
			ConfigFiles: []string{"config-001", "config-004"},
			LastSeen:    baseTime.Add(-45 * time.Second),
			CreatedAt:   baseTime.Add(-25 * 24 * time.Hour),
			UpdatedAt:   baseTime.Add(-45 * time.Second),
		},
		{
			ID:          "agent-003",
			Name:        "staging-agent-01",
			Version:     "8.10.0",
			Status:      "offline",
			IPAddress:   "192.168.2.100",
			Port:        5044,
			ConfigFiles: []string{"config-005"},
			LastSeen:    baseTime.Add(-5 * time.Minute),
			CreatedAt:   baseTime.Add(-20 * 24 * time.Hour),
			UpdatedAt:   baseTime.Add(-5 * time.Minute),
		},
		{
			ID:           "agent-004",
			Name:         "development-agent-01",
			Version:      "8.12.0-beta",
			Status:       "error",
			IPAddress:    "192.168.3.100",
			Port:         5044,
			ConfigFiles:  []string{},
			LastSeen:     baseTime.Add(-2 * time.Hour),
			CreatedAt:    baseTime.Add(-10 * 24 * time.Hour),
			UpdatedAt:    baseTime.Add(-2 * time.Hour),
			ErrorMessage: "Configuration validation failed",
		},
	}
}

// GetAgentStatusCounts returns agent status counts for testing
func GetAgentStatusCounts() map[string]int {
	return map[string]int{
		"online":  25,
		"offline": 5,
		"error":   2,
		"total":   32,
	}
}

// AgentMetricsTestData represents test metrics data
type AgentMetricsTestData struct {
	AgentID        string
	CPUUsage       float64
	MemoryUsage    float64
	DiskUsage      float64
	EventsReceived int64
	EventsSent     int64
	EventsFailed   int64
	Uptime         int64
	Timestamp      time.Time
}

// GetAgentMetrics returns test agent metrics
func GetAgentMetrics() *AgentMetricsTestData {
	return &AgentMetricsTestData{
		AgentID:        "agent-001",
		CPUUsage:       45.2,
		MemoryUsage:    68.5,
		DiskUsage:      82.1,
		EventsReceived: 150000,
		EventsSent:     149850,
		EventsFailed:   150,
		Uptime:         86400, // 24 hours in seconds
		Timestamp:      time.Now(),
	}
}