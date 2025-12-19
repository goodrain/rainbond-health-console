#!/bin/bash

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Rainbond Health Console Quick Start ===${NC}"
echo ""

# 显示菜单
echo "Please select an option:"
echo ""
echo "  1) Build AMD64 Docker image (for Mac ARM users)"
echo "  2) Build local Docker image"
echo "  3) Run with Docker"
echo "  4) Deploy to Kubernetes"
echo "  5) Run locally (development mode)"
echo "  6) Exit"
echo ""

read -p "Enter your choice (1-6): " choice

case $choice in
    1)
        echo -e "${GREEN}Building AMD64 Docker image...${NC}"
        if [ ! -f "build-amd64.sh" ]; then
            echo -e "${RED}Error: build-amd64.sh not found${NC}"
            exit 1
        fi
        ./build-amd64.sh
        ;;

    2)
        echo -e "${GREEN}Building local Docker image...${NC}"
        docker build -t rainbond-health-console:latest .
        echo -e "${GREEN}Build complete!${NC}"
        echo "Image: rainbond-health-console:latest"
        ;;

    3)
        echo -e "${GREEN}Running with Docker...${NC}"
        echo ""

        # 检查 .env 文件
        if [ ! -f ".env" ]; then
            echo -e "${YELLOW}Creating .env file from template...${NC}"
            if [ -f ".env.example" ]; then
                cp .env.example .env
                echo -e "${YELLOW}Please edit .env file with your actual configuration${NC}"
                read -p "Press Enter after editing .env file..."
            else
                echo -e "${RED}Error: .env.example not found${NC}"
                exit 1
            fi
        fi

        echo "Starting container..."
        docker run -d \
            --name health-console \
            -p 9090:9090 \
            --env-file .env \
            rainbond-health-console:latest

        echo -e "${GREEN}Container started!${NC}"
        echo ""
        echo "Access the service:"
        echo "  - Metrics: http://localhost:9090/metrics"
        echo "  - Health: http://localhost:9090/health"
        echo "  - Info: http://localhost:9090/"
        echo ""
        echo "View logs:"
        echo "  docker logs -f health-console"
        echo ""
        echo "Stop container:"
        echo "  docker stop health-console && docker rm health-console"
        ;;

    4)
        echo -e "${GREEN}Deploying to Kubernetes...${NC}"
        echo ""

        if [ ! -f "deploy/kubernetes/deploy.sh" ]; then
            echo -e "${RED}Error: deploy/kubernetes/deploy.sh not found${NC}"
            exit 1
        fi

        cd deploy/kubernetes
        ./deploy.sh
        cd ../..
        ;;

    5)
        echo -e "${GREEN}Running in development mode...${NC}"
        echo ""

        # 检查 Go
        if ! command -v go &> /dev/null; then
            echo -e "${RED}Error: Go is not installed${NC}"
            exit 1
        fi

        # 设置开发环境变量
        export IN_CLUSTER=false
        export METRICS_PORT=9090
        export COLLECT_INTERVAL=30s

        echo -e "${YELLOW}Development mode configuration:${NC}"
        echo "  IN_CLUSTER=false"
        echo "  METRICS_PORT=9090"
        echo "  COLLECT_INTERVAL=30s"
        echo ""
        echo -e "${YELLOW}Note: You need to set database and registry configuration${NC}"
        echo "Example:"
        echo "  export DB_1_NAME=test"
        echo "  export DB_1_HOST=localhost"
        echo "  export DB_1_PASSWORD=password"
        echo ""

        read -p "Do you want to continue? (y/n) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 0
        fi

        echo -e "${GREEN}Starting application...${NC}"
        go run main.go
        ;;

    6)
        echo -e "${YELLOW}Goodbye!${NC}"
        exit 0
        ;;

    *)
        echo -e "${RED}Invalid choice${NC}"
        exit 1
        ;;
esac

echo ""
echo -e "${GREEN}Done!${NC}"
