#!/bin/bash

# Elasticsearch配置
ES_HOST="${ES_HOST:-http://localhost:9200}"
ES_USER="${ES_USER:-}"
ES_PASS="${ES_PASS:-}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

# 构建认证参数
AUTH=""
if [ -n "$ES_USER" ] && [ -n "$ES_PASS" ]; then
    AUTH="-u $ES_USER:$ES_PASS"
fi

echo -e "${YELLOW}初始化Elasticsearch索引...${NC}"
echo "ES地址: $ES_HOST"

# 检查ES连接
echo -e "\n${YELLOW}检查Elasticsearch连接...${NC}"
if curl -s $AUTH "$ES_HOST/_cluster/health" > /dev/null; then
    echo -e "${GREEN}✓ Elasticsearch连接成功${NC}"
else
    echo -e "${RED}✗ 无法连接到Elasticsearch${NC}"
    exit 1
fi

# 创建配置索引
echo -e "\n${YELLOW}创建logstash_configs索引...${NC}"
curl -s -X PUT $AUTH "$ES_HOST/logstash_configs" -H 'Content-Type: application/json' -d '{
  "mappings": {
    "properties": {
      "id": { "type": "keyword" },
      "name": { "type": "text" },
      "description": { "type": "text" },
      "type": { "type": "keyword" },
      "content": { "type": "text" },
      "tags": { "type": "keyword" },
      "version": { "type": "integer" },
      "enabled": { "type": "boolean" },
      "test_status": { "type": "keyword" },
      "created_at": { "type": "date" },
      "updated_at": { "type": "date" },
      "created_by": { "type": "keyword" },
      "updated_by": { "type": "keyword" }
    }
  }
}' | python3 -m json.tool

# 创建配置历史索引
echo -e "\n${YELLOW}创建logstash_config_history索引...${NC}"
curl -s -X PUT $AUTH "$ES_HOST/logstash_config_history" -H 'Content-Type: application/json' -d '{
  "mappings": {
    "properties": {
      "config_id": { "type": "keyword" },
      "version": { "type": "integer" },
      "content": { "type": "text" },
      "change_type": { "type": "keyword" },
      "change_log": { "type": "text" },
      "modified_by": { "type": "keyword" },
      "modified_at": { "type": "date" }
    }
  }
}' | python3 -m json.tool

# 创建Agent索引
echo -e "\n${YELLOW}创建logstash_agents索引...${NC}"
curl -s -X PUT $AUTH "$ES_HOST/logstash_agents" -H 'Content-Type: application/json' -d '{
  "mappings": {
    "properties": {
      "agent_id": { "type": "keyword" },
      "hostname": { "type": "keyword" },
      "ip": { "type": "ip" },
      "logstash_version": { "type": "keyword" },
      "status": { "type": "keyword" },
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
}' | python3 -m json.tool

# 检查索引创建状态
echo -e "\n${YELLOW}检查索引状态...${NC}"
curl -s $AUTH "$ES_HOST/_cat/indices/logstash_*?v"

echo -e "\n${GREEN}索引初始化完成！${NC}"