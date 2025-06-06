package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"logstash-platform/internal/platform/models"
	"logstash-platform/tests/mocks"
)

func TestConfigRepository_Create(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	tests := []struct {
		name    string
		config  *models.Config
		setup   func(*mocks.MockElasticsearchClient)
		wantErr bool
		check   func(*testing.T, *models.Config)
	}{
		{
			name: "successful creation",
			config: &models.Config{
				Name:        "test-config",
				Type:        models.ConfigTypeFilter,
				Content:     "filter { }",
				Tags:        []string{"test"},
				Description: "Test config",
				CreatedBy:   "user1",
				UpdatedBy:   "user1",
			},
			setup: func(m *mocks.MockElasticsearchClient) {
				// Expect index call for config
				m.On("Index", ctx, "logstash_configs", mock.AnythingOfType("string"), mock.AnythingOfType("*models.Config")).
					Return(nil).
					Run(func(args mock.Arguments) {
						// Verify the config passed
						config := args.Get(3).(*models.Config)
						assert.NotEmpty(t, config.ID)
						assert.Equal(t, 1, config.Version)
						assert.True(t, config.Enabled)
						assert.Equal(t, models.TestStatusUntested, config.TestStatus)
					})

				// Expect index call for history
				m.On("Index", ctx, "logstash_config_history", mock.AnythingOfType("string"), mock.AnythingOfType("*models.ConfigHistory")).
					Return(nil)
			},
			wantErr: false,
			check: func(t *testing.T, config *models.Config) {
				assert.NotEmpty(t, config.ID)
				assert.Equal(t, 1, config.Version)
				assert.True(t, config.Enabled)
				assert.Equal(t, models.TestStatusUntested, config.TestStatus)
				assert.NotZero(t, config.CreatedAt)
				assert.NotZero(t, config.UpdatedAt)
			},
		},
		{
			name: "index failure",
			config: &models.Config{
				Name:      "test-config",
				Type:      models.ConfigTypeFilter,
				Content:   "filter { }",
				CreatedBy: "user1",
			},
			setup: func(m *mocks.MockElasticsearchClient) {
				m.On("Index", ctx, "logstash_configs", mock.AnythingOfType("string"), mock.AnythingOfType("*models.Config")).
					Return(errors.New("ES connection failed"))
			},
			wantErr: true,
		},
		{
			name: "with existing ID",
			config: &models.Config{
				ID:         "custom-id",
				Name:       "test-config",
				Type:       models.ConfigTypeFilter,
				Content:    "filter { }",
				CreatedBy:  "user1",
			},
			setup: func(m *mocks.MockElasticsearchClient) {
				// Should use the provided ID
				m.On("Index", ctx, "logstash_configs", "custom-id", mock.AnythingOfType("*models.Config")).
					Return(nil)
				m.On("Index", ctx, "logstash_config_history", mock.AnythingOfType("string"), mock.AnythingOfType("*models.ConfigHistory")).
					Return(nil)
			},
			wantErr: false,
			check: func(t *testing.T, config *models.Config) {
				assert.Equal(t, "custom-id", config.ID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockES := new(mocks.MockElasticsearchClient)
			tt.setup(mockES)

			// Create repository with mock
			repo := NewConfigRepository(mockES, logger)
			
			// Execute
			err := repo.Create(ctx, tt.config)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.check != nil {
					tt.check(t, tt.config)
				}
			}

			mockES.AssertExpectations(t)
		})
	}
}

