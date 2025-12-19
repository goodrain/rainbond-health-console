#!/bin/bash

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Building AMD64 Docker Image on ARM Mac ===${NC}"

# 镜像名称和标签
IMAGE_NAME="rainbond-health-console"
IMAGE_TAG="latest"
FULL_IMAGE_NAME="${IMAGE_NAME}:${IMAGE_TAG}"

echo -e "${YELLOW}Image: ${FULL_IMAGE_NAME}${NC}"
echo -e "${YELLOW}Target Platform: linux/amd64${NC}"
echo ""

# 检查 Docker 是否安装
if ! command -v docker &> /dev/null; then
    echo -e "${RED}Error: Docker is not installed${NC}"
    exit 1
fi

# 检查 Docker 是否运行
if ! docker info &> /dev/null; then
    echo -e "${RED}Error: Docker is not running${NC}"
    exit 1
fi

# 创建 buildx builder（如果不存在）
BUILDER_NAME="multiarch-builder"

echo -e "${GREEN}Step 1: Setting up buildx builder${NC}"
if ! docker buildx inspect ${BUILDER_NAME} &> /dev/null; then
    echo "Creating new builder: ${BUILDER_NAME}"
    docker buildx create --name ${BUILDER_NAME} --driver docker-container --bootstrap --use
else
    echo "Using existing builder: ${BUILDER_NAME}"
    docker buildx use ${BUILDER_NAME}
fi

# 启动 builder
docker buildx inspect --bootstrap

echo ""
echo -e "${GREEN}Step 2: Building AMD64 image${NC}"
echo "This may take a few minutes..."

# 构建镜像并加载到本地 Docker
docker buildx build \
    --platform linux/amd64 \
    --tag ${FULL_IMAGE_NAME} \
    --load \
    .

echo ""
echo -e "${GREEN}Step 3: Verifying image${NC}"
docker images | grep ${IMAGE_NAME}

echo ""
echo -e "${GREEN}=== Build Complete ===${NC}"
echo -e "Image: ${GREEN}${FULL_IMAGE_NAME}${NC}"
echo -e "Architecture: ${GREEN}linux/amd64${NC}"
echo ""
echo "You can now:"
echo "  1. Run the image: docker run -d -p 9090:9090 ${FULL_IMAGE_NAME}"
echo "  2. Save to file: docker save ${FULL_IMAGE_NAME} -o ${IMAGE_NAME}.tar"
echo "  3. Push to registry: docker tag ${FULL_IMAGE_NAME} your-registry/${FULL_IMAGE_NAME} && docker push your-registry/${FULL_IMAGE_NAME}"
echo ""

# 可选：保存镜像到 tar 文件
read -p "Do you want to save the image to a tar file? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    TAR_FILE="${IMAGE_NAME}-amd64.tar"
    echo -e "${GREEN}Saving image to ${TAR_FILE}...${NC}"
    docker save ${FULL_IMAGE_NAME} -o ${TAR_FILE}
    echo -e "${GREEN}Saved to ${TAR_FILE}${NC}"
    echo "File size: $(du -h ${TAR_FILE} | cut -f1)"
fi

echo ""
echo -e "${GREEN}Done!${NC}"
