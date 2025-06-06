package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockElasticsearchClient 是 Elasticsearch 客户端的 mock 实现
type MockElasticsearchClient struct {
	mock.Mock
}

// InitializeIndices 初始化所需的索引
func (m *MockElasticsearchClient) InitializeIndices(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// IndexExists 检查索引是否存在
func (m *MockElasticsearchClient) IndexExists(ctx context.Context, index string) (bool, error) {
	args := m.Called(ctx, index)
	return args.Bool(0), args.Error(1)
}

// CreateIndex 创建索引
func (m *MockElasticsearchClient) CreateIndex(ctx context.Context, index string, mapping string) error {
	args := m.Called(ctx, index, mapping)
	return args.Error(0)
}

// Index 索引文档
func (m *MockElasticsearchClient) Index(ctx context.Context, index, id string, doc interface{}) error {
	args := m.Called(ctx, index, id, doc)
	return args.Error(0)
}

// Get 获取文档
func (m *MockElasticsearchClient) Get(ctx context.Context, index, id string, result interface{}) error {
	args := m.Called(ctx, index, id, result)
	
	// 如果 mock 设置了返回数据，填充到 result 中
	if args.Get(0) != nil {
		// 这里可以根据测试需要进行类型转换和赋值
		// 例如：*result.(*models.Config) = args.Get(0).(models.Config)
	}
	
	return args.Error(1)
}

// Search 搜索文档
func (m *MockElasticsearchClient) Search(ctx context.Context, index string, query map[string]interface{}, result interface{}) error {
	args := m.Called(ctx, index, query, result)
	
	// 如果 mock 设置了返回数据，填充到 result 中
	if args.Get(0) != nil {
		// 这里可以根据测试需要进行类型转换和赋值
	}
	
	return args.Error(1)
}

// Delete 删除文档
func (m *MockElasticsearchClient) Delete(ctx context.Context, index, id string) error {
	args := m.Called(ctx, index, id)
	return args.Error(0)
}