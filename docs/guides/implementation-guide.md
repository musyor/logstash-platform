# Logstash配置管理平台实施指南（简化版）

## 1. 快速开始

### 1.1 环境要求
- 现有Elasticsearch集群（用于配置存储）
- 现有Kafka集群（用于测试数据源）
- Go 1.21+（编译用）
- Logstash 7.x/8.x（各节点已安装）

### 1.2 系统部署步骤

#### Step 1: 部署管理平台
```bash
# 方式1：Docker部署（推荐）
docker run -d --name logstash-manager \
  -p 8080:8080 \
  -e ES_HOSTS=your-es-cluster:9200 \
  -e KAFKA_BROKERS=kafka1:9092,kafka2:9092 \
  -v /usr/share/logstash:/usr/share/logstash \
  logstash-manager:latest

# 方式2：二进制部署
wget https://releases/logstash-manager-linux-amd64
chmod +x logstash-manager-linux-amd64
./logstash-manager-linux-amd64 --config config.yaml
```

#### Step 2: 部署Agent到各Logstash节点
```bash
# 在每个Logstash节点上执行
curl -L https://your-platform:8080/agent/install.sh | bash -s -- \
  --platform-url=ws://your-platform:8080 \
  --agent-key=YOUR_SECURE_KEY \
  --logstash-path=/usr/share/logstash

# 或手动安装
wget https://releases/logstash-agent-linux-amd64
chmod +x logstash-agent-linux-amd64
sudo mv logstash-agent-linux-amd64 /usr/local/bin/logstash-agent

# 创建systemd服务
sudo tee /etc/systemd/system/logstash-agent.service << EOF
[Unit]
Description=Logstash Configuration Agent
After=network.target

[Service]
Type=simple
User=logstash
ExecStart=/usr/local/bin/logstash-agent
Restart=always
Environment="PLATFORM_URL=ws://your-platform:8080"
Environment="AGENT_KEY=YOUR_SECURE_KEY"

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl enable logstash-agent
sudo systemctl start logstash-agent
```

## 2. 配置示例

### 2.1 Nginx日志解析配置
```ruby
filter {
  grok {
    match => { 
      "message" => '%{IPORHOST:remote_addr} - %{DATA:user_name} \[%{HTTPDATE:time}\] "%{WORD:request_method} %{DATA:request_path} HTTP/%{NUMBER:http_version}" %{NUMBER:response_code} %{NUMBER:body_sent_bytes} "%{DATA:http_referer}" "%{DATA:http_user_agent}"'
    }
  }
  
  date {
    match => [ "time", "dd/MMM/yyyy:HH:mm:ss Z" ]
    target => "@timestamp"
  }
  
  mutate {
    convert => {
      "response_code" => "integer"
      "body_sent_bytes" => "integer"
    }
  }
}
```

### 2.2 JSON日志处理配置
```ruby
filter {
  json {
    source => "message"
    target => "parsed"
  }
  
  mutate {
    add_field => {
      "app_name" => "%{[parsed][app]}"
      "log_level" => "%{[parsed][level]}"
    }
  }
  
  if [parsed][level] == "ERROR" {
    mutate {
      add_tag => ["error", "alert"]
    }
  }
}
```

## 3. Agent配置文件

### 3.1 agent.yaml
```yaml
# Agent配置文件示例
server:
  platform_url: "ws://logstash-platform:8080"
  agent_key: "your-secure-agent-key"
  reconnect_interval: 30s
  heartbeat_interval: 30s

logstash:
  config_path: "/etc/logstash/conf.d"
  bin_path: "/usr/share/logstash/bin/logstash"
  reload_method: "signal"  # signal或restart
  test_timeout: 30s

version_control:
  base_path: "/etc/logstash-agent/configs"
  current_dir: "current"
  previous_dir: "previous"
  backup_dir: "backup"
  metadata_file: "/etc/logstash-agent/metadata.json"

logging:
  level: "info"
  file: "/var/log/logstash-agent/agent.log"
  max_size: "100MB"
  max_backups: 5
```

### 3.2 目录结构
```bash
/etc/logstash-agent/
├── agent.yaml          # Agent配置
├── configs/           # 版本控制目录
│   ├── current/       # 当前运行版本
│   ├── previous/      # 上一个版本
│   └── backup/        # 备份版本
└── metadata.json      # 版本元数据
```

## 4. 测试配置指南

### 4.1 使用样本数据测试
```bash
# 创建测试请求
curl -X POST "http://platform:8080/api/v1/test" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "config_id": "nginx-parser-config",
    "test_type": "sample",
    "samples": [
      "192.168.1.100 - - [01/Dec/2024:10:15:30 +0800] \"GET /api/users HTTP/1.1\" 200 1234",
      "192.168.1.101 - admin [01/Dec/2024:10:15:31 +0800] \"POST /api/login HTTP/1.1\" 401 567"
    ]
  }'
```

