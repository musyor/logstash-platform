# Project Memory
Last Updated: 2025-06-06

## Project Overview
- Project Name: Logstash Platform
- Technology Stack: Go, Logstash, Elasticsearch, WebSocket
- Main Objectives: 构建一个Logstash配置管理平台，实现配置的统一存储、版本管理、自动分发和真实数据测试

## Current Architecture
基于简化版架构设计：
- Management Platform (Go/Gin) - 管理平台核心
- Agent (Go) - 部署在各Logstash节点的轻量级代理
- 内嵌测试引擎 - 管理平台内置的测试Logstash实例
- 存储层 - 复用现有Elasticsearch集群
- 数据源 - 复用现有Kafka集群用于测试

## Key Technical Decisions
1. 采用WebSocket实现配置实时推送，减少轮询开销
2. Agent本地维护3个版本（current/previous/backup）支持快速回滚
3. 配置存储在Elasticsearch中，利用其版本控制和全文搜索能力
4. 内嵌Logstash测试引擎，避免独立部署测试环境
5. 文档统一管理在docs目录下，按功能分类组织

## Known Issues
- 项目刚初始化，核心功能待开发
- GitHub仓库已连接：https://github.com/musyor/logstash-platform