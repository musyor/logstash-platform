package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"logstash-platform/internal/platform/models"
)

// MockConfigService is a mock implementation of ConfigService
type MockConfigService struct {
	mock.Mock
}

func (m *MockConfigService) CreateConfig(ctx context.Context, req *models.CreateConfigRequest, userID string) (*models.Config, error) {
	args := m.Called(ctx, req, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Config), args.Error(1)
}

func (m *MockConfigService) UpdateConfig(ctx context.Context, id string, req *models.UpdateConfigRequest, userID string) (*models.Config, error) {
	args := m.Called(ctx, id, req, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Config), args.Error(1)
}

func (m *MockConfigService) DeleteConfig(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockConfigService) GetConfig(ctx context.Context, id string) (*models.Config, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Config), args.Error(1)
}

func (m *MockConfigService) ListConfigs(ctx context.Context, req *models.ConfigListRequest) (*models.ConfigListResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ConfigListResponse), args.Error(1)
}

func (m *MockConfigService) GetConfigHistory(ctx context.Context, configID string) ([]*models.ConfigHistory, error) {
	args := m.Called(ctx, configID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.ConfigHistory), args.Error(1)
}

func (m *MockConfigService) RollbackConfig(ctx context.Context, configID string, version int, userID string) (*models.Config, error) {
	args := m.Called(ctx, configID, version, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Config), args.Error(1)
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestConfigHandler_CreateConfig(t *testing.T) {
	logger := logrus.New()
	
	tests := []struct {
		name         string
		body         interface{}
		setup        func(*MockConfigService)
		expectedCode int
		checkBody    func(*testing.T, map[string]interface{})
	}{
		{
			name: "successful creation",
			body: map[string]interface{}{
				"name":        "test-config",
				"type":        "filter",
				"content":     "filter { }",
				"description": "Test config",
				"tags":        []string{"test"},
			},
			setup: func(m *MockConfigService) {
				m.On("CreateConfig", mock.Anything, mock.AnythingOfType("*models.CreateConfigRequest"), "admin").
					Return(&models.Config{
						ID:          "config-123",
						Name:        "test-config",
						Type:        models.ConfigTypeFilter,
						Content:     "filter { }",
						Description: "Test config",
						Tags:        []string{"test"},
						Version:     1,
						Enabled:     true,
					}, nil)
			},
			expectedCode: http.StatusCreated,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "config-123", body["id"])
				assert.Equal(t, "test-config", body["name"])
				assert.Equal(t, "filter", body["type"])
			},
		},
		{
			name: "invalid request body",
			body: map[string]interface{}{
				"name": "test-config",
				// Missing required fields
			},
			setup:        func(m *MockConfigService) {},
			expectedCode: http.StatusBadRequest,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "INVALID_REQUEST", body["code"])
			},
		},
		{
			name: "service error",
			body: map[string]interface{}{
				"name":    "test-config",
				"type":    "filter",
				"content": "filter { }",
			},
			setup: func(m *MockConfigService) {
				m.On("CreateConfig", mock.Anything, mock.AnythingOfType("*models.CreateConfigRequest"), "admin").
					Return(nil, assert.AnError)
			},
			expectedCode: http.StatusInternalServerError,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "CREATE_FAILED", body["code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := new(MockConfigService)
			tt.setup(mockService)
			
			handler := NewConfigHandler(mockService, logger)
			router := setupTestRouter()
			router.POST("/configs", handler.CreateConfig)

			// Prepare request
			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/configs", bytes.NewBuffer(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			
			// Execute
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, tt.expectedCode, w.Code)
			
			var responseBody map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &responseBody)
			assert.NoError(t, err)
			
			tt.checkBody(t, responseBody)
			mockService.AssertExpectations(t)
		})
	}
}

func TestConfigHandler_ListConfigs(t *testing.T) {
	logger := logrus.New()
	
	tests := []struct {
		name         string
		query        string
		setup        func(*MockConfigService)
		expectedCode int
		checkBody    func(*testing.T, map[string]interface{})
	}{
		{
			name:  "successful list",
			query: "?type=filter&page=1&size=10",
			setup: func(m *MockConfigService) {
				m.On("ListConfigs", mock.Anything, mock.MatchedBy(func(req *models.ConfigListRequest) bool {
					return req.Type == models.ConfigTypeFilter && req.Page == 1 && req.PageSize == 10
				})).Return(&models.ConfigListResponse{
					Total: 2,
					Page:  1,
					Size:  10,
					Items: []*models.Config{
						{ID: "1", Name: "config1"},
						{ID: "2", Name: "config2"},
					},
				}, nil)
			},
			expectedCode: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, float64(2), body["total"])
				assert.Equal(t, float64(1), body["page"])
				items := body["items"].([]interface{})
				assert.Len(t, items, 2)
			},
		},
		{
			name:  "empty result",
			query: "",
			setup: func(m *MockConfigService) {
				m.On("ListConfigs", mock.Anything, mock.Anything).
					Return(&models.ConfigListResponse{
						Total: 0,
						Page:  1,
						Size:  10,
						Items: []*models.Config{},
					}, nil)
			},
			expectedCode: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, float64(0), body["total"])
				items := body["items"].([]interface{})
				assert.Len(t, items, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := new(MockConfigService)
			tt.setup(mockService)
			
			handler := NewConfigHandler(mockService, logger)
			router := setupTestRouter()
			router.GET("/configs", handler.ListConfigs)

			// Prepare request
			req := httptest.NewRequest("GET", "/configs"+tt.query, nil)
			
			// Execute
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, tt.expectedCode, w.Code)
			
			var responseBody map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &responseBody)
			assert.NoError(t, err)
			
			tt.checkBody(t, responseBody)
			mockService.AssertExpectations(t)
		})
	}
}