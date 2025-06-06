package service

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"logstash-platform/internal/platform/models"
	"logstash-platform/tests/mocks"
)

func TestConfigService_CreateConfig(t *testing.T) {
	logger := logrus.New()
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *models.CreateConfigRequest
		userID  string
		setup   func(*mocks.MockConfigRepository)
		want    *models.Config
		wantErr bool
	}{
		{
			name: "successful creation",
			req: &models.CreateConfigRequest{
				Name:        "test-filter",
				Description: "Test filter config",
				Type:        models.ConfigTypeFilter,
				Content:     "filter { mutate { add_field => { \"test\" => \"value\" } } }",
				Tags:        []string{"test", "filter"},
			},
			userID: "user123",
			setup: func(m *mocks.MockConfigRepository) {
				m.On("Create", ctx, mock.MatchedBy(func(cfg *models.Config) bool {
					return cfg.Name == "test-filter" &&
						cfg.Type == models.ConfigTypeFilter &&
						cfg.CreatedBy == "user123"
				})).Return(nil)
			},
			want: &models.Config{
				Name:        "test-filter",
				Type:        models.ConfigTypeFilter,
				Description: "Test filter config",
			},
			wantErr: false,
		},
		{
			name: "empty content validation",
			req: &models.CreateConfigRequest{
				Name:    "empty-config",
				Type:    models.ConfigTypeFilter,
				Content: "",
			},
			userID:  "user123",
			setup:   func(m *mocks.MockConfigRepository) {},
			want:    nil,
			wantErr: true,
		},
		{
			name: "repository error",
			req: &models.CreateConfigRequest{
				Name:    "test-config",
				Type:    models.ConfigTypeInput,
				Content: "input { stdin {} }",
			},
			userID: "user123",
			setup: func(m *mocks.MockConfigRepository) {
				m.On("Create", ctx, mock.Anything).Return(assert.AnError)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mocks.MockConfigRepository)
			tt.setup(mockRepo)

			service := NewConfigService(mockRepo, logger)
			got, err := service.CreateConfig(ctx, tt.req, tt.userID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.want.Name, got.Name)
				assert.Equal(t, tt.want.Type, got.Type)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestConfigService_UpdateConfig(t *testing.T) {
	logger := logrus.New()
	ctx := context.Background()

	existingConfig := &models.Config{
		ID:          "config-123",
		Name:        "existing-config",
		Type:        models.ConfigTypeFilter,
		Content:     "filter { old }",
		Version:     1,
		CreatedAt:   time.Now().Add(-24 * time.Hour),
		CreatedBy:   "user1",
		TestStatus:  models.TestStatusPassed,
	}

	tests := []struct {
		name    string
		id      string
		req     *models.UpdateConfigRequest
		userID  string
		setup   func(*mocks.MockConfigRepository)
		wantErr bool
	}{
		{
			name: "successful update",
			id:   "config-123",
			req: &models.UpdateConfigRequest{
				Name:        "updated-config",
				Description: "Updated description",
				Type:        models.ConfigTypeFilter,
				Content:     "filter { new }",
				Tags:        []string{"updated"},
			},
			userID: "user2",
			setup: func(m *mocks.MockConfigRepository) {
				m.On("GetByID", ctx, "config-123").Return(existingConfig, nil)
				m.On("Update", ctx, mock.MatchedBy(func(cfg *models.Config) bool {
					return cfg.ID == "config-123" &&
						cfg.Name == "updated-config" &&
						cfg.UpdatedBy == "user2" &&
						cfg.Version == 1 && // Version not incremented by service
						cfg.TestStatus == models.TestStatusPassed // Service doesn't change TestStatus, repository does
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "config not found",
			id:   "non-existent",
			req: &models.UpdateConfigRequest{
				Name:    "updated",
				Type:    models.ConfigTypeFilter,
				Content: "filter {}",
			},
			userID: "user2",
			setup: func(m *mocks.MockConfigRepository) {
				m.On("GetByID", ctx, "non-existent").Return(nil, assert.AnError)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mocks.MockConfigRepository)
			tt.setup(mockRepo)

			service := NewConfigService(mockRepo, logger)
			got, err := service.UpdateConfig(ctx, tt.id, tt.req, tt.userID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestConfigService_GetConfigHistory(t *testing.T) {
	logger := logrus.New()
	ctx := context.Background()

	tests := []struct {
		name     string
		configID string
		setup    func(*mocks.MockConfigRepository)
		want     int
		wantErr  bool
	}{
		{
			name:     "successful history retrieval",
			configID: "config-123",
			setup: func(m *mocks.MockConfigRepository) {
				m.On("GetByID", ctx, "config-123").Return(&models.Config{ID: "config-123"}, nil)
				m.On("GetHistory", ctx, "config-123").Return([]*models.ConfigHistory{
					{
						ConfigID:   "config-123",
						Version:    2,
						ChangeType: "update",
					},
					{
						ConfigID:   "config-123",
						Version:    1,
						ChangeType: "create",
					},
				}, nil)
			},
			want:    2,
			wantErr: false,
		},
		{
			name:     "config not found",
			configID: "non-existent",
			setup: func(m *mocks.MockConfigRepository) {
				m.On("GetByID", ctx, "non-existent").Return(nil, assert.AnError)
			},
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mocks.MockConfigRepository)
			tt.setup(mockRepo)

			service := NewConfigService(mockRepo, logger)
			got, err := service.GetConfigHistory(ctx, tt.configID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, got, tt.want)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestConfigService_DeleteConfig(t *testing.T) {
	logger := logrus.New()
	ctx := context.Background()

	tests := []struct {
		name    string
		id      string
		setup   func(*mocks.MockConfigRepository)
		wantErr bool
	}{
		{
			name: "successful deletion",
			id:   "config-123",
			setup: func(m *mocks.MockConfigRepository) {
				m.On("Delete", ctx, "config-123").Return(nil)
			},
			wantErr: false,
		},
		{
			name: "repository error",
			id:   "config-123",
			setup: func(m *mocks.MockConfigRepository) {
				m.On("Delete", ctx, "config-123").Return(assert.AnError)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mocks.MockConfigRepository)
			tt.setup(mockRepo)

			service := NewConfigService(mockRepo, logger)
			err := service.DeleteConfig(ctx, tt.id)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestConfigService_GetConfig(t *testing.T) {
	logger := logrus.New()
	ctx := context.Background()

	tests := []struct {
		name    string
		id      string
		setup   func(*mocks.MockConfigRepository)
		want    *models.Config
		wantErr bool
	}{
		{
			name: "successful get",
			id:   "config-123",
			setup: func(m *mocks.MockConfigRepository) {
				config := &models.Config{
					ID:      "config-123",
					Name:    "test-config",
					Type:    models.ConfigTypeFilter,
					Content: "filter { }",
				}
				m.On("GetByID", ctx, "config-123").Return(config, nil)
			},
			want: &models.Config{
				ID:      "config-123",
				Name:    "test-config",
				Type:    models.ConfigTypeFilter,
				Content: "filter { }",
			},
			wantErr: false,
		},
		{
			name: "config not found",
			id:   "non-existent",
			setup: func(m *mocks.MockConfigRepository) {
				m.On("GetByID", ctx, "non-existent").Return(nil, assert.AnError)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mocks.MockConfigRepository)
			tt.setup(mockRepo)

			service := NewConfigService(mockRepo, logger)
			got, err := service.GetConfig(ctx, tt.id)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.ID, got.ID)
				assert.Equal(t, tt.want.Name, got.Name)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestConfigService_ListConfigs(t *testing.T) {
	logger := logrus.New()
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *models.ConfigListRequest
		setup   func(*mocks.MockConfigRepository)
		want    *models.ConfigListResponse
		wantErr bool
	}{
		{
			name: "successful list with defaults",
			req: &models.ConfigListRequest{
				Page:     0, // Should be set to 1
				PageSize: 0, // Should be set to 10
			},
			setup: func(m *mocks.MockConfigRepository) {
				response := &models.ConfigListResponse{
					Total: 2,
					Page:  1,
					Size:  10,
					Items: []*models.Config{
						{ID: "config-1", Name: "Config 1"},
						{ID: "config-2", Name: "Config 2"},
					},
				}
				m.On("List", ctx, mock.MatchedBy(func(req *models.ConfigListRequest) bool {
					return req.Page == 1 && req.PageSize == 10
				})).Return(response, nil)
			},
			want: &models.ConfigListResponse{
				Total: 2,
				Page:  1,
				Size:  10,
			},
			wantErr: false,
		},
		{
			name: "page size too large",
			req: &models.ConfigListRequest{
				Page:     1,
				PageSize: 200, // Should be capped at 10
			},
			setup: func(m *mocks.MockConfigRepository) {
				response := &models.ConfigListResponse{
					Total: 0,
					Page:  1,
					Size:  10,
					Items: []*models.Config{},
				}
				m.On("List", ctx, mock.MatchedBy(func(req *models.ConfigListRequest) bool {
					return req.PageSize == 10
				})).Return(response, nil)
			},
			want: &models.ConfigListResponse{
				Total: 0,
				Page:  1,
				Size:  10,
			},
			wantErr: false,
		},
		{
			name: "repository error",
			req: &models.ConfigListRequest{
				Page:     1,
				PageSize: 10,
			},
			setup: func(m *mocks.MockConfigRepository) {
				m.On("List", ctx, mock.Anything).Return(nil, assert.AnError)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mocks.MockConfigRepository)
			tt.setup(mockRepo)

			service := NewConfigService(mockRepo, logger)
			got, err := service.ListConfigs(ctx, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.Total, got.Total)
				assert.Equal(t, tt.want.Page, got.Page)
				assert.Equal(t, tt.want.Size, got.Size)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestConfigService_RollbackConfig(t *testing.T) {
	logger := logrus.New()
	ctx := context.Background()

	tests := []struct {
		name     string
		configID string
		version  int
		userID   string
		setup    func(*mocks.MockConfigRepository)
		want     *models.Config
		wantErr  bool
	}{
		{
			name:     "successful rollback",
			configID: "config-123",
			version:  1,
			userID:   "user2",
			setup: func(m *mocks.MockConfigRepository) {
				// Mock GetHistory
				history := []*models.ConfigHistory{
					{
						ConfigID:   "config-123",
						Version:    2,
						Content:    "filter { new }",
						ChangeType: "update",
					},
					{
						ConfigID:   "config-123",
						Version:    1,
						Content:    "filter { old }",
						ChangeType: "create",
					},
				}
				m.On("GetHistory", ctx, "config-123").Return(history, nil)

				// Mock GetByID
				currentConfig := &models.Config{
					ID:      "config-123",
					Name:    "test-config",
					Type:    models.ConfigTypeFilter,
					Content: "filter { current }",
					Version: 2,
				}
				m.On("GetByID", ctx, "config-123").Return(currentConfig, nil)

				// Mock Update
				m.On("Update", ctx, mock.MatchedBy(func(cfg *models.Config) bool {
					return cfg.Content == "filter { old }" && cfg.UpdatedBy == "user2"
				})).Return(nil)
			},
			want: &models.Config{
				Content: "filter { old }",
			},
			wantErr: false,
		},
		{
			name:     "version not found",
			configID: "config-123",
			version:  5,
			userID:   "user2",
			setup: func(m *mocks.MockConfigRepository) {
				history := []*models.ConfigHistory{
					{
						ConfigID: "config-123",
						Version:  2,
					},
					{
						ConfigID: "config-123",
						Version:  1,
					},
				}
				m.On("GetHistory", ctx, "config-123").Return(history, nil)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:     "history retrieval error",
			configID: "config-123",
			version:  1,
			userID:   "user2",
			setup: func(m *mocks.MockConfigRepository) {
				m.On("GetHistory", ctx, "config-123").Return(nil, assert.AnError)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mocks.MockConfigRepository)
			tt.setup(mockRepo)

			service := NewConfigService(mockRepo, logger)
			got, err := service.RollbackConfig(ctx, tt.configID, tt.version, tt.userID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				if tt.want != nil {
					assert.Equal(t, tt.want.Content, got.Content)
				}
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestValidateConfigContent(t *testing.T) {
	tests := []struct {
		name        string
		configType  models.ConfigType
		content     string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid filter config",
			configType:  models.ConfigTypeFilter,
			content:     "filter { mutate { add_field => { \"test\" => \"value\" } } }",
			wantErr:     false,
		},
		{
			name:        "empty content",
			configType:  models.ConfigTypeFilter,
			content:     "",
			wantErr:     true,
			errContains: "配置内容不能为空",
		},
		{
			name:        "valid input config",
			configType:  models.ConfigTypeInput,
			content:     "input { stdin {} }",
			wantErr:     false,
		},
		{
			name:        "valid output config",
			configType:  models.ConfigTypeOutput,
			content:     "output { stdout {} }",
			wantErr:     false,
		},
		{
			name:        "missing input keyword",
			configType:  models.ConfigTypeInput,
			content:     "stdin {}",
			wantErr:     true,
			errContains: "输入配置必须包含 'input' 关键字",
		},
		{
			name:        "missing filter keyword",
			configType:  models.ConfigTypeFilter,
			content:     "mutate {}",
			wantErr:     true,
			errContains: "过滤配置必须包含 'filter' 关键字",
		},
		{
			name:        "missing output keyword",
			configType:  models.ConfigTypeOutput,
			content:     "stdout {}",
			wantErr:     true,
			errContains: "输出配置必须包含 'output' 关键字",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logrus.New()
			service := &configService{logger: logger}
			
			err := service.validateConfigContent(tt.configType, tt.content)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}