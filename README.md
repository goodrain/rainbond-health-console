# Rainbond Health Console

Rainbond 平台稳定性监控服务，用于监控平台关键基础设施和资源的健康状态，并通过 Prometheus metrics 接口暴露指标。

## 功能特性

### P0 - 致命级别监控（平台核心基础设施）

- **数据库连接监控**：支持多个 MySQL 实例的连接健康检查
- **Kubernetes 集群监控**：
  - API Server 可用性
  - CoreDNS 服务状态和 DNS 解析
  - Etcd 集群健康
  - 存储类（StorageClass）可用性
- **容器镜像仓库监控**：支持多个 Registry 的连接检查
- **对象存储监控**：MinIO/S3 服务可用性

**说明**: 资源级别监控（磁盘空间、计算资源、节点状态）已迁移至 node-exporter 统一收集。

## 快速开始

### 前置要求

- Go 1.21+
- Kubernetes 集群（需要部署在集群内）
- 访问 Kubernetes API Server 的权限

### 编译

```bash
# 克隆项目
git clone <repository-url>
cd rainbond-health-console

# 下载依赖
go mod download

# 编译
go build -o health-console .
```

### 配置

所有配置通过环境变量传递：

#### 基础配置

| 环境变量 | 说明 | 默认值 | 必填 |
|---------|------|-------|-----|
| `METRICS_PORT` | Metrics 暴露端口 | 9090 | 否 |
| `COLLECT_INTERVAL` | 采集间隔（如 30s, 1m） | 30s | 否 |
| `IN_CLUSTER` | 是否运行在 K8s 集群内 | true | 否 |

#### 数据库配置（支持多实例）

格式：`DB_N_*`，其中 N 为实例编号（1, 2, 3, ...）

| 环境变量 | 说明 | 默认值 | 必填 |
|---------|------|-------|-----|
| `DB_1_NAME` | 实例名称（用于 metrics label） | - | 是 |
| `DB_1_HOST` | 数据库主机地址 | - | 是 |
| `DB_1_PORT` | 数据库端口 | 3306 | 否 |
| `DB_1_USER` | 数据库用户名 | root | 否 |
| `DB_1_PASSWORD` | 数据库密码 | - | 否 |
| `DB_1_DATABASE` | 数据库名称 | mysql | 否 |

示例：
```bash
# 第一个数据库
export DB_1_NAME="internal"
export DB_1_HOST="mysql.database.svc.cluster.local"
export DB_1_PORT="3306"
export DB_1_USER="root"
export DB_1_PASSWORD="password"

# 第二个数据库
export DB_2_NAME="external"
export DB_2_HOST="192.168.1.100"
export DB_2_PORT="3306"
export DB_2_USER="root"
export DB_2_PASSWORD="password"
```

#### 镜像仓库配置（支持多实例）

格式：`REGISTRY_N_*`，其中 N 为实例编号（1, 2, 3, ...）

| 环境变量 | 说明 | 默认值 | 必填 |
|---------|------|-------|-----|
| `REGISTRY_1_NAME` | 实例名称（用于 metrics label） | - | 是 |
| `REGISTRY_1_URL` | Registry URL | - | 是 |
| `REGISTRY_1_USER` | 用户名 | - | 否 |
| `REGISTRY_1_PASSWORD` | 密码 | - | 否 |
| `REGISTRY_1_INSECURE` | 是否使用 HTTP | false | 否 |

示例：
```bash
# 第一个 Registry
export REGISTRY_1_NAME="internal"
export REGISTRY_1_URL="registry.cluster.local"
export REGISTRY_1_USER="admin"
export REGISTRY_1_PASSWORD="password"
export REGISTRY_1_INSECURE="false"

# 第二个 Registry
export REGISTRY_2_NAME="dockerhub"
export REGISTRY_2_URL="https://registry.hub.docker.com"
```

#### MinIO 配置

| 环境变量 | 说明 | 默认值 | 必填 |
|---------|------|-------|-----|
| `MINIO_ENDPOINT` | MinIO 端点地址 | - | 否* |
| `MINIO_ACCESS_KEY` | Access Key | - | 否* |
| `MINIO_SECRET_KEY` | Secret Key | - | 否* |
| `MINIO_USE_SSL` | 是否使用 SSL | false | 否 |

*注意：如果不配置 MINIO_ENDPOINT，将跳过 MinIO 监控。

示例：
```bash
export MINIO_ENDPOINT="minio.storage.svc.cluster.local:9000"
export MINIO_ACCESS_KEY="minioadmin"
export MINIO_SECRET_KEY="minioadmin"
export MINIO_USE_SSL="false"
```

### Kubernetes 部署

#### 1. 创建 ServiceAccount 和 RBAC

```yaml
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
```

#### 2. 创建 ConfigMap（可选）

如果配置项较多，可以使用 ConfigMap：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: health-console-config
  namespace: rbd-system