### 4.2 使用Kafka实时数据测试
```json
{
  "config_id": "app-log-filter",
  "test_type": "kafka",
  "kafka_config": {
    "brokers": ["kafka1:9092", "kafka2:9092"],
    "topic": "application-logs",
    "consumer_group": "test-${timestamp}",
    "max_messages": 10,
    "timeout_seconds": 5
  }
}
```

### 4.3 测试结果示例
```json
{
  "test_id": "test-123456",
  "status": "completed",
  "input_count": 2,
  "output_count": 2,
  "results": [
    {
      "input": "192.168.1.100 - - [01/Dec/2024:10:15:30 +0800] \"GET /api/users HTTP/1.1\" 200 1234",
      "output": {
        "remote_addr": "192.168.1.100",
        "request_method": "GET",
        "request_path": "/api/users",
        "response_code": 200,
        "body_sent_bytes": 1234,
        "@timestamp": "2024-12-01T02:15:30.000Z"
      }
    }
  ],
  "errors": []
}
```

## 5. API使用示例

### 5.1 使用curl测试API
```bash
# 获取配置列表
curl -X GET "http://localhost:8080/api/v1/configs?type=filter&page=1&size=10" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 创建新配置
curl -X POST "http://localhost:8080/api/v1/configs" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "name": "json-parser",
    "type": "filter",
    "content": "filter { json { source => \"message\" } }",
    "tags": ["json", "parser"],
    "description": "Parse JSON formatted logs"
  }'

# 部署配置到Agent（实时推送）
curl -X POST "http://localhost:8080/api/v1/deploy" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "config_id": "config-123",
    "agent_ids": ["agent-node1", "agent-node2"]
  }'

# 回滚配置
curl -X POST "http://localhost:8080/api/v1/agents/agent-node1/rollback" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 5.2 Python客户端示例
```python
import requests
import json

class LogstashManagerClient:
    def __init__(self, base_url, token):
        self.base_url = base_url
        self.headers = {"Authorization": f"Bearer {token}"}
    
    def create_config(self, config_data):
        """创建新配置"""
        response = requests.post(
            f"{self.base_url}/api/v1/configs",
            headers=self.headers,
            json=config_data
        )
        return response.json()
    
    def test_config_with_samples(self, config_id, samples):
        """使用样本数据测试配置"""
        response = requests.post(
            f"{self.base_url}/api/v1/test",
            headers=self.headers,
            json={
                "config_id": config_id,
                "test_type": "sample",
                "samples": samples
            }
        )
        return response.json()
    
    def test_config_with_kafka(self, config_id, kafka_config):
        """使用Kafka数据测试配置"""
        response = requests.post(
            f"{self.base_url}/api/v1/test",
            headers=self.headers,
            json={
                "config_id": config_id,
                "test_type": "kafka",
                "kafka_config": kafka_config
            }
        )
        return response.json()
    
    def deploy_config(self, config_id, agent_ids):
        """实时部署配置到Agent"""
        response = requests.post(
            f"{self.base_url}/api/v1/deploy",
            headers=self.headers,
            json={
                "config_id": config_id,
                "agent_ids": agent_ids
            }
        )
        return response.json()
    
    def rollback_agent(self, agent_id):
        """回滚Agent配置到上一版本"""
        response = requests.post(
            f"{self.base_url}/api/v1/agents/{agent_id}/rollback",
            headers=self.headers
        )
        return response.json()

# 使用示例
client = LogstashManagerClient("http://localhost:8080", "your-token")

# 创建配置
config = client.create_config({
    "name": "nginx-access-parser",
    "type": "filter",
    "content": """filter {
        grok {
            match => { "message" => "%{COMBINEDAPACHELOG}" }
        }
    }""",
    "tags": ["nginx", "access-log"]
})

# 使用样本测试
test_result = client.test_config_with_samples(
    config["id"], 
    ['192.168.1.1 - - [20/Dec/2024:10:00:00 +0800] "GET /index.html HTTP/1.1" 200 1234']
)

# 如果测试通过，部署到所有Agent
if test_result["status"] == "completed" and not test_result["errors"]:
    deploy_result = client.deploy_config(
        config["id"], 
        ["agent-node1", "agent-node2", "agent-node3"]
    )
    print(f"部署状态: {deploy_result}")
```

## 6. 故障排查

### 6.1 常见问题

#### Agent无法连接平台
```bash
# 检查网络连通性
telnet platform-host 8080

