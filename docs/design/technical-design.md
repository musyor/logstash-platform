# Logstash配置管理平台技术方案（简化版）

## 1. 项目背景

### 1.1 现状与问题
- **手动管理低效**：需要登录每台Logstash服务器手动修改配置文件
- **无法验证正确性**：配置修改后无法提前测试，只能在生产环境验证
- **缺乏版本控制**：配置变更无记录，出问题难以回滚
- **多节点同步困难**：集群环境下配置同步复杂易出错

### 1.2 解决方案
构建一个精简的Logstash配置管理平台，利用现有ELK基础设施，仅增加管理平台和Agent两个组件，实现配置的统一存储、版本管理、自动分发和真实数据测试。

## 2. 系统架构

### 2.1 整体架构图（简化版）
```
┌─────────────────────────────────────────────────────────────────────┐
│                          Web Console (Vue.js)                        │
│                   配置管理 | 测试中心 | Agent监控                     │
└───────────────────────────────┬─────────────────────────────────────┘
                                │ HTTPS
┌───────────────────────────────▼─────────────────────────────────────┐
│                    Management Platform (Go/Gin)                      │
│  ┌─────────────┐ ┌──────────────┐ ┌──────────────┐ ┌─────────────┐ │
│  │  Config API │ │   Test API   │ │  Agent API   │ │ Kafka Client│ │
│  └─────────────┘ └──────────────┘ └──────────────┘ └─────────────┘ │
│                  ┌────────────────────────────────┐                  │
│                  │  内嵌测试Logstash实例         │                  │
│                  └────────────────────────────────┘                  │
└───────┬────────────────────────────────┬────────────────────────────┘
        │                                │ WebSocket
        ▼                                ▼
┌──────────────┐              ┌──────────────────────────────┐
│ Elasticsearch│              │         Agent群组            │
│ (现有ES集群) │              ├──────────────────────────────┤
│              │              │ Agent1 │ Agent2 │ Agent3    │
│ ·配置存储    │              │   ↓       ↓        ↓       │
│ ·版本历史    │              │ Logstash Logstash Logstash │
│ ·Agent信息   │              └──────────────────────────────┘
└──────────────┘
```

### 2.2 核心组件说明

#### 2.2.1 Management Platform（管理平台）
- **技术栈**：Go + Gin框架
- **主要功能**：
  - 配置CRUD操作
  - 版本管理
  - 配置实时推送（WebSocket）
  - 内嵌Logstash测试
  - Agent连接管理
  - 测试数据获取（Kafka/样本）

#### 2.2.2 Agent（节点代理）
- **部署位置**：每个Logstash节点
- **主要功能**：
  - WebSocket长连接
  - 接收配置更新实时推送
  - 本地版本管理（3个版本）
  - 应用配置到Logstash
  - 状态上报和心跳
  - 一键回滚能力

#### 2.2.3 内嵌测试引擎
- **部署方式**：管理平台内嵌轻量级Logstash
- **测试流程**：
  1. 接收测试请求（配置+数据）
  2. 创建临时pipeline
  3. 注入测试数据
  4. 收集处理结果
  5. 对比分析并返回
- **数据源支持**：
  - 用户上传样本
  - Kafka实时数据（限量获取）

## 3. 详细设计

### 3.1 数据模型

#### 3.1.1 配置存储结构（Elasticsearch）
```json
{
  "index": "logstash_configs",
  "mappings": {
    "properties": {
      "id": { "type": "keyword" },
      "name": { "type": "text" },
      "description": { "type": "text" },
      "type": { "type": "keyword" },  // input/filter/output
      "content": { "type": "text" },   // 配置内容
      "tags": { "type": "keyword" },   // 标签数组
      "version": { "type": "integer" },
      "enabled": { "type": "boolean" },
      "test_status": { "type": "keyword" }, // untested/testing/passed/failed
      "created_at": { "type": "date" },
      "updated_at": { "type": "date" },
      "created_by": { "type": "keyword" },
      "updated_by": { "type": "keyword" }
    }
  }
}
```

