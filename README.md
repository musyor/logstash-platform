# Logstash配置管理平台

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go" alt="go version" />
  <img src="https://img.shields.io/badge/Vue-3.0+-4FC08D?style=flat&logo=vue.js" alt="vue version" />
  <img src="https://img.shields.io/badge/Elasticsearch-7.x/8.x-005571?style=flat&logo=elasticsearch" alt="elasticsearch version" />
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat" alt="license" />
</p>

## 📋 项目简介

Logstash配置管理平台是一个专为ELK Stack设计的配置集中管理系统，通过Web界面实现Logstash配置的统一管理、版本控制、实时分发和在线测试，彻底解决多节点Logstash集群的配置管理难题。

### 🎯 解决的核心问题

- **配置分散管理困难**：无需再登录每台服务器手动修改配置
- **配置验证风险高**：提供真实数据测试，上线前验证配置正确性
- **缺乏版本控制**：完整的配置版本历史，支持一键回滚
- **多节点同步复杂**：WebSocket实时推送，确保配置一致性

## ✨ 核心特性

### 配置管理
- 📝 Web界面配置编辑器，支持语法高亮
- 🏷️ 配置标签分类管理
- 📊 版本控制与历史追踪
- 🔄 配置导入导出

### 智能测试
- 🧪 内嵌Logstash测试引擎
- 📥 支持样本数据和Kafka实时数据测试
- 📈 测试结果可视化对比
- ✅ 测试通过后才允许部署

### 实时分发
- 🚀 WebSocket实时配置推送
- 📦 Agent本地三版本管理（current/previous/backup）
- ⚡ 秒级配置更新
- 🔙 一键回滚机制

### 运维友好
- 💻 Agent轻量级设计，资源占用小
- 🔌 利用现有ELK基础设施
- 📊 Agent状态实时监控
- 🛡️ 完整的操作审计日志

## 🏗️ 系统架构

```
┌─────────────────────────────────────────────────────┐
│                   Web Console                       │
│              (Vue.js + Element Plus)                │
└────────────────────────┬────────────────────────────┘
                         │ HTTPS
┌────────────────────────▼────────────────────────────┐
│              Management Platform                     │
│                  (Go + Gin)                         │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │Config API│ │ Test API │ │Agent API │           │
│  └──────────┘ └──────────┘ └──────────┘           │
│              内嵌测试Logstash                        │
└───────┬─────────────────────┬───────────────────────┘
        │                     │ WebSocket
        ▼                     ▼
   Elasticsearch          Agent集群
   (配置存储)         (部署在Logstash节点)
```

## 🛠️ 技术栈

### 后端
- **语言**：Go 1.21+
- **框架**：Gin Web Framework
- **存储**：Elasticsearch 7.x/8.x
- **通信**：WebSocket (gorilla/websocket)
- **消息队列**：Kafka (可选，用于测试数据源)

### 前端
- **框架**：Vue 3 + TypeScript
- **UI组件**：Element Plus
- **HTTP客户端**：Axios
- **代码编辑器**：Monaco Editor

### Agent
- **语言**：Go
- **部署**：单二进制文件
- **依赖**：无外部依赖

## 🚀 快速开始

### 环境要求
- Elasticsearch集群（已存在）
- Kafka集群（可选，用于测试）
- Logstash 7.x/8.x（各节点已安装）

### 部署管理平台

```bash
# Docker方式（推荐）
docker run -d --name logstash-manager \
  -p 8080:8080 \
  -e ES_HOSTS=your-es:9200 \
  -e KAFKA_BROKERS=kafka:9092 \
  logstash-manager:latest

# 或者使用docker-compose
docker-compose up -d
```

### 部署Agent

在每个Logstash节点执行：

```bash
# 下载并安装Agent
curl -L https://your-platform:8080/install.sh | bash

# 或手动安装
wget https://releases/logstash-agent-linux-amd64
chmod +x logstash-agent-linux-amd64
sudo mv logstash-agent-linux-amd64 /usr/local/bin/logstash-agent

# 启动Agent
logstash-agent --platform=ws://your-platform:8080 --key=YOUR_KEY
```

### 访问平台

打开浏览器访问 `http://your-platform:8080`

默认账号：`admin` / `admin123`

## 📅 开发计划

### Phase 1: MVP版本（4周）✅
- [x] 项目架构设计
- [x] 技术方案文档
- [ ] 后端基础框架
  - [ ] Elasticsearch存储层
  - [ ] 配置管理CRUD API
  - [ ] JWT认证中间件
- [ ] 前端基础框架
  - [ ] Vue3项目搭建
  - [ ] 基础布局和路由
  - [ ] 配置列表页面
  - [ ] 配置编辑器
- [ ] Agent基础功能
  - [ ] WebSocket连接
  - [ ] 配置接收和应用
  - [ ] 心跳机制

### Phase 2: 测试功能（3周）
- [ ] 测试引擎开发
  - [ ] 内嵌Logstash集成
  - [ ] 样本数据测试
  - [ ] Kafka数据源对接
  - [ ] 测试结果分析
- [ ] 前端测试中心
  - [ ] 测试任务创建
  - [ ] 结果可视化
  - [ ] 测试历史记录
- [ ] Agent版本管理
  - [ ] 三版本目录管理
  - [ ] 配置验证
  - [ ] 回滚机制

### Phase 3: 生产就绪（3周）
- [ ] 高级功能
  - [ ] 批量配置分发
  - [ ] 配置模板
  - [ ] 定时任务
- [ ] 运维功能
  - [ ] Agent监控大屏
  - [ ] 操作审计日志
  - [ ] 告警通知
- [ ] 性能优化
  - [ ] API性能优化
  - [ ] 前端加载优化
  - [ ] Agent资源优化

### Phase 4: 企业特性（4周）
- [ ] 高可用部署
- [ ] 多租户支持
- [ ] LDAP/AD集成
- [ ] 配置加密存储
- [ ] 自动化测试套件

## 📖 文档

- [技术设计文档](./docs/technical-design.md)
- [实施指南](./docs/implementation-guide.md)
- [API文档](./docs/api.md)
- [部署文档](./docs/deployment.md)

## 🤝 贡献指南

我们欢迎所有形式的贡献，包括但不限于：

- 🐛 提交Bug和建议
- 📝 改进文档
- 🚀 提交新功能
- 🎨 改进UI/UX

请查看 [CONTRIBUTING.md](./CONTRIBUTING.md) 了解详情。

## 📄 许可证

本项目采用 MIT 许可证，详见 [LICENSE](./LICENSE) 文件。

## 🙏 致谢

- 感谢Elastic团队提供优秀的ELK Stack
- 感谢所有贡献者的努力

## 📞 联系我们

- 问题反馈：[GitHub Issues](https://github.com/musyor/logstash-platform/issues)
- 邮箱：logstash-manager@example.com

---

<p align="center">Made with ❤️ by Logstash Manager Team</p>