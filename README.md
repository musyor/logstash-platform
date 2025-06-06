# Logstash配置管理平台

集中管理Logstash配置规则，支持配置存储、分发、测试和版本控制。

## 核心功能

1. **配置集中存储**: 所有Logstash配置存储在Elasticsearch中
2. **配置分发**: 通过Agent自动同步配置到各Logstash节点
3. **配置测试**: 在应用配置前进行语法和逻辑测试
4. **版本管理**: 配置变更历史记录和回滚
5. **实时监控**: Agent状态和配置同步状态监控

## 系统组件

- **Platform Server**: 中心管理平台，提供API和Web界面
- **Agent**: 部署在Logstash节点，负责配置同步和应用
- **Elasticsearch**: 配置数据存储
- **消息队列**: 配置变更通知（可选）

## 技术栈

- 后端: Go (Gin框架)
- 存储: Elasticsearch
- 通信: WebSocket/gRPC
- 前端: Vue.js (计划)