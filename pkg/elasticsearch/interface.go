package elasticsearch

import "context"

// ClientInterface 定义 Elasticsearch 客户端接口
// 这个接口抽象了所有 Elasticsearch 操作，便于测试时使用 mock
type ClientInterface interface {
	// InitializeIndices 初始化所需的索引
	InitializeIndices(ctx context.Context) error

	// IndexExists 检查索引是否存在
	IndexExists(ctx context.Context, index string) (bool, error)

	// CreateIndex 创建索引
	CreateIndex(ctx context.Context, index string, mapping string) error

	// Index 索引文档
	Index(ctx context.Context, index, id string, doc interface{}) error

	// Get 获取文档
	Get(ctx context.Context, index, id string, result interface{}) error

	// Search 搜索文档
	Search(ctx context.Context, index string, query map[string]interface{}, result interface{}) error

	// Delete 删除文档
	Delete(ctx context.Context, index, id string) error
}