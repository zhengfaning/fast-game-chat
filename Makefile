.PHONY: help up down restart logs clean backup restore test-db test-redis

help: ## 显示帮助信息
	@echo "游戏开发项目 - Docker 管理命令"
	@echo ""
	@echo "使用方法: make [命令]"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

up: ## 启动所有服务
	docker-compose up -d
	@echo "✅ 所有服务已启动"
	@echo "📊 PostgreSQL: localhost:5432"
	@echo "📊 Redis: localhost:6379"
	@echo "🌐 pgAdmin: http://localhost:5050"
	@echo "🌐 Redis Commander: http://localhost:8081"

up-core: ## 仅启动核心服务 (PostgreSQL + Redis)
	docker-compose up -d postgres redis
	@echo "✅ 核心服务已启动"

down: ## 停止所有服务
	docker-compose down
	@echo "✅ 所有服务已停止"

restart: ## 重启所有服务
	docker-compose restart
	@echo "✅ 所有服务已重启"

logs: ## 查看所有服务日志
	docker-compose logs -f

logs-postgres: ## 查看 PostgreSQL 日志
	docker-compose logs -f postgres

logs-redis: ## 查看 Redis 日志
	docker-compose logs -f redis

ps: ## 查看服务状态
	docker-compose ps

clean: ## 停止服务并清理数据卷 (谨慎使用！)
	@echo "⚠️  警告: 这将删除所有数据！"
	@read -p "确认删除所有数据？(yes/no): " confirm && [ "$$confirm" = "yes" ] || exit 1
	docker-compose down -v
	@echo "✅ 服务已停止，数据已清理"

backup-db: ## 备份 PostgreSQL 数据库
	@mkdir -p backups
	docker exec game-postgres pg_dump -U user game_chat > backups/game_chat_$(shell date +%Y%m%d_%H%M%S).sql
	@echo "✅ 数据库已备份到 backups/ 目录"

backup-redis: ## 备份 Redis 数据
	@mkdir -p backups
	docker exec game-redis redis-cli BGSAVE
	@sleep 2
	docker cp game-redis:/data/dump.rdb backups/redis_$(shell date +%Y%m%d_%H%M%S).rdb
	@echo "✅ Redis 已备份到 backups/ 目录"

restore-db: ## 恢复数据库 (使用方法: make restore-db FILE=backup.sql)
	@if [ -z "$(FILE)" ]; then \
		echo "❌ 错误: 请指定备份文件，例如: make restore-db FILE=backups/game_chat_20231217.sql"; \
		exit 1; \
	fi
	docker exec -i game-postgres psql -U user game_chat < $(FILE)
	@echo "✅ 数据库已恢复"

psql: ## 连接到 PostgreSQL
	docker exec -it game-postgres psql -U user -d game_chat

redis-cli: ## 连接到 Redis
	docker exec -it game-redis redis-cli

test-db: ## 测试数据库连接
	@docker exec game-postgres pg_isready -U user -d game_chat && \
		echo "✅ PostgreSQL 连接正常" || \
		echo "❌ PostgreSQL 连接失败"

test-redis: ## 测试 Redis 连接
	@docker exec game-redis redis-cli ping > /dev/null 2>&1 && \
		echo "✅ Redis 连接正常" || \
		echo "❌ Redis 连接失败"

test: test-db test-redis ## 测试所有服务连接

stats: ## 显示资源使用统计
	@echo "📊 Docker 容器资源使用:"
	docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}" game-postgres game-redis

db-size: ## 查看数据库大小
	@echo "📊 数据库表大小:"
	@docker exec game-postgres psql -U user -d game_chat -c "\
		SELECT \
			schemaname, \
			tablename, \
			pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size \
		FROM pg_tables \
		WHERE schemaname = 'public' \
		ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;"

db-connections: ## 查看数据库连接数
	@echo "📊 当前数据库连接:"
	@docker exec game-postgres psql -U user -d game_chat -c "\
		SELECT \
			pid, \
			usename, \
			application_name, \
			client_addr, \
			state, \
			state_change \
		FROM pg_stat_activity \
		WHERE datname = 'game_chat';"

redis-info: ## 查看 Redis 信息
	@echo "📊 Redis 信息:"
	@docker exec game-redis redis-cli INFO | grep -E "(redis_version|uptime_in_days|connected_clients|used_memory_human|total_commands_processed)"

redis-keys: ## 查看 Redis 键数量
	@echo "📊 Redis 键统计:"
	@docker exec game-redis redis-cli DBSIZE

init-sample-data: ## 初始化示例数据
	@echo "💾 插入示例数据..."
	@docker exec -i game-postgres psql -U user -d game_chat << 'EOF'
	-- 插入测试用户在线状态
	INSERT INTO user_presence (user_id, game_id, status) VALUES
		(1001, 'mmo', 'online'),
		(1002, 'mmo', 'online'),
		(1003, 'mmo', 'offline')
	ON CONFLICT (user_id) DO UPDATE SET status = EXCLUDED.status;
	
	-- 插入测试消息
	INSERT INTO messages (sender_id, receiver_id, content, message_type) VALUES
		(1001, 1002, '你好！欢迎来到游戏世界！', 'private'),
		(1002, 1001, '谢谢！这个游戏真不错！', 'private');
	
	-- 插入测试公告
	INSERT INTO announcements (title, content, announcement_type, game_id, start_time, end_time, created_by) VALUES
		('欢迎公告', '欢迎来到我们的游戏！', 'game', 'mmo', NOW(), NOW() + INTERVAL '7 days', 'system'),
		('维护通知', '服务器将于今晚 22:00 进行维护', 'maintenance', 'mmo', NOW(), NOW() + INTERVAL '1 day', 'admin');
	
	SELECT '✅ 示例数据已插入' AS status;
	EOF

clean-messages: ## 清空消息表 (保留其他数据)
	@echo "⚠️  清空消息表..."
	@read -p "确认清空消息表？(yes/no): " confirm && [ "$$confirm" = "yes" ] || exit 1
	docker exec game-postgres psql -U user -d game_chat -c "TRUNCATE TABLE messages RESTART IDENTITY CASCADE;"
	@echo "✅ 消息表已清空"

migrate: ## 运行数据库迁移 (预留接口)
	@echo "🔄 运行数据库迁移..."
	@echo "ℹ️  提示: 请实现您的迁移工具逻辑"

rebuild: down ## 重建服务 (删除容器但保留数据)
	docker-compose up -d --build
	@echo "✅ 服务已重建"

prune: ## 清理未使用的 Docker 资源
	docker system prune -f
	@echo "✅ Docker 资源已清理"