# 查看Agent日志
tail -f /var/log/logstash-agent/agent.log

# 验证Agent密钥
curl -X POST http://platform:8080/api/v1/agents/verify \
  -d '{"agent_key": "your-key"}'
```

#### 配置应用失败
```bash
# 手动测试配置
/usr/share/logstash/bin/logstash -t -f /tmp/test-config.conf

# 检查Logstash日志
tail -f /var/log/logstash/logstash-plain.log

# 查看Agent执行日志
grep "config apply" /var/log/logstash-agent/agent.log
```

### 6.2 性能优化

#### Elasticsearch优化
```yaml
# 增加配置索引的刷新间隔
PUT /logstash_configs/_settings
{
  "index": {
    "refresh_interval": "30s"
  }
}

# 优化查询性能
PUT /logstash_configs/_settings
{
  "index.max_result_window": 50000
}
```

#### Agent优化
```yaml
# agent.yaml
performance:
  max_concurrent_tests: 5
  config_cache_size: 100MB
  connection_pool_size: 10
```

## 7. 监控集成

### 7.1 Prometheus监控
```yaml
# prometheus配置
scrape_configs:
  - job_name: 'logstash-manager'
    static_configs:
      - targets: ['platform:8080']
    metrics_path: '/metrics'
```

### 7.2 关键指标
```
# 平台指标
logstash_manager_configs_total{type="filter"} 
logstash_manager_tests_total{status="passed"}
logstash_manager_agents_online

# Agent指标
logstash_agent_config_apply_duration_seconds
logstash_agent_last_heartbeat_timestamp
logstash_agent_config_version{config_id="xxx"}
```

## 8. 备份恢复

### 8.1 配置备份
```bash
# 导出所有配置
curl -X GET "http://platform:8080/api/v1/export" \
  -H "Authorization: Bearer TOKEN" \
  -o configs-backup-$(date +%Y%m%d).json

# ES快照备份（利用现有ES备份策略）
PUT /_snapshot/logstash_backup/snapshot_1
{
  "indices": ["logstash_configs*", "logstash_agents*"],
  "include_global_state": false
}
```

### 8.2 恢复流程
```bash
# 导入配置
curl -X POST "http://platform:8080/api/v1/import" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  --data-binary @configs-backup.json

# ES快照恢复
POST /_snapshot/logstash_backup/snapshot_1/_restore
{
  "indices": "logstash_configs*"
}
```

### 8.3 Agent本地备份
```bash
# Agent自动维护3个版本
ls -la /etc/logstash-agent/configs/
drwxr-xr-x  2 logstash logstash 4096 Dec 20 10:00 current/
drwxr-xr-x  2 logstash logstash 4096 Dec 20 09:00 previous/
drwxr-xr-x  2 logstash logstash 4096 Dec 19 10:00 backup/

# 手动备份当前配置
tar czf logstash-config-backup-$(date +%Y%m%d).tar.gz /etc/logstash-agent/configs/current/
```

## 9. 安全加固

### 9.1 网络安全
```nginx
# Nginx反向代理配置
server {
    listen 443 ssl;
    server_name logstash-manager.company.com;
    
    ssl_certificate /etc/nginx/certs/server.crt;
    ssl_certificate_key /etc/nginx/certs/server.key;
    
    location / {
        proxy_pass http://platform:8080;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
    
    location /ws {
        proxy_pass http://platform:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

### 9.2 访问控制
```yaml
# RBAC配置示例
roles:
  - name: admin
    permissions:
      - configs:*
      - agents:*
      - tests:*
  
  - name: developer
    permissions:
      - configs:read
      - configs:create
      - configs:update
      - tests:*
  
  - name: viewer
    permissions:
      - configs:read
      - agents:read
```

## 10. 简化版架构优势

### 10.1 架构简化点
- **仅两个新组件**：管理平台 + Agent
- **复用现有基础设施**：ES存储、Kafka数据源
- **内嵌测试引擎**：无需独立部署测试Logstash
- **实时配置推送**：WebSocket直接推送，响应快速
- **本地版本控制**：Agent自主管理3个版本

### 10.2 运维简化
- **部署简单**：管理平台单实例即可
- **Agent轻量**：Go编写的单二进制文件
- **无额外依赖**：充分利用现有ELK环境
- **故障隔离**：Agent故障不影响其他节点

### 10.3 未来扩展方向
- **配置模板化**：支持变量和模板
- **批量操作**：批量部署和回滚
- **配置继承**：基础配置+差异配置
- **审批流程**：生产环境变更审批

这份简化版实施指南专注于核心功能，确保快速上线并稳定运行。