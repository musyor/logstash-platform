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

func TestConfigHandler_GetConfig(t *testing.T) {
	logger := logrus.New()
	
	tests := []struct {
		name         string
		id           string
		setup        func(*MockConfigService)
		expectedCode int
		checkBody    func(*testing.T, map[string]interface{})
	}{
		{
			name: "successful get",
			id:   "config-123",
			setup: func(m *MockConfigService) {
				m.On("GetConfig", mock.Anything, "config-123").
					Return(&models.Config{
						ID:      "config-123",
						Name:    "test-config",
						Type:    models.ConfigTypeFilter,
						Content: "filter { }",
						Version: 1,
						Enabled: true,
					}, nil)
			},
			expectedCode: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "config-123", body["id"])
				assert.Equal(t, "test-config", body["name"])
			},
		},
		{
			name: "config not found",
			id:   "non-existent",
			setup: func(m *MockConfigService) {
				m.On("GetConfig", mock.Anything, "non-existent").
					Return(nil, assert.AnError)
			},
			expectedCode: http.StatusInternalServerError,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "INTERNAL_ERROR", body["code"])
			},
		},
		{
			name:         "empty id",
			id:           "",
			setup:        func(m *MockConfigService) {},
			expectedCode: http.StatusNotFound, // Gin returns 404 for missing route parameter
			checkBody:    func(t *testing.T, body map[string]interface{}) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := new(MockConfigService)
			tt.setup(mockService)
			
			handler := NewConfigHandler(mockService, logger)
			router := setupTestRouter()
			router.GET("/configs/:id", handler.GetConfig)

			// Prepare request
			req := httptest.NewRequest("GET", "/configs/"+tt.id, nil)
			
			// Execute
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, tt.expectedCode, w.Code)
			
			if w.Code != http.StatusNotFound {
				var responseBody map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &responseBody)
				assert.NoError(t, err)
				tt.checkBody(t, responseBody)
			}
			
			mockService.AssertExpectations(t)
		})
	}
}

func TestConfigHandler_UpdateConfig(t *testing.T) {
	logger := logrus.New()
	
	tests := []struct {
		name         string
		id           string
		body         interface{}
		setup        func(*MockConfigService)
		expectedCode int
		checkBody    func(*testing.T, map[string]interface{})
	}{
		{
			name: "successful update",
			id:   "config-123",
			body: map[string]interface{}{
				"name":        "updated-config",
				"type":        "filter",
				"content":     "filter { updated }",
				"description": "Updated config",
				"enabled":     true,
			},
			setup: func(m *MockConfigService) {
				m.On("UpdateConfig", mock.Anything, "config-123", mock.AnythingOfType("*models.UpdateConfigRequest"), "admin").
					Return(&models.Config{
						ID:      "config-123",
						Name:    "updated-config",
						Type:    models.ConfigTypeFilter,
						Content: "filter { updated }",
						Version: 2,
						Enabled: true,
					}, nil)
			},
			expectedCode: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "config-123", body["id"])
				assert.Equal(t, "updated-config", body["name"])
				assert.Equal(t, float64(2), body["version"])
			},
		},
		{
			name: "invalid request body",
			id:   "config-123",
			body: "invalid json",
			setup:        func(m *MockConfigService) {},
			expectedCode: http.StatusBadRequest,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "INVALID_REQUEST", body["code"])
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
			router.PUT("/configs/:id", handler.UpdateConfig)

			// Prepare request
			var bodyBytes []byte
			if str, ok := tt.body.(string); ok {
				bodyBytes = []byte(str)
			} else {
				bodyBytes, _ = json.Marshal(tt.body)
			}
			req := httptest.NewRequest("PUT", "/configs/"+tt.id, bytes.NewBuffer(bodyBytes))
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