#### 3.1.2 配置历史记录
```json
{
  "index": "logstash_config_history",
  "mappings": {
    "properties": {
      "config_id": { "type": "keyword" },
      "version": { "type": "integer" },
      "content": { "type": "text" },
      "change_type": { "type": "keyword" }, // create/update/delete
      "change_log": { "type": "text" },
      "modified_by": { "type": "keyword" },
      "modified_at": { "type": "date" }
    }
  }
}
```

#### 3.1.3 Agent注册信息
```json
{
  "index": "logstash_agents",
  "mappings": {
    "properties": {
      "agent_id": { "type": "keyword" },
      "hostname": { "type": "keyword" },
      "ip": { "type": "ip" },
      "logstash_version": { "type": "keyword" },
      "status": { "type": "keyword" }, // online/offline/error
      "last_heartbeat": { "type": "date" },
      "applied_configs": {
        "type": "nested",
        "properties": {
          "config_id": { "type": "keyword" },
          "version": { "type": "integer" },
          "applied_at": { "type": "date" }
        }
      }
    }
  }
}
```

### 3.2 配置管理流程

#### 3.2.1 配置创建/更新流程
```
1. 用户在Web界面创建/编辑配置
2. 平台验证配置格式
3. 保存到Elasticsearch（新版本）
4. 记录变更历史
5. 用户可选择立即测试
6. 测试通过后标记为可部署
```

#### 3.2.2 配置实时推送流程
```
1. 用户点击"部署"按钮
2. 平台通过WebSocket实时推送配置到Agent
3. Agent接收配置内容
4. Agent执行版本轮转：
   - backup = previous
   - previous = current
   - current = 新配置
5. 应用新配置到Logstash
6. 执行 logstash -t 验证
7. 重载或重启Logstash
8. 上报部署结果
9. 失败时可一键回滚到previous版本
```

#### 3.2.3 版本控制机制
```
Agent本地目录结构：
/etc/logstash-agent/
├── configs/
│   ├── current/      # 当前运行版本
│   ├── previous/     # 上一个版本
│   └── backup/       # 备份版本
├── metadata.json     # 版本元数据
└── agent.conf        # Agent配置
```

### 3.3 配置测试设计

#### 3.3.1 内嵌测试架构
```
管理平台内部：
┌─────────────────────────────────────────────────────┐
│                 Test Engine                         │
│  ┌──────────┐    ┌─────────────┐   ┌────────────┐ │
│  │Test Data │───▶│  Embedded   │──▶│   Result   │ │
│  │ Source   │    │  Logstash   │   │  Analyzer  │ │
│  └──────────┘    └─────────────┘   └────────────┘ │
│       ↑                                      ↓      │
│   Kafka/样本                            对比展示    │
└─────────────────────────────────────────────────────┘
```

#### 3.3.2 测试数据获取

**用户上传样本**
```json
{
  "test_type": "sample",
  "samples": [
    "192.168.1.1 - - [20/Dec/2024:10:00:00 +0800] \"GET /api/users HTTP/1.1\" 200 1234",
    "192.168.1.2 - - [20/Dec/2024:10:00:01 +0800] \"POST /api/login HTTP/1.1\" 401 567"
  ]
}
```

**Kafka实时数据**
```json
{
  "test_type": "kafka",
  "kafka_config": {
    "brokers": ["kafka1:9092", "kafka2:9092"],
    "topic": "app-logs",
    "consumer_group": "test-${timestamp}",
    "max_messages": 10,
    "timeout_seconds": 5
  }
}
```

