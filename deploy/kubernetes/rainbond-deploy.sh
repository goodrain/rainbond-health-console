#!/bin/bash

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Rainbond Health Console Deployment ===${NC}"
echo ""

# 检查 kubectl
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}Error: kubectl is not installed${NC}"
    exit 1
fi

# 检查 kubectl 连接
if ! kubectl cluster-info &> /dev/null; then
    echo -e "${RED}Error: Cannot connect to Kubernetes cluster${NC}"
    exit 1
fi

# 显示当前集群信息
CURRENT_CONTEXT=$(kubectl config current-context)
echo -e "${BLUE}Current Kubernetes context: ${YELLOW}${CURRENT_CONTEXT}${NC}"
echo ""

# 检查 rbd-system namespace
if ! kubectl get namespace rbd-system &> /dev/null; then
    echo -e "${RED}Error: rbd-system namespace not found${NC}"
    echo "This script is designed for Rainbond environments."
    echo "Please use deploy.sh for standard Kubernetes deployments."
    exit 1
fi

echo -e "${GREEN}Found rbd-system namespace${NC}"

# 检查 rainbond-operator ServiceAccount
if ! kubectl get serviceaccount rainbond-operator -n rbd-system &> /dev/null; then
    echo -e "${RED}Error: rainbond-operator ServiceAccount not found${NC}"
    echo "This script requires Rainbond to be installed."
    exit 1
fi

echo -e "${GREEN}Found rainbond-operator ServiceAccount${NC}"
echo ""

# 确认部署
read -p "Do you want to deploy Health Console to rbd-system namespace? (yes/no) " -r
echo
if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo -e "${YELLOW}Deployment cancelled${NC}"
    exit 0
fi

echo ""
echo -e "${BLUE}=== Configuration Guide ===${NC}"
echo ""
echo "Before deploying, you need to configure the following in rainbond-deploy.yaml:"
echo ""
echo -e "${YELLOW}1. Database Password:${NC}"
echo "   Find the Rainbond database password:"
echo "   kubectl get secret -n rbd-system <db-secret-name> -o jsonpath='{.data.password}' | base64 -d"
echo ""
echo -e "${YELLOW}2. Registry Configuration:${NC}"
echo "   Update registry URL and credentials if needed"
echo ""
echo -e "${YELLOW}3. Image Address:${NC}"
echo "   Update the container image to your registry"
echo ""

read -p "Have you updated the configuration in rainbond-deploy.yaml? (yes/no) " -r
echo
if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo -e "${YELLOW}Please edit rainbond-deploy.yaml first${NC}"
    echo ""
    echo "Required changes:"
    echo "  1. Line ~48: DB_1_PASSWORD"
    echo "  2. Line ~54: REGISTRY_1_PASSWORD"
    echo "  3. Line ~88: image address"
    echo ""
    exit 0
fi

echo ""
echo -e "${GREEN}Step 1: Applying ConfigMap${NC}"
kubectl apply -f rainbond-deploy.yaml --dry-run=client -o yaml | grep -A 50 "kind: ConfigMap" | grep -B 50 "^---" | head -n -1 | kubectl apply -f -

echo ""
echo -e "${GREEN}Step 2: Applying Secret${NC}"
kubectl apply -f rainbond-deploy.yaml --dry-run=client -o yaml | grep -A 100 "kind: Secret" | grep -B 100 "^---" | head -n -1 | kubectl apply -f -

echo ""
echo -e "${GREEN}Step 3: Deploying application${NC}"
kubectl apply -f rainbond-deploy.yaml

echo ""
echo -e "${GREEN}Step 4: Waiting for deployment to be ready${NC}"
kubectl rollout status deployment/health-console -n rbd-system --timeout=120s

echo ""
echo -e "${GREEN}=== Deployment Complete ===${NC}"
echo ""

# 显示部署状态
echo -e "${BLUE}Deployment Status:${NC}"
kubectl get deployment health-console -n rbd-system
echo ""

echo -e "${BLUE}Pod Status:${NC}"
kubectl get pods -n rbd-system -l name=health-console
echo ""

echo -e "${BLUE}Service Status:${NC}"
kubectl get svc health-console -n rbd-system
echo ""

# 显示访问信息
echo -e "${GREEN}=== Access Information ===${NC}"
echo ""
echo "To access metrics:"
echo "  kubectl port-forward -n rbd-system svc/health-console 9090:9090"
echo "  Then visit: http://localhost:9090/metrics"
echo ""
echo "Cluster internal access:"
echo "  http://health-console.rbd-system.svc.cluster.local:9090"
echo ""
echo "To view logs:"
echo "  kubectl logs -n rbd-system -l name=health-console -f"
echo ""
echo "To check specific metrics:"
echo "  kubectl exec -n rbd-system \$(kubectl get pod -n rbd-system -l name=health-console -o jsonpath='{.items[0].metadata.name}') -- wget -qO- http://localhost:9090/metrics | grep mysql_up"
echo ""

# 可选：检查 ServiceMonitor
if kubectl get servicemonitor -n rbd-system health-console &> /dev/null 2>&1; then
    echo -e "${GREEN}✓ ServiceMonitor created successfully${NC}"
    echo "  Prometheus should automatically discover this service"
else
    echo -e "${YELLOW}! ServiceMonitor not created (Prometheus Operator may not be installed)${NC}"
    echo "  You can manually configure Prometheus to scrape: health-console.rbd-system.svc.cluster.local:9090"
fi

echo ""

# 快速健康检查
echo -e "${BLUE}=== Quick Health Check ===${NC}"
echo "Waiting for pod to be ready..."
sleep 5

POD_NAME=$(kubectl get pod -n rbd-system -l name=health-console -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -n "$POD_NAME" ]; then
    echo "Checking health endpoint..."
    if kubectl exec -n rbd-system "$POD_NAME" -- wget -qO- http://localhost:9090/health &> /dev/null; then
        echo -e "${GREEN}✓ Health check passed${NC}"
    else
        echo -e "${YELLOW}! Health check failed - check logs for details${NC}"
        echo "  kubectl logs -n rbd-system $POD_NAME"
    fi

    echo ""
    echo "Checking metrics collection..."
    if kubectl exec -n rbd-system "$POD_NAME" -- wget -qO- http://localhost:9090/metrics 2>/dev/null | grep -q "mysql_up"; then
        echo -e "${GREEN}✓ Metrics collection working${NC}"
    else
        echo -e "${YELLOW}! Metrics may not be collecting - check configuration${NC}"
    fi
fi

echo ""
echo -e "${GREEN}Deployment completed successfully!${NC}"
echo ""
echo "Next steps:"
echo "  1. Check logs: kubectl logs -n rbd-system -l name=health-console -f"
echo "  2. Port forward: kubectl port-forward -n rbd-system svc/health-console 9090:9090"
echo "  3. View metrics: http://localhost:9090/metrics"
echo ""
