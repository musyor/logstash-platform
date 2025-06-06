package mocks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestMockElasticsearchClient 测试 mock 实现是否正常工作
func TestMockElasticsearchClient(t *testing.T) {
	ctx := context.Background()
	mockClient := new(MockElasticsearchClient)

	// 测试 InitializeIndices
	t.Run("InitializeIndices", func(t *testing.T) {
		mockClient.On("InitializeIndices", ctx).Return(nil).Once()
		
		err := mockClient.InitializeIndices(ctx)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	// 测试 IndexExists
	t.Run("IndexExists", func(t *testing.T) {
		mockClient.On("IndexExists", ctx, "test-index").Return(true, nil).Once()
		
		exists, err := mockClient.IndexExists(ctx, "test-index")
		assert.NoError(t, err)
		assert.True(t, exists)
		mockClient.AssertExpectations(t)
	})

	// 测试 Index
	t.Run("Index", func(t *testing.T) {
		doc := map[string]string{"field": "value"}
		mockClient.On("Index", ctx, "test-index", "doc-1", doc).Return(nil).Once()
		
		err := mockClient.Index(ctx, "test-index", "doc-1", doc)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	// 测试 Get
	t.Run("Get", func(t *testing.T) {
		var result map[string]string
		mockClient.On("Get", ctx, "test-index", "doc-1", mock.Anything).Return(nil, nil).Once()
		
		err := mockClient.Get(ctx, "test-index", "doc-1", &result)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	// 测试 Search
	t.Run("Search", func(t *testing.T) {
		query := map[string]interface{}{"match_all": map[string]interface{}{}}
		var result interface{}
		mockClient.On("Search", ctx, "test-index", query, mock.Anything).Return(nil, nil).Once()
		
		err := mockClient.Search(ctx, "test-index", query, &result)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	// 测试 Delete
	t.Run("Delete", func(t *testing.T) {
		mockClient.On("Delete", ctx, "test-index", "doc-1").Return(nil).Once()
		
		err := mockClient.Delete(ctx, "test-index", "doc-1")
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	// 测试错误场景
	t.Run("Error scenarios", func(t *testing.T) {
		mockClient.On("CreateIndex", ctx, "error-index", "").Return(assert.AnError).Once()
		
		err := mockClient.CreateIndex(ctx, "error-index", "")
		assert.Error(t, err)
		mockClient.AssertExpectations(t)
	})
}