#### 3.3.3 测试执行实现
```go
type TestEngine struct {
    logstashBin string
    tempDir     string
}

func (te *TestEngine) ExecuteTest(config Config, testData TestData) (*TestResult, error) {
    // 1. 准备测试配置
    testPipeline := te.buildTestPipeline(config, testData)
    
    // 2. 写入临时配置文件
    configPath := te.writeTempConfig(testPipeline)
    defer os.RemoveAll(filepath.Dir(configPath))
    
    // 3. 启动内嵌Logstash进程
    cmd := exec.Command(te.logstashBin, "-f", configPath, "--path.data", te.tempDir)
    
    // 4. 注入测试数据并收集结果
    input, output := te.runTestWithData(cmd, testData)
    
    // 5. 分析结果
    return te.analyzeResults(input, output), nil
}

// 构建测试专用pipeline配置
func (te *TestEngine) buildTestPipeline(config Config, testData TestData) string {
    input := te.getTestInput(testData)
    filter := config.Content
    output := "output { stdout { codec => json } }"
    
    return fmt.Sprintf("%s\n%s\n%s", input, filter, output)
}
```

### 3.4 Agent设计

#### 3.4.1 Agent架构
```
┌──────────────────────────────────────────────┐
│                   Agent                      │
│  ┌─────────────┐    ┌───────────────────┐   │
│  │ WS Client   │    │ Version Manager   │   │
│  │ (实时推送)  │    │ (3版本控制)      │   │
│  └──────┬──────┘    └─────────┬─────────┘   │
│         │                     │              │
│  ┌──────▼──────┐    ┌─────────▼─────────┐   │
│  │ Heartbeat   │    │ Config Dirs       │   │
│  │ (30s)       │    │ current/previous/ │   │
│  └─────────────┘    │ backup            │   │
│                     └───────────────────┘   │
│  ┌────────────────────────────────────┐     │
│  │    Logstash Controller             │     │
│  │  · Apply (应用新配置)              │     │
│  │  · Reload (重载服务)               │     │
│  │  · Rollback (版本回滚)             │     │
│  └────────────────────────────────────┘     │
└──────────────────────────────────────────────┘
```

#### 3.4.2 Agent核心功能
1. **实时连接**
   - WebSocket长连接
   - 自动重连（指数退避）
   - 心跳保活（30秒）
   - 推送确认机制

2. **版本管理**
   ```go
   type VersionManager struct {
       CurrentDir   string  // 当前运行版本
       PreviousDir  string  // 上一个版本
       BackupDir    string  // 备份版本
       Metadata     VersionMetadata
   }
   
   func (vm *VersionManager) ApplyNewVersion(config []byte) error {
       // 1. 轮转版本目录
       os.RemoveAll(vm.BackupDir)
       os.Rename(vm.PreviousDir, vm.BackupDir)
       os.Rename(vm.CurrentDir, vm.PreviousDir)
       
       // 2. 写入新配置
       os.MkdirAll(vm.CurrentDir, 0755)
       // 写入配置文件...
       
       // 3. 更新元数据
       vm.updateMetadata()
   }
   ```

3. **Logstash控制**
   - 配置验证：`logstash -t -f current/logstash.conf`
   - 优雅重载：SIGHUP信号或API调用
   - 快速回滚：切换到previous目录并重载

## 4. API设计

### 4.1 配置管理API

```yaml
# 配置列表
GET /api/v1/configs
Query参数:
  - type: input/filter/output
  - tags: 标签过滤
  - enabled: true/false
  - page: 页码
  - size: 每页数量

# 创建配置
POST /api/v1/configs
Body:
{
  "name": "nginx-access-log-parser",
  "type": "filter",
  "content": "filter { grok { ... } }",
  "tags": ["nginx", "access-log"],
  "description": "Parse nginx access logs"
}

# 更新配置
PUT /api/v1/configs/{id}
Body: 同创建

# 删除配置
DELETE /api/v1/configs/{id}

# 获取配置历史
GET /api/v1/configs/{id}/history

# 回滚配置
POST /api/v1/configs/{id}/rollback
Body: { "version": 2 }
```

### 4.2 测试API

