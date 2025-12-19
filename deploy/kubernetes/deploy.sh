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

# 确认部署
read -p "Do you want to deploy to this cluster? (yes/no) " -r
echo
if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo -e "${YELLOW}Deployment cancelled${NC}"
    exit 0
fi

# 检查 secret.yaml 是否存在
if [ ! -f "secret.yaml" ]; then
    echo -e "${YELLOW}Warning: secret.yaml not found!${NC}"
    echo "Please create secret.yaml from secret.yaml.template:"
    echo "  1. cp secret.yaml.template secret.yaml"
    echo "  2. Edit secret.yaml and update with your actual credentials"
    echo ""
    read -p "Do you want to continue with the template file (NOT RECOMMENDED for production)? (yes/no) " -r
    echo
    if [[ $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
        SECRET_FILE="secret.yaml.template"
        echo -e "${YELLOW}Using template file - remember to update secrets later!${NC}"
    else
        echo -e "${RED}Deployment cancelled${NC}"
        exit 1
    fi
else
    SECRET_FILE="secret.yaml"
    echo -e "${GREEN}Found secret.yaml${NC}"
fi

echo ""
echo -e "${GREEN}Step 1: Creating namespace and RBAC${NC}"

# 应用 RBAC 和基础配置
kubectl apply -f - <<EOF
---
apiVersion: v1
kind: Namespace
metadata:
  name: rbd-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: health-console
  namespace: rbd-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: health-console
rules:
- apiGroups: [""]
  resources: ["nodes", "pods", "services"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["storage.k8s.io"]
  resources: ["storageclasses"]
  verbs: ["get", "list"]
- apiGroups: ["metrics.k8s.io"]
  resources: ["nodes", "pods"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: health-console
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: health-console
subjects:
- kind: ServiceAccount
  name: health-console
  namespace: rbd-system
EOF

echo ""
echo -e "${GREEN}Step 2: Deploying application${NC}"

# 提示用户更新镜像地址
echo ""
echo -e "${YELLOW}Please make sure to update the image in deploy.yaml:${NC}"
echo "  image: your-registry/health-console:latest"
echo ""
read -p "Press Enter to continue..."

# 应用主部署文件（不包括 Secret）
kubectl apply -f <(grep -v "kind: Secret" deploy.yaml | grep -v "^# Secret" | awk '/^---$/{if(p)print;p=0}/kind: Secret/{p=1;next}{if(!p)print}')

# 应用 Secret
echo ""
echo -e "${GREEN}Step 3: Applying secrets${NC}"
kubectl apply -f "${SECRET_FILE}"

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
kubectl get pods -n rbd-system -l app=health-console
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
echo "To view logs:"
echo "  kubectl logs -n rbd-system -l app=health-console -f"
echo ""
echo "To check health:"
echo "  kubectl exec -n rbd-system -it \$(kubectl get pod -n rbd-system -l app=health-console -o jsonpath='{.items[0].metadata.name}') -- wget -qO- http://localhost:9090/health"
echo ""

# 可选：检查 ServiceMonitor
if kubectl get servicemonitor -n rbd-system health-console &> /dev/null; then
    echo -e "${GREEN}ServiceMonitor created successfully${NC}"
    echo "Prometheus should automatically discover this service"
else
    echo -e "${YELLOW}Note: ServiceMonitor requires Prometheus Operator${NC}"
fi

echo ""
echo -e "${GREEN}Done!${NC}"
