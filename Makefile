# Makefile for Logstash Platform

# 变量定义
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME_PLATFORM=logstash-platform
BINARY_NAME_AGENT=logstash-agent
PLATFORM_PATH=./cmd/platform
AGENT_PATH=./cmd/agent

# 版本信息
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date +%Y%m%d-%H%M%S)
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# 默认目标
.PHONY: all
all: build

# 构建平台
.PHONY: build-platform
build-platform:
	@echo "构建管理平台..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME_PLATFORM) -v $(PLATFORM_PATH)

# 构建Agent
.PHONY: build-agent
build-agent:
	@echo "构建Agent..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME_AGENT) -v $(AGENT_PATH)

# 构建所有
.PHONY: build
build: build-platform

# 运行平台
.PHONY: run-platform
run-platform:
	@echo "启动管理平台..."
	$(GOBUILD) -o $(BINARY_NAME_PLATFORM) -v $(PLATFORM_PATH)
	./$(BINARY_NAME_PLATFORM)

# 运行平台（开发模式）
.PHONY: dev
dev:
	@echo "启动管理平台（开发模式）..."
	$(GOCMD) run $(PLATFORM_PATH)/main.go

# 测试
.PHONY: test
test:
	@echo "运行测试..."
	$(GOTEST) -v ./...

# 测试覆盖率
.PHONY: test-coverage
test-coverage:
	@echo "运行测试并生成覆盖率报告..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# 清理
.PHONY: clean
clean:
	@echo "清理构建文件..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME_PLATFORM)
	rm -f $(BINARY_NAME_AGENT)
	rm -f coverage.out coverage.html

# 依赖管理
.PHONY: deps
deps:
	@echo "下载依赖..."
	$(GOMOD) download

# 更新依赖
.PHONY: deps-update
deps-update:
	@echo "更新依赖..."
	$(GOMOD) tidy

# 代码检查
.PHONY: lint
lint:
	@echo "运行代码检查..."
	golangci-lint run

# 格式化代码
.PHONY: fmt
fmt:
	@echo "格式化代码..."
	$(GOCMD) fmt ./...

# 初始化ES索引
.PHONY: init-es
init-es:
	@echo "初始化Elasticsearch索引..."
	chmod +x ./scripts/init_es_indices.sh
	./scripts/init_es_indices.sh

# Docker构建
.PHONY: docker-build
docker-build:
	@echo "构建Docker镜像..."
	docker build -t logstash-platform:$(VERSION) -f Dockerfile.platform .
	docker build -t logstash-agent:$(VERSION) -f Dockerfile.agent .

# 帮助
.PHONY: help
help:
	@echo "Logstash Platform Makefile 使用说明:"
	@echo ""
	@echo "  make build          - 构建平台二进制文件"
	@echo "  make build-agent    - 构建Agent二进制文件" 
	@echo "  make run-platform   - 构建并运行平台"
	@echo "  make dev            - 开发模式运行平台"
	@echo "  make test           - 运行测试"
	@echo "  make test-coverage  - 运行测试并生成覆盖率报告"
	@echo "  make clean          - 清理构建文件"
	@echo "  make deps           - 下载依赖"
	@echo "  make deps-update    - 更新依赖"
	@echo "  make lint           - 运行代码检查"
	@echo "  make fmt            - 格式化代码"
	@echo "  make init-es        - 初始化ES索引"
	@echo "  make docker-build   - 构建Docker镜像"
	@echo "  make help           - 显示此帮助信息"