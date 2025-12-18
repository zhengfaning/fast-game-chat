# 游戏开发项目 - Docker 部署指南

本项目使用 Docker Compose 管理 PostgreSQL 和 Redis 服务，用于支持游戏聊天系统和网关服务。

## 📦 服务列表

### 核心服务
- **PostgreSQL** (端口: 5432) - 聊天消息持久化数据库
- **Redis** (端口: 6379) - 实时数据缓存和 Pub/Sub

### 管理工具（可选）
- **pgAdmin** (端口: 5050) - PostgreSQL 可视化管理工具
- **Redis Commander** (端口: 8081) - Redis 可视化管理工具

## 🚀 快速开始

### 1. 启动所有服务

```bash
# 启动所有服务（包含管理工具）
docker-compose up -d

# 仅启动核心服务（PostgreSQL + Redis）
docker-compose up -d postgres redis
```

### 2. 查看服务状态

```bash
# 查看所有服务状态
docker-compose ps

# 查看服务日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f postgres
docker-compose logs -f redis
```

### 3. 停止服务

```bash
# 停止所有服务
docker-compose down

# 停止服务并删除数据卷（谨慎使用！）
docker-compose down -v
```

## 🔧 配置说明

### PostgreSQL 配置

- **用户名**: `user`
- **密码**: `password`
- **数据库**: `game_chat`
- **端口**: `5432`
- **连接字符串**: `postgres://user:password@localhost:5432/game_chat?sslmode=disable`

### Redis 配置

- **地址**: `localhost:6379`
- **密码**: 无（开发环境）
- **持久化**: 启用 AOF

### 管理工具访问

#### pgAdmin
- **URL**: http://localhost:5050
- **邮箱**: admin@example.com
- **密码**: admin

连接到 PostgreSQL 服务器：
1. 在 pgAdmin 中添加新服务器
2. 主机名: `game-postgres` (Docker 网络内) 或 `localhost` (宿主机)
3. 端口: `5432`
4. 用户名: `user`
5. 密码: `password`

#### Redis Commander
- **URL**: http://localhost:8081

## 📊 数据库结构

数据库初始化脚本位于 `init-db/01-init-schema.sql`，包含以下表：

### 核心表
- **messages** - 消息表（支持私聊、频道、系统消息）
- **channels** - 频道表
- **channel_members** - 频道成员关系表
- **announcements** - 公告表
- **user_presence** - 用户在线状态表
- **user_blacklist** - 用户黑名单表
- **message_statistics** - 消息统计表

### 索引优化
所有表都已根据常见查询模式创建了优化索引，包括：
- 发送者/接收者索引
- 时间范围查询索引
- 复合索引（未读消息、频道消息等）

## 🔍 数据库操作

### 连接到 PostgreSQL

```bash
# 使用 Docker exec 连接
docker exec -it game-postgres psql -U user -d game_chat

# 使用本地 psql 客户端
psql -h localhost -p 5432 -U user -d game_chat
```

### 连接到 Redis

```bash
# 使用 Docker exec 连接
docker exec -it game-redis redis-cli

# 使用本地 redis-cli 客户端
redis-cli -h localhost -p 6379
```

### 常用 SQL 查询

```sql
-- 查看所有表
\dt

-- 查看消息表结构
\d messages

-- 查询最近的消息
SELECT id, sender_id, receiver_id, content, timestamp 
FROM messages 
ORDER BY timestamp DESC 
LIMIT 10;

-- 查询未读消息数量
SELECT receiver_id, COUNT(*) as unread_count 
FROM messages 
WHERE is_read = FALSE 
GROUP BY receiver_id;

-- 查询频道列表
SELECT * FROM channels WHERE is_active = TRUE;
```

## 🛠️ 开发环境配置

### 更新服务配置文件

根据 Docker 服务配置，已经匹配项目中的配置文件：

**game-chat-service/configs/chat.yaml**
```yaml
database:
  dsn: "postgres://user:password@localhost:5432/game_chat?sslmode=disable"

redis:
  addr: "localhost:6379"
  password: ""
```

**game-gateway/configs/gateway.yaml**
```yaml
redis:
  addr: "localhost:6379"
  password: ""
```

### 使用 Docker 网络（容器化部署）

如果服务也运行在 Docker 中，修改配置使用服务名：

```yaml
database:
  dsn: "postgres://user:password@game-postgres:5432/game_chat?sslmode=disable"

redis:
  addr: "game-redis:6379"
  password: ""
```

## 📈 性能优化建议

### PostgreSQL 优化
```sql
-- 定期清理和分析表
VACUUM ANALYZE messages;

-- 查看表大小
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

### Redis 优化
```bash
# 查看 Redis 内存使用
redis-cli INFO memory

# 查看连接数
redis-cli INFO clients

# 查看键统计
redis-cli --scan --pattern "*" | wc -l
```

## 🔐 生产环境注意事项

### 安全配置
1. **修改默认密码**：更改 PostgreSQL 和 Redis 密码
2. **限制访问**：配置防火墙规则，仅允许必要的 IP 访问
3. **使用环境变量**：不要在代码中硬编码密码

### 备份策略
```bash
# PostgreSQL 备份
docker exec game-postgres pg_dump -U user game_chat > backup_$(date +%Y%m%d).sql

# PostgreSQL 恢复
docker exec -i game-postgres psql -U user game_chat < backup_20231217.sql

# Redis 备份
docker exec game-redis redis-cli BGSAVE
docker cp game-redis:/data/dump.rdb ./redis_backup_$(date +%Y%m%d).rdb
```

### 资源限制
在生产环境中，建议在 `docker-compose.yml` 中添加资源限制：

```yaml
services:
  postgres:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 4G
        reservations:
          cpus: '1'
          memory: 2G
```

## 📚 相关文档

- [项目设计文档](./docs/design.md)
- [Gateway 实现计划](./docs/gateway_chat_implementation_plan.md)
- [多游戏架构](./docs/multi_game_architecture.md)

## 🆘 故障排查

### PostgreSQL 无法启动
```bash
# 查看日志
docker-compose logs postgres

# 检查数据卷
docker volume ls
docker volume inspect game_dev_postgres_data
```

### Redis 连接失败
```bash
# 测试连接
docker exec game-redis redis-cli ping

# 查看配置
docker exec game-redis redis-cli CONFIG GET "*"
```

### 数据库连接数过多
```sql
-- 查看当前连接
SELECT * FROM pg_stat_activity;

-- 关闭空闲连接
SELECT pg_terminate_backend(pid) 
FROM pg_stat_activity 
WHERE state = 'idle' 
AND state_change < NOW() - INTERVAL '5 minutes';
```

## 📝 更新日志

- **2025-12-17**: 初始版本，包含 PostgreSQL 15 和 Redis 7 配置
- 数据库表结构完全匹配设计文档要求
- 添加管理工具支持
- 包含完整的初始化脚本

## 🤝 贡献

如有问题或建议，请提交 Issue 或 Pull Request。
