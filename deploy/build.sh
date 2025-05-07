#!/bin/bash

# 终端颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# 脚本所在目录
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
# 项目根目录
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

# 输出信息
echo -e "${GREEN}Gateway 容器构建脚本${NC}"
echo -e "${YELLOW}项目路径: ${ROOT_DIR}${NC}"

# 切换到项目根目录
cd "$ROOT_DIR"

# 检查docker和docker-compose是否安装
if ! command -v docker &> /dev/null; then
    echo -e "${RED}错误: Docker 未安装，请先安装 Docker${NC}"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}错误: Docker Compose 未安装，请先安装 Docker Compose${NC}"
    exit 1
fi

# 创建.env文件（如果不存在）
if [ ! -f "$SCRIPT_DIR/.env" ]; then
    echo -e "${YELLOW}创建默认.env文件${NC}"
    cat > "$SCRIPT_DIR/.env" << EOL
# 日志级别
LOG_LEVEL=info
EOL
    echo -e "${GREEN}已创建.env文件，请根据需要修改配置${NC}"
fi

# 构建选项
read -p "是否清理Docker缓存并重新构建所有镜像? (y/n) " -n 1 -r REBUILD
echo
if [[ $REBUILD =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}清理Docker缓存并重新构建...${NC}"
    docker-compose -f "$SCRIPT_DIR/docker-compose.yml" build --no-cache
else
    echo -e "${YELLOW}使用缓存构建...${NC}"
    docker-compose -f "$SCRIPT_DIR/docker-compose.yml" build
fi

# 部署选项
read -p "是否立即部署并启动容器? (y/n) " -n 1 -r DEPLOY
echo
if [[ $DEPLOY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}启动服务...${NC}"
    docker-compose -f "$SCRIPT_DIR/docker-compose.yml" up -d

    echo -e "${GREEN}服务已启动！${NC}"
    echo -e "${GREEN}前端访问地址: http://localhost:3000${NC}"
    echo -e "${GREEN}后端API地址: http://localhost:8080${NC}"

    echo -e "${YELLOW}容器状态:${NC}"
    docker-compose -f "$SCRIPT_DIR/docker-compose.yml" ps
else
    echo -e "${YELLOW}构建完成。使用以下命令启动服务:${NC}"
    echo -e "${GREEN}cd $SCRIPT_DIR && docker-compose up -d${NC}"
fi

echo -e "${GREEN}构建过程完成!${NC}"
