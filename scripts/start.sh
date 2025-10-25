#!/bin/bash

set -e

echo "================================"
echo "Instrument Server 启动脚本"
echo "================================"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 1. 检查Go环境
echo -e "\n${YELLOW}[1/6] 检查Go环境...${NC}"
if ! command -v go &> /dev/null; then
    echo -e "${RED}错误: 未找到Go，请先安装Go 1.21+${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Go版本: $(go version)${NC}"

# 2. 检查Redis
echo -e "\n${YELLOW}[2/6] 检查Redis...${NC}"
if ! command -v redis-cli &> /dev/null; then
    echo -e "${RED}警告: 未找到redis-cli${NC}"
    echo -e "${YELLOW}尝试启动Docker Redis...${NC}"
    docker run -d --name instrument-redis -p 6379:6379 redis:alpine || true
else
    if redis-cli ping &> /dev/null; then
        echo -e "${GREEN}✓ Redis运行正常${NC}"
    else
        echo -e "${RED}错误: Redis未运行，请启动Redis${NC}"
        exit 1
    fi
fi

# 3. 下载依赖
echo -e "\n${YELLOW}[3/6] 下载依赖...${NC}"
go mod tidy
echo -e "${GREEN}✓ 依赖下载完成${NC}"

# 4. 编译
echo -e "\n${YELLOW}[4/6] 编译项目...${NC}"
mkdir -p build
go build -o build/instrument-server cmd/server/main.go
echo -e "${GREEN}✓ 编译完成${NC}"

# 5. 检查配置文件
echo -e "\n${YELLOW}[5/6] 检查配置文件...${NC}"
if [ ! -f "configs/config.yaml" ]; then
    echo -e "${RED}错误: 配置文件不存在: configs/config.yaml${NC}"
    exit 1
fi
echo -e "${GREEN}✓ 配置文件存在${NC}"

# 6. 启动服务器
echo -e "\n${YELLOW}[6/6] 启动服务器...${NC}"
echo -e "${GREEN}================================${NC}"
./build/instrument-server -config configs/config.yaml