func TestConfigRepository_Update(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()

	tests := []struct {
		name    string
		config  *models.Config
		setup   func(*mocks.MockElasticsearchClient)
		wantErr bool
		check   func(*testing.T, *models.Config)
	}{
		{
			name: "successful update",
			config: &models.Config{
				ID:         "test-id",
				Name:       "updated-config",
				Type:       models.ConfigTypeFilter,
				Content:    "filter { updated }",
				Tags:       []string{"test", "updated"},
				UpdatedBy:  "user2",
			},
			setup: func(m *mocks.MockElasticsearchClient) {
				existingConfig := &models.Config{
					ID:        "test-id",
					Name:      "original-config",
					Type:      models.ConfigTypeFilter,
					Content:   "filter { original }",
					Version:   1,
					CreatedAt: time.Now().Add(-24 * time.Hour),
					CreatedBy: "user1",
					TestStatus: models.TestStatusPassed,
				}
				
				// Mock GetByID
				m.On("Get", ctx, "logstash_configs", "test-id", mock.AnythingOfType("*models.Config")).
					Return(nil).
					Run(func(args mock.Arguments) {
						result := args.Get(3).(*models.Config)
						*result = *existingConfig
					})
				
				// Mock Update
				m.On("Index", ctx, "logstash_configs", "test-id", mock.AnythingOfType("*models.Config")).
					Return(nil).
					Run(func(args mock.Arguments) {
						config := args.Get(3).(*models.Config)
						assert.Equal(t, 2, config.Version)
						assert.Equal(t, models.TestStatusUntested, config.TestStatus) // Content changed
					})
				
				// Mock SaveHistory
				m.On("Index", ctx, "logstash_config_history", mock.AnythingOfType("string"), mock.AnythingOfType("*models.ConfigHistory")).
					Return(nil)
			},
			wantErr: false,
			check: func(t *testing.T, config *models.Config) {
				assert.Equal(t, 2, config.Version)
				assert.Equal(t, models.TestStatusUntested, config.TestStatus)
				assert.NotZero(t, config.UpdatedAt)
			},
		},
		{
			name: "config not found",
			config: &models.Config{
				ID:   "non-existent",
				Name: "test",
			},
			setup: func(m *mocks.MockElasticsearchClient) {
				m.On("Get", ctx, "logstash_configs", "non-existent", mock.AnythingOfType("*models.Config")).
					Return(errors.New("not found"))
			},
			wantErr: true,
		},
		{
			name: "update without content change",
			config: &models.Config{
				ID:         "test-id",
				Name:       "updated-config",
				Type:       models.ConfigTypeFilter,
				Content:    "filter { original }", // Same content
				UpdatedBy:  "user2",
			},
			setup: func(m *mocks.MockElasticsearchClient) {
				existingConfig := &models.Config{
					ID:        "test-id",
					Content:   "filter { original }",
					Version:   1,
					TestStatus: models.TestStatusPassed,
					CreatedAt: time.Now().Add(-24 * time.Hour),
					CreatedBy: "user1",
				}
				
				m.On("Get", ctx, "logstash_configs", "test-id", mock.AnythingOfType("*models.Config")).
					Return(nil).
					Run(func(args mock.Arguments) {
						result := args.Get(3).(*models.Config)
						*result = *existingConfig
					})
				
				m.On("Index", ctx, "logstash_configs", "test-id", mock.AnythingOfType("*models.Config")).
					Return(nil).
					Run(func(args mock.Arguments) {
						config := args.Get(3).(*models.Config)
						// TestStatus should not change when content is same
						assert.Equal(t, models.TestStatusPassed, config.TestStatus)
					})
				
				m.On("Index", ctx, "logstash_config_history", mock.AnythingOfType("string"), mock.AnythingOfType("*models.ConfigHistory")).
					Return(nil)
			},
			wantErr: false,
			check: func(t *testing.T, config *models.Config) {
				assert.Equal(t, models.TestStatusPassed, config.TestStatus)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockES := new(mocks.MockElasticsearchClient)
			tt.setup(mockES)

			repo := NewConfigRepository(mockES, logger)
			err := repo.Update(ctx, tt.config)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.check != nil {
					tt.check(t, tt.config)
				}
			}

			mockES.AssertExpectations(t)
		})
	}
}

func TestConfigRepository_GetByID(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()

	tests := []struct {
		name    string
		id      string
		setup   func(*mocks.MockElasticsearchClient)
		wantErr bool
		check   func(*testing.T, *models.Config)
	}{
		{
			name: "successful get",
			id:   "test-id",
			setup: func(m *mocks.MockElasticsearchClient) {
				expected := &models.Config{
					ID:          "test-id",
					Name:        "test-config",
					Type:        models.ConfigTypeFilter,
					Content:     "filter { }",
					Version:     1,
					Enabled:     true,
					Tags:        []string{"test"},
					Description: "Test config",
					TestStatus:  models.TestStatusPassed,
					CreatedBy:   "user1",
					UpdatedBy:   "user1",
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}
				
				m.On("Get", ctx, "logstash_configs", "test-id", mock.AnythingOfType("*models.Config")).
					Return(nil).
					Run(func(args mock.Arguments) {
						result := args.Get(3).(*models.Config)
						*result = *expected
					})
			},
			wantErr: false,
			check: func(t *testing.T, config *models.Config) {
				assert.Equal(t, "test-id", config.ID)
				assert.Equal(t, "test-config", config.Name)
				assert.Equal(t, models.ConfigTypeFilter, config.Type)
				assert.Equal(t, "filter { }", config.Content)
				assert.Equal(t, 1, config.Version)
				assert.True(t, config.Enabled)
				assert.Equal(t, models.TestStatusPassed, config.TestStatus)
			},
		},
		{
			name: "config not found",
			id:   "non-existent",
			setup: func(m *mocks.MockElasticsearchClient) {
				m.On("Get", ctx, "logstash_configs", "non-existent", mock.AnythingOfType("*models.Config")).
					Return(errors.New("document not found"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockES := new(mocks.MockElasticsearchClient)
			tt.setup(mockES)

			repo := NewConfigRepository(mockES, logger)
			config, err := repo.GetByID(ctx, tt.id)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
				if tt.check != nil {
					tt.check(t, config)
				}
			}

			mockES.AssertExpectations(t)
		})
	}
}

func TestConfigRepository_Delete(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()

	tests := []struct {
		name    string
		id      string
		setup   func(*mocks.MockElasticsearchClient)
		wantErr bool
	}{
		{
			name: "successful delete",
			id:   "test-id",
			setup: func(m *mocks.MockElasticsearchClient) {
				config := &models.Config{
					ID:        "test-id",
					Name:      "test-config",
					Version:   1,
					Content:   "filter { }",
					UpdatedBy: "user1",
				}
				
				// Mock GetByID
				m.On("Get", ctx, "logstash_configs", "test-id", mock.AnythingOfType("*models.Config")).
					Return(nil).
					Run(func(args mock.Arguments) {
						result := args.Get(3).(*models.Config)
						*result = *config
					})
				
				// Mock Delete
				m.On("Delete", ctx, "logstash_configs", "test-id").Return(nil)
				
				// Mock SaveHistory
				m.On("Index", ctx, "logstash_config_history", mock.AnythingOfType("string"), mock.AnythingOfType("*models.ConfigHistory")).
					Return(nil).
					Run(func(args mock.Arguments) {
						history := args.Get(3).(*models.ConfigHistory)
						assert.Equal(t, "test-id", history.ConfigID)
						assert.Equal(t, "delete", history.ChangeType)
						assert.Contains(t, history.ChangeLog, "删除配置")
					})
			},
			wantErr: false,
		},
		{
			name: "config not found",
			id:   "non-existent",
			setup: func(m *mocks.MockElasticsearchClient) {
				m.On("Get", ctx, "logstash_configs", "non-existent", mock.AnythingOfType("*models.Config")).
					Return(errors.New("not found"))
			},
			wantErr: true,
		},
		{
			name: "delete failure",
			id:   "test-id",
			setup: func(m *mocks.MockElasticsearchClient) {
				config := &models.Config{
					ID:   "test-id",
					Name: "test-config",
				}
				
				m.On("Get", ctx, "logstash_configs", "test-id", mock.AnythingOfType("*models.Config")).
					Return(nil).
					Run(func(args mock.Arguments) {
						result := args.Get(3).(*models.Config)
						*result = *config
					})
				
				m.On("Delete", ctx, "logstash_configs", "test-id").
					Return(errors.New("ES connection failed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockES := new(mocks.MockElasticsearchClient)
			tt.setup(mockES)

			repo := NewConfigRepository(mockES, logger)
			err := repo.Delete(ctx, tt.id)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockES.AssertExpectations(t)
		})
	}
}

func TestConfigRepository_List(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()

	tests := []struct {
		name    string
		req     *models.ConfigListRequest
		setup   func(*mocks.MockElasticsearchClient)
		wantErr bool
		check   func(*testing.T, *models.ConfigListResponse)
	}{
		{
			name: "successful list with filters",
			req: &models.ConfigListRequest{
				Page:     1,
				PageSize: 10,
				Type:     models.ConfigTypeFilter,
				Enabled:  &[]bool{true}[0],
				Tags:     []string{"production"},
			},
			setup: func(m *mocks.MockElasticsearchClient) {
				
				result := struct {
					Hits struct {
						Total struct {
							Value int64 `json:"value"`
						} `json:"total"`
						Hits []struct {
							Source models.Config `json:"_source"`
						} `json:"hits"`
					} `json:"hits"`
				}{
					Hits: struct {
						Total struct {
							Value int64 `json:"value"`
						} `json:"total"`
						Hits []struct {
							Source models.Config `json:"_source"`
						} `json:"hits"`
					}{
						Total: struct {
							Value int64 `json:"value"`
						}{Value: 2},
						Hits: []struct {
							Source models.Config `json:"_source"`
						}{
							{
								Source: models.Config{
									ID:      "config-1",
									Name:    "Filter 1",
									Type:    models.ConfigTypeFilter,
									Enabled: true,
									Tags:    []string{"production"},
								},
							},
							{
								Source: models.Config{
									ID:      "config-2",
									Name:    "Filter 2",
									Type:    models.ConfigTypeFilter,
									Enabled: true,
									Tags:    []string{"production"},
								},
							},
						},
					},
				}
				
				m.On("Search", ctx, "logstash_configs", mock.Anything, mock.Anything).
					Return(nil).
					Run(func(args mock.Arguments) {
						dest := args.Get(3).(*struct {
							Hits struct {
								Total struct {
									Value int64 `json:"value"`
								} `json:"total"`
								Hits []struct {
									Source models.Config `json:"_source"`
								} `json:"hits"`
							} `json:"hits"`
						})
						*dest = result
					})
			},
			wantErr: false,
			check: func(t *testing.T, resp *models.ConfigListResponse) {
				assert.Equal(t, int64(2), resp.Total)
				assert.Equal(t, 1, resp.Page)
				assert.Equal(t, 10, resp.Size)
				assert.Len(t, resp.Items, 2)
				assert.Equal(t, "config-1", resp.Items[0].ID)
				assert.Equal(t, "config-2", resp.Items[1].ID)
			},
		},
		{
			name: "empty list",
			req: &models.ConfigListRequest{
				Page:     1,
				PageSize: 10,
			},
			setup: func(m *mocks.MockElasticsearchClient) {
				result := struct {
					Hits struct {
						Total struct {
							Value int64 `json:"value"`
						} `json:"total"`
						Hits []struct {
							Source models.Config `json:"_source"`
						} `json:"hits"`
					} `json:"hits"`
				}{}
				
				m.On("Search", ctx, "logstash_configs", mock.Anything, mock.Anything).
					Return(nil).
					Run(func(args mock.Arguments) {
						dest := args.Get(3).(*struct {
							Hits struct {
								Total struct {
									Value int64 `json:"value"`
								} `json:"total"`
								Hits []struct {
									Source models.Config `json:"_source"`
								} `json:"hits"`
							} `json:"hits"`
						})
						*dest = result
					})
			},
			wantErr: false,
			check: func(t *testing.T, resp *models.ConfigListResponse) {
				assert.Equal(t, int64(0), resp.Total)
				assert.Len(t, resp.Items, 0)
			},
		},
		{
			name: "search failure",
			req: &models.ConfigListRequest{
				Page:     1,
				PageSize: 10,
			},
			setup: func(m *mocks.MockElasticsearchClient) {
				m.On("Search", ctx, "logstash_configs", mock.Anything, mock.Anything).
					Return(errors.New("ES connection failed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockES := new(mocks.MockElasticsearchClient)
			tt.setup(mockES)

			repo := NewConfigRepository(mockES, logger)
			resp, err := repo.List(ctx, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				if tt.check != nil {
					tt.check(t, resp)
				}
			}

			mockES.AssertExpectations(t)
		})
	}
}

func TestConfigRepository_SaveHistory(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()

	tests := []struct {
		name    string
		history *models.ConfigHistory
		setup   func(*mocks.MockElasticsearchClient)
		wantErr bool
	}{
		{
			name: "successful save with ID",
			history: &models.ConfigHistory{
				ID:         "history-1",
				ConfigID:   "config-1",
				Version:    1,
				Content:    "filter { }",
				ChangeType: "create",
				ChangeLog:  "Created config",
				ModifiedBy: "user1",
				ModifiedAt: time.Now(),
			},
			setup: func(m *mocks.MockElasticsearchClient) {
				m.On("Index", ctx, "logstash_config_history", "history-1", mock.AnythingOfType("*models.ConfigHistory")).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "successful save without ID",
			history: &models.ConfigHistory{
				ConfigID:   "config-1",
				Version:    1,
				Content:    "filter { }",
				ChangeType: "create",
				ModifiedBy: "user1",
				ModifiedAt: time.Now(),
			},
			setup: func(m *mocks.MockElasticsearchClient) {
				m.On("Index", ctx, "logstash_config_history", mock.AnythingOfType("string"), mock.AnythingOfType("*models.ConfigHistory")).
					Return(nil).
					Run(func(args mock.Arguments) {
						history := args.Get(3).(*models.ConfigHistory)
						assert.NotEmpty(t, history.ID)
					})
			},
			wantErr: false,
		},
		{
			name: "index failure",
			history: &models.ConfigHistory{
				ConfigID: "config-1",
				Version:  1,
			},
			setup: func(m *mocks.MockElasticsearchClient) {
				m.On("Index", ctx, "logstash_config_history", mock.AnythingOfType("string"), mock.AnythingOfType("*models.ConfigHistory")).
					Return(errors.New("ES connection failed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockES := new(mocks.MockElasticsearchClient)
			tt.setup(mockES)

			repo := NewConfigRepository(mockES, logger)
			err := repo.SaveHistory(ctx, tt.history)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockES.AssertExpectations(t)
		})
	}
}

func TestConfigRepository_GetHistory(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()

	tests := []struct {
		name     string
		configID string
		setup    func(*mocks.MockElasticsearchClient)
		wantErr  bool
		check    func(*testing.T, []*models.ConfigHistory)
	}{
		{
			name:     "successful get history",
			configID: "config-1",
			setup: func(m *mocks.MockElasticsearchClient) {
				
				result := struct {
					Hits struct {
						Hits []struct {
							Source models.ConfigHistory `json:"_source"`
						} `json:"hits"`
					} `json:"hits"`
				}{
					Hits: struct {
						Hits []struct {
							Source models.ConfigHistory `json:"_source"`
						} `json:"hits"`
					}{
						Hits: []struct {
							Source models.ConfigHistory `json:"_source"`
						}{
							{
								Source: models.ConfigHistory{
									ID:         "history-1",
									ConfigID:   "config-1",
									Version:    2,
									ChangeType: "update",
									ModifiedAt: time.Now(),
								},
							},
							{
								Source: models.ConfigHistory{
									ID:         "history-2",
									ConfigID:   "config-1",
									Version:    1,
									ChangeType: "create",
									ModifiedAt: time.Now().Add(-1 * time.Hour),
								},
							},
						},
					},
				}
				
				m.On("Search", ctx, "logstash_config_history", mock.Anything, mock.Anything).
					Return(nil).
					Run(func(args mock.Arguments) {
						dest := args.Get(3).(*struct {
							Hits struct {
								Hits []struct {
									Source models.ConfigHistory `json:"_source"`
								} `json:"hits"`
							} `json:"hits"`
						})
						*dest = result
					})
			},
			wantErr: false,
			check: func(t *testing.T, history []*models.ConfigHistory) {
				assert.Len(t, history, 2)
				assert.Equal(t, "history-1", history[0].ID)
				assert.Equal(t, 2, history[0].Version)
				assert.Equal(t, "update", history[0].ChangeType)
				assert.Equal(t, "history-2", history[1].ID)
				assert.Equal(t, 1, history[1].Version)
				assert.Equal(t, "create", history[1].ChangeType)
			},
		},
		{
			name:     "empty history",
			configID: "config-2",
			setup: func(m *mocks.MockElasticsearchClient) {
				result := struct {
					Hits struct {
						Hits []struct {
							Source models.ConfigHistory `json:"_source"`
						} `json:"hits"`
					} `json:"hits"`
				}{}
				
				m.On("Search", ctx, "logstash_config_history", mock.Anything, mock.Anything).
					Return(nil).
					Run(func(args mock.Arguments) {
						dest := args.Get(3).(*struct {
							Hits struct {
								Hits []struct {
									Source models.ConfigHistory `json:"_source"`
								} `json:"hits"`
							} `json:"hits"`
						})
						*dest = result
					})
			},
			wantErr: false,
			check: func(t *testing.T, history []*models.ConfigHistory) {
				assert.Len(t, history, 0)
			},
		},
		{
			name:     "search failure",
			configID: "config-1",
			setup: func(m *mocks.MockElasticsearchClient) {
				m.On("Search", ctx, "logstash_config_history", mock.Anything, mock.Anything).
					Return(errors.New("ES connection failed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockES := new(mocks.MockElasticsearchClient)
			tt.setup(mockES)

			repo := NewConfigRepository(mockES, logger)
			history, err := repo.GetHistory(ctx, tt.configID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, history)
			} else {
				assert.NoError(t, err)
				if tt.check != nil {
					tt.check(t, history)
				}
			}

			mockES.AssertExpectations(t)
		})
	}
}