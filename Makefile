# 项目配置
DOCKER_DIR := docker
DIST_DIR := dist
DIST_BIN := $(DIST_DIR)/bin
DIST_CONFIG := $(DIST_DIR)/configs
DOCKER_COMPOSE := docker-compose -f $(DOCKER_DIR)/docker-compose.yml

# 服务名称
CHAT_SERVICE := game-chat-service
GATEWAY_SERVICE := game-gateway

# 编译产物
CHAT_BIN := bin/$(CHAT_SERVICE)
GATEWAY_BIN := bin/$(GATEWAY_SERVICE)

# 源文件 (用于依赖检查)
CHAT_SRC := $(shell find $(CHAT_SERVICE) -name "*.go" 2>/dev/null)
GATEWAY_SRC := $(shell find $(GATEWAY_SERVICE) -name "*.go" 2>/dev/null)
# 依赖生成的 Go 文件
GENERATED_GO := $(shell find game-protocols -name "*.pb.go" 2>/dev/null)

.PHONY: all help build release docker-up docker-down docker-restart docker-logs docker-ps docker-clean \
        run stop restart-app clean-dist psql redis-cli test-db test-redis stats

all: help

help: ## 显示帮助信息
	@echo "游戏开发项目 - 管理命令"
	@echo ""
	@echo "使用方法: make [命令]"
	@echo ""
	@echo "Docker 命令:"
	@grep -E '^[a-zA-Z_-]+:.*?## Docker: .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## Docker: "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "构建与运行命令:"
	@grep -E '^[a-zA-Z_-]+:.*?## App: .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## App: "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# --- 构建命令 ---

$(CHAT_BIN): $(CHAT_SRC) $(GENERATED_GO)
	@echo "🚀 编译 $(CHAT_SERVICE)..."
	@mkdir -p bin
	cd ./$(CHAT_SERVICE) && go build -o ../$(CHAT_BIN) ./cmd/chat/main.go

$(GATEWAY_BIN): $(GATEWAY_SRC) $(GENERATED_GO)
	@echo "🚀 编译 $(GATEWAY_SERVICE)..."
	@mkdir -p bin
	cd ./$(GATEWAY_SERVICE) && go build -o ../$(GATEWAY_BIN) ./cmd/gateway/main.go

build: $(CHAT_BIN) $(GATEWAY_BIN) ## App: 编译所有服务

release: build ## App: 编译并部署到 dist 目录
	@echo "📦 准备发布版本..."
	@mkdir -p $(DIST_BIN) $(DIST_CONFIG)
	@cp $(CHAT_BIN) $(DIST_BIN)/
	@cp $(GATEWAY_BIN) $(DIST_BIN)/
	@cp chat.yaml $(DIST_CONFIG)/
	@cp gateway.yaml $(DIST_CONFIG)/
	@echo "✅ 发布版本已就绪: $(DIST_DIR)"

# --- 运行命令 ---

run: release ## App: 启动服务 (后台运行)
	@echo "🟢 正在启动服务..."
	@mkdir -p logs
	@cd $(DIST_BIN) && ./$(GATEWAY_SERVICE) > ../../gateway.log 2>&1 & echo $$! > ../../gateway.pid
	@cd $(DIST_BIN) && ./$(CHAT_SERVICE) -config ../configs/chat.yaml > ../../chat.log 2>&1 & echo $$! > ../../chat.pid
	@echo "✅ 服务已在后台启动"

stop: ## App: 停止服务
	@echo "🔴 正在停止服务..."
	@if [ -f gateway.pid ]; then kill $$(cat gateway.pid) && rm gateway.pid && echo "Stop Gateway ok"; fi
	@if [ -f chat.pid ]; then kill $$(cat chat.pid) && rm chat.pid && echo "Stop Chat ok"; fi
	@echo "✅ 所有服务已停止"

restart-app: stop run ## App: 重启应用服务

# --- Docker 命令 (已调整路径) ---

docker-up: ## Docker: 启动所有基础服务
	$(DOCKER_COMPOSE) up -d
	@echo "✅ Docker 基础服务已启动"

docker-down: ## Docker: 停止所有 Docker 服务
	$(DOCKER_COMPOSE) down
	@echo "✅ Docker 服务已停止"

docker-restart: ## Docker: 重启 Docker 服务
	$(DOCKER_COMPOSE) restart

docker-logs: ## Docker: 查看日志
	$(DOCKER_COMPOSE) logs -f

docker-ps: ## Docker: 查看容器状态
	$(DOCKER_COMPOSE) ps

docker-clean: ## Docker: 清理容器和数据
	@echo "⚠️  警告: 这将删除所有数据！"
	@read -p "确认删除所有数据？(yes/no): " confirm && [ "$$confirm" = "yes" ] || exit 1
	$(DOCKER_COMPOSE) down -v

# --- 工具命令 ---

psql: ## Docker: 连接到 PostgreSQL
	docker exec -it game-postgres psql -U user -d game_chat

redis-cli: ## Docker: 连接到 Redis
	docker exec -it game-redis redis-cli

test-db: ## Docker: 测试数据库连接
	@docker exec game-postgres pg_isready -U user -d game_chat && echo "✅ PostgreSQL OK" || echo "❌ PostgreSQL Fail"

test-redis: ## Docker: 测试 Redis 连接
	@docker exec game-redis redis-cli ping > /dev/null 2>&1 && echo "✅ Redis OK" || echo "❌ Redis Fail"

stats: ## Docker: 显示资源使用
	docker stats --no-stream game-postgres game-redis

clean: ## App: 清理编译与发布目录
	rm -rf bin $(DIST_DIR) *.pid
	@echo "✅ 清理完成"