func TestConfigHandler_DeleteConfig(t *testing.T) {
	logger := logrus.New()
	
	tests := []struct {
		name         string
		id           string
		setup        func(*MockConfigService)
		expectedCode int
	}{
		{
			name: "successful delete",
			id:   "config-123",
			setup: func(m *MockConfigService) {
				m.On("DeleteConfig", mock.Anything, "config-123").Return(nil)
			},
			expectedCode: http.StatusNoContent,
		},
		{
			name: "delete failure",
			id:   "config-123",
			setup: func(m *MockConfigService) {
				m.On("DeleteConfig", mock.Anything, "config-123").Return(assert.AnError)
			},
			expectedCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := new(MockConfigService)
			tt.setup(mockService)
			
			handler := NewConfigHandler(mockService, logger)
			router := setupTestRouter()
			router.DELETE("/configs/:id", handler.DeleteConfig)

			// Prepare request
			req := httptest.NewRequest("DELETE", "/configs/"+tt.id, nil)
			
			// Execute
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, tt.expectedCode, w.Code)
			mockService.AssertExpectations(t)
		})
	}
}

func TestConfigHandler_GetConfigHistory(t *testing.T) {
	logger := logrus.New()
	
	tests := []struct {
		name         string
		id           string
		setup        func(*MockConfigService)
		expectedCode int
		checkBody    func(*testing.T, map[string]interface{})
	}{
		{
			name: "successful get history",
			id:   "config-123",
			setup: func(m *MockConfigService) {
				m.On("GetConfigHistory", mock.Anything, "config-123").
					Return([]*models.ConfigHistory{
						{
							ID:         "history-1",
							ConfigID:   "config-123",
							Version:    2,
							Content:    "filter { new }",
							ChangeType: "update",
							ChangeLog:  "Updated filter",
							ModifiedBy: "admin",
						},
						{
							ID:         "history-2",
							ConfigID:   "config-123",
							Version:    1,
							Content:    "filter { old }",
							ChangeType: "create",
							ChangeLog:  "Created filter",
							ModifiedBy: "admin",
						},
					}, nil)
			},
			expectedCode: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, float64(2), body["total"])
				items := body["items"].([]interface{})
				assert.Len(t, items, 2)
			},
		},
		{
			name: "empty history",
			id:   "config-123",
			setup: func(m *MockConfigService) {
				m.On("GetConfigHistory", mock.Anything, "config-123").
					Return([]*models.ConfigHistory{}, nil)
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
			router.GET("/configs/:id/history", handler.GetConfigHistory)

			// Prepare request
			req := httptest.NewRequest("GET", "/configs/"+tt.id+"/history", nil)
			
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

func TestConfigHandler_RollbackConfig(t *testing.T) {
	logger := logrus.New()
	
	tests := []struct {
		name         string
		id           string
		body         interface{}
		setup        func(*MockConfigService)
		expectedCode int
		checkBody    func(*testing.T, map[string]interface{})
	}{
		{
			name: "successful rollback",
			id:   "config-123",
			body: map[string]interface{}{
				"version": 1,
			},
			setup: func(m *MockConfigService) {
				m.On("RollbackConfig", mock.Anything, "config-123", 1, "admin").
					Return(&models.Config{
						ID:      "config-123",
						Name:    "rolled-back-config",
						Type:    models.ConfigTypeFilter,
						Content: "filter { old }",
						Version: 3,
					}, nil)
			},
			expectedCode: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "config-123", body["id"])
				assert.Equal(t, float64(3), body["version"])
				assert.Equal(t, "filter { old }", body["content"])
			},
		},
		{
			name: "invalid version",
			id:   "config-123",
			body: map[string]interface{}{
				"version": 0,
			},
			setup:        func(m *MockConfigService) {},
			expectedCode: http.StatusBadRequest,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "INVALID_REQUEST", body["code"])
			},
		},
		{
			name: "missing version",
			id:   "config-123",
			body: map[string]interface{}{},
			setup:        func(m *MockConfigService) {},
			expectedCode: http.StatusBadRequest,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "INVALID_REQUEST", body["code"])
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
			router.POST("/configs/:id/rollback", handler.RollbackConfig)

			// Prepare request
			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/configs/"+tt.id+"/rollback", bytes.NewBuffer(bodyBytes))
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