```yaml
# 创建测试任务
POST /api/v1/test
Body:
{
  "config_id": "xxx",
  "test_data": {
    "type": "kafka",
    "kafka_config": { ... }
  }
}

# 获取测试结果
GET /api/v1/test/{test_id}/result

# 测试历史
GET /api/v1/configs/{id}/tests
```

### 4.3 Agent管理API

```yaml
# Agent列表
GET /api/v1/agents

# Agent详情
GET /api/v1/agents/{id}

# 配置分发
POST /api/v1/agents/{id}/deploy
Body:
{
  "config_id": "xxx",
  "version": 3
}

# 批量分发
POST /api/v1/deploy
Body:
{
  "config_id": "xxx",
  "agent_ids": ["agent1", "agent2"]
}
```

## 5. 安全设计

### 5.1 认证授权
- **平台认证**：JWT Token
- **Agent认证**：预共享密钥 + TLS
- **权限控制**：RBAC模型

### 5.2 通信安全
- **API通信**：HTTPS
- **Agent通信**：WSS(WebSocket Secure)
- **配置加密**：敏感配置AES加密存储

### 5.3 审计日志
记录所有配置变更和操作：
- WHO：操作用户
- WHEN：操作时间
- WHAT：操作内容
- WHERE：操作来源

## 6. 部署架构

### 6.1 简化部署架构
```
                    Nginx (可选)
                         │
                         ▼
              Management Platform
              (内嵌测试Logstash)
                         │
        ┌────────────────┼────────────────┐
        │                                 │
        ▼                                 ▼
  现有Elasticsearch                   Agent群组
    (配置存储)                     (各Logstash节点)
```

### 6.2 容器化部署
```yaml
# docker-compose.yml
version: '3.8'
services:
  platform:
    image: logstash-manager:latest
    ports:
      - "8080:8080"
    environment:
      - ES_HOSTS=elasticsearch:9200  # 使用现有ES
      - KAFKA_BROKERS=kafka:9092      # 使用现有Kafka
      - LOGSTASH_BIN=/usr/share/logstash/bin/logstash
    volumes:
      - ./logstash:/usr/share/logstash  # 内嵌Logstash
      
  # Agent通常直接部署在Logstash节点上
  # 不需要容器化部署
```

## 7. 实施计划

### 7.1 第一阶段（MVP）- 4周
- 基础配置管理（CRUD）
- ES存储实现
- 简单的Web界面
- 单节点Agent

### 7.2 第二阶段（测试功能）- 3周
- 测试引擎开发
- Kafka数据源集成
- 测试结果展示

### 7.3 第三阶段（生产就绪）- 3周
- Agent集群管理
- 配置分发优化
- 监控告警
- 高可用部署

## 8. 监控指标

### 8.1 系统指标
- Agent在线率
- 配置同步成功率
- 测试执行时间
- API响应时间

### 8.2 业务指标
- 配置变更频率
- 测试通过率
- 回滚次数
- 活跃用户数

## 9. 风险与对策

| 风险 | 影响 | 对策 |
|------|------|------|
| Agent断连 | 配置无法更新 | 本地缓存 + 自动重连 |
| 配置错误 | Logstash停止 | 强制测试 + 自动回滚 |
| ES故障 | 服务不可用 | ES集群 + 本地备份 |
| 并发测试 | 资源耗尽 | 测试队列 + 资源限制 |

## 10. 技术选型理由

### 10.1 为什么选择Go？
- 高性能，适合Agent开发
- 部署简单，单二进制文件
- 并发处理能力强
- 跨平台支持好

### 10.2 为什么用Elasticsearch存储？
- 与ELK生态系统天然集成
- 支持全文搜索配置内容
- 自带版本控制机制
- 团队熟悉度高

### 10.3 为什么用WebSocket？
- 实时双向通信
- 低延迟配置推送
- 减少轮询开销
- 支持大规模Agent连接