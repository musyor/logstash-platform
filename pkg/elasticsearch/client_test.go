package elasticsearch

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func setupTestConfig() {
	viper.Reset()
	viper.Set("elasticsearch.addresses", []string{"http://localhost:9200"})
	viper.Set("elasticsearch.max_retries", 3)
	viper.Set("elasticsearch.timeout", 30*time.Second)
	viper.Set("elasticsearch.indices.configs", "test_logstash_configs")
	viper.Set("elasticsearch.indices.config_history", "test_logstash_config_history")
	viper.Set("elasticsearch.indices.agents", "test_logstash_agents")
}

func TestNewClient(t *testing.T) {
	setupTestConfig()
	logger := logrus.New()

	tests := []struct {
		name    string
		setup   func()
		wantErr bool
	}{
		{
			name: "successful connection",
			setup: func() {
				// Use default test config
			},
			wantErr: false,
		},
		{
			name: "invalid address",
			setup: func() {
				viper.Set("elasticsearch.addresses", []string{"http://invalid-host:9999"})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			_, err := NewClient(logger)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_IndexExists(t *testing.T) {
	setupTestConfig()
	logger := logrus.New()

	// Skip if no ES instance available
	client, err := NewClient(logger)
	if err != nil {
		t.Skip("Elasticsearch not available")
	}

	ctx := context.Background()
	
	tests := []struct {
		name    string
		index   string
		want    bool
		wantErr bool
	}{
		{
			name:    "check non-existent index",
			index:   "non_existent_index_12345",
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.IndexExists(ctx, tt.index)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestClient_CreateIndex(t *testing.T) {
	setupTestConfig()
	logger := logrus.New()

	// Skip if no ES instance available
	client, err := NewClient(logger)
	if err != nil {
		t.Skip("Elasticsearch not available")
	}

	ctx := context.Background()
	testIndex := "test_index_" + time.Now().Format("20060102150405")
	
	// Clean up
	defer func() {
		_ = client.Delete(ctx, testIndex, "")
	}()

	tests := []struct {
		name    string
		index   string
		mapping string
		wantErr bool
	}{
		{
			name:  "create valid index",
			index: testIndex,
			mapping: `{
				"mappings": {
					"properties": {
						"test_field": { "type": "keyword" }
					}
				}
			}`,
			wantErr: false,
		},
		{
			name:    "create with invalid mapping",
			index:   testIndex + "_invalid",
			mapping: `{"invalid": "json"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.CreateIndex(ctx, tt.index, tt.mapping)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}