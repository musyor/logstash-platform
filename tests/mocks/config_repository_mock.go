package mocks

import (
	"context"
	"logstash-platform/internal/platform/models"
	"github.com/stretchr/testify/mock"
)

// MockConfigRepository is a mock implementation of ConfigRepository
type MockConfigRepository struct {
	mock.Mock
}

// Create mocks the Create method
func (m *MockConfigRepository) Create(ctx context.Context, config *models.Config) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

// Update mocks the Update method
func (m *MockConfigRepository) Update(ctx context.Context, config *models.Config) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

// Delete mocks the Delete method
func (m *MockConfigRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// GetByID mocks the GetByID method
func (m *MockConfigRepository) GetByID(ctx context.Context, id string) (*models.Config, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Config), args.Error(1)
}

// List mocks the List method
func (m *MockConfigRepository) List(ctx context.Context, req *models.ConfigListRequest) (*models.ConfigListResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ConfigListResponse), args.Error(1)
}

// SaveHistory mocks the SaveHistory method
func (m *MockConfigRepository) SaveHistory(ctx context.Context, history *models.ConfigHistory) error {
	args := m.Called(ctx, history)
	return args.Error(0)
}

// GetHistory mocks the GetHistory method
func (m *MockConfigRepository) GetHistory(ctx context.Context, configID string) ([]*models.ConfigHistory, error) {
	args := m.Called(ctx, configID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.ConfigHistory), args.Error(1)
}