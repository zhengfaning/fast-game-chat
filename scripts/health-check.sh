#!/bin/bash

# 游戏开发项目 - 服务健康检查脚本

echo "🔍 检查 Docker 服务状态..."
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检查 PostgreSQL
echo -n "PostgreSQL: "
if docker exec game-postgres pg_isready -U user -d game_chat > /dev/null 2>&1; then
    echo -e "${GREEN}✅ 运行正常${NC}"
    PG_VERSION=$(docker exec game-postgres psql -U user -d game_chat -t -c "SELECT version();" | head -n 1 | xargs)
    echo "   版本: ${PG_VERSION:0:50}..."
else
    echo -e "${RED}❌ 连接失败${NC}"
fi

echo ""

# 检查 Redis
echo -n "Redis: "
if docker exec game-redis redis-cli ping > /dev/null 2>&1; then
    echo -e "${GREEN}✅ 运行正常${NC}"
    REDIS_VERSION=$(docker exec game-redis redis-cli INFO server | grep redis_version | cut -d: -f2 | tr -d '\r')
    echo "   版本: ${REDIS_VERSION}"
else
    echo -e "${RED}❌ 连接失败${NC}"
fi

echo ""

# 检查数据库表
echo "📊 数据库表统计:"
docker exec game-postgres psql -U user -d game_chat -t -c "
SELECT 
    tablename as 表名,
    pg_size_pretty(pg_total_relation_size('public.'||tablename)) as 大小
FROM pg_tables 
WHERE schemaname = 'public' 
ORDER BY tablename;
" | grep -v "^$"

echo ""

# 检查频道数据
echo "📢 频道列表:"
docker exec game-postgres psql -U user -d game_chat -t -c "
SELECT id, name, channel_type, game_id 
FROM channels 
WHERE is_active = true;
" | grep -v "^$"

echo ""

# 检查 Redis 键数量
echo -n "🔑 Redis 键数量: "
REDIS_KEYS=$(docker exec game-redis redis-cli DBSIZE | cut -d: -f2)
echo "${REDIS_KEYS}"

echo ""

# 检查容器资源使用
echo "💻 容器资源使用:"
docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}" \
    game-postgres game-redis game-pgadmin game-redis-commander 2>/dev/null

echo ""

# 检查管理工具
echo "🌐 管理工具访问地址:"
echo -e "   pgAdmin:         ${YELLOW}http://localhost:5050${NC} (邮箱: admin@example.com, 密码: admin)"
echo -e "   Redis Commander: ${YELLOW}http://localhost:8081${NC}"

echo ""
echo "✅ 健康检查完成！"