data:
  METRICS_PORT: "9090"
  COLLECT_INTERVAL: "30s"
```

#### 3. 创建 Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: health-console-secrets
  namespace: rbd-system
type: Opaque
stringData:
  # 数据库配置
  DB_1_NAME: "internal"
  DB_1_HOST: "mysql.database.svc.cluster.local"
  DB_1_PASSWORD: "your-password"

  # Registry 配置
  REGISTRY_1_NAME: "internal"
  REGISTRY_1_URL: "registry.cluster.local"
  REGISTRY_1_PASSWORD: "your-password"

  # MinIO 配置
  MINIO_ENDPOINT: "minio.storage.svc.cluster.local:9000"
  MINIO_ACCESS_KEY: "minioadmin"
  MINIO_SECRET_KEY: "minioadmin"
```

#### 4. 创建 Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: health-console
  namespace: rbd-system
  labels:
    app: health-console
spec:
  replicas: 1
  selector:
    matchLabels:
      app: health-console
  template:
    metadata:
      labels:
        app: health-console
    spec:
      serviceAccountName: health-console
      containers:
      - name: health-console
        image: your-registry/health-console:latest
        ports:
        - containerPort: 9090
          name: metrics
        envFrom:
        - configMapRef:
            name: health-console-config
        - secretRef:
            name: health-console-secrets
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "200m"
        livenessProbe:
          httpGet:
            path: /health
            port: 9090
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 9090
          initialDelaySeconds: 5
          periodSeconds: 10
```

#### 5. 创建 Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: health-console
  namespace: rbd-system
  labels:
    app: health-console
spec:
  type: ClusterIP
  ports:
  - port: 9090
    targetPort: 9090
    name: metrics
  selector:
    app: health-console
```

#### 6. 创建 ServiceMonitor（用于 Prometheus 自动发现）

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: health-console
  namespace: rbd-system
  labels:
    app: health-console
spec:
  selector:
    matchLabels:
      app: health-console
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

## Metrics 指标

### P0 级别指标

| 指标名称 | 类型 | 标签 | 说明 |
|---------|------|-----|------|
| `mysql_up` | Gauge | instance | MySQL 可用性（1=正常，0=异常） |
| `kubernetes_apiserver_up` | Gauge | - | API Server 可用性 |
| `coredns_up` | Gauge | - | CoreDNS 可用性 |
| `etcd_up` | Gauge | - | Etcd 可用性 |
| `cluster_storage_up` | Gauge | storage_class | 存储类可用性 |
| `registry_up` | Gauge | instance | 镜像仓库可用性 |
| `minio_up` | Gauge | - | MinIO 可用性 |

### 监控系统自身指标

| 指标名称 | 类型 | 标签 | 说明 |
|---------|------|-----|------|
| `health_check_errors_total` | Counter | collector, error_type | 健康检查错误计数 |
| `health_check_duration_seconds` | Histogram | collector | 健康检查耗时 |

## Prometheus 告警规则示例

```yaml
groups:
- name: rainbond_platform
  interval: 30s
  rules:
  # P0 - 致命告警
  - alert: DatabaseDown
    expr: mysql_up == 0
    for: 1m
    labels:
      severity: critical
      level: P0
    annotations:
      summary: "数据库 {{ $labels.instance }} 不可用"

  - alert: KubernetesAPIServerDown
    expr: kubernetes_apiserver_up == 0
    for: 1m
    labels:
      severity: critical
      level: P0
    annotations:
      summary: "Kubernetes API Server 不可用"

  - alert: CoreDNSDown
    expr: coredns_up == 0
    for: 2m
    labels:
      severity: critical
      level: P0
    annotations:
      summary: "CoreDNS 服务异常"

  - alert: EtcdDown
    expr: etcd_up == 0
    for: 1m
    labels:
      severity: critical
      level: P0
    annotations:
      summary: "Etcd 集群不可用"
```

## 端点说明

- `GET /metrics` - Prometheus metrics 端点
- `GET /health` - 健康检查端点
- `GET /` - 服务信息页面

## 开发

### 项目结构

```
.
├── main.go                 # 主入口
├── config/
│   └── config.go          # 配置管理
├── collectors/
│   ├── database.go        # 数据库监控
│   ├── kubernetes.go      # K8s 集群监控
│   ├── registry.go        # 镜像仓库监控
│   └── storage.go         # 对象存储监控
├── metrics/
│   └── metrics.go         # Metrics 定义
├── go.mod
├── go.sum
└── README.md
```

### 本地开发

```bash
# 运行（需要 kubeconfig）
export IN_CLUSTER=false
export DB_1_NAME="test"
export DB_1_HOST="localhost"
go run main.go

# 查看 metrics
curl http://localhost:9090/metrics
```

## 许可证

[待定]

## 参考文档

- [platform-health-check-rules-refined.md](./platform-health-check-rules-refined.md) - 监控规则定义
# rainbond-health-console
