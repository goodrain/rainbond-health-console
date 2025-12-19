# Rainbond Health Console 使用指南

## 快速开始

### 方式一：使用快速启动脚本（推荐）

```bash
./quick-start.sh
```

脚本提供以下选项：
1. 构建 AMD64 Docker 镜像（Mac ARM 用户）
2. 构建本地 Docker 镜像
3. 使用 Docker 运行
4. 部署到 Kubernetes
5. 本地开发模式运行

### 方式二：Docker 运行

```bash
# 1. 准备配置文件
cp .env.example .env
vim .env  # 编辑配置

# 2. 构建镜像
./build-amd64.sh  # Mac ARM 用户
# 或
docker build -t rainbond-health-console:latest .  # 其他用户

# 3. 运行容器
docker run -d \
  --name health-console \
  -p 9090:9090 \
  --env-file .env \
  rainbond-health-console:latest

# 4. 访问服务
curl http://localhost:9090/health
curl http://localhost:9090/metrics
```

### 方式三：Kubernetes 部署

```bash
# 1. 准备配置
cd deploy/kubernetes
cp secret.yaml.template secret.yaml
vim secret.yaml  # 编辑实际配置

# 2. 更新镜像地址
vim deploy.yaml  # 修改 image 字段

# 3. 执行部署
./deploy.sh

# 4. 访问服务
kubectl port-forward -n rbd-system svc/health-console 9090:9090
```

详细部署说明见 [deploy/kubernetes/README.md](deploy/kubernetes/README.md)

### 方式四：本地开发

```bash
# 1. 设置环境变量
export IN_CLUSTER=false
export METRICS_PORT=9090
export DB_1_NAME=test
export DB_1_HOST=localhost
export DB_1_PASSWORD=password

# 2. 运行
go run main.go

# 3. 访问
curl http://localhost:9090/metrics
```

---

## 配置说明

### 环境变量配置

所有配置通过环境变量传递，配置模板见 [.env.example](.env.example)

#### 基础配置

| 变量 | 说明 | 默认值 |
|-----|------|-------|
| `METRICS_PORT` | Metrics 端口 | 9090 |
| `COLLECT_INTERVAL` | 采集间隔 | 30s |
| `IN_CLUSTER` | K8s 集群内运行 | true |

#### 数据库配置（多实例）

```bash
# 实例 1
DB_1_NAME=internal
DB_1_HOST=mysql.svc.cluster.local
DB_1_PORT=3306
DB_1_USER=root
DB_1_PASSWORD=password
DB_1_DATABASE=mysql

# 实例 2（可选）
DB_2_NAME=external
DB_2_HOST=192.168.1.100
# ...
```

#### 镜像仓库配置（多实例）

```bash
# 实例 1
REGISTRY_1_NAME=internal
REGISTRY_1_URL=registry.cluster.local
REGISTRY_1_USER=admin
REGISTRY_1_PASSWORD=password
REGISTRY_1_INSECURE=false

# 实例 2（可选）
REGISTRY_2_NAME=dockerhub
REGISTRY_2_URL=https://registry.hub.docker.com
# ...
```

#### MinIO 配置

```bash
MINIO_ENDPOINT=minio.storage.svc.cluster.local:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_USE_SSL=false
```

---

## 监控指标

### P0 级别 - 基础设施监控

| 指标 | 说明 | 值 |
|-----|------|---|
| `mysql_up` | MySQL 可用性 | 1=正常, 0=异常 |
| `kubernetes_apiserver_up` | API Server 可用性 | 1=正常, 0=异常 |
| `coredns_up` | CoreDNS 可用性 | 1=正常, 0=异常 |
| `etcd_up` | Etcd 可用性 | 1=正常, 0=异常 |
| `cluster_storage_up` | 存储类可用性 | 1=正常, 0=异常 |
| `registry_up` | 镜像仓库可用性 | 1=正常, 0=异常 |
| `minio_up` | MinIO 可用性 | 1=正常, 0=异常 |

**说明**: 资源级别监控（磁盘空间、计算资源、节点状态）已迁移至 node-exporter 统一收集。

### 监控系统自身指标

- `health_check_errors_total` - 错误计数
- `health_check_duration_seconds` - 检查耗时

---

## 访问端点

| 端点 | 说明 |
|-----|------|
| `/metrics` | Prometheus metrics |
| `/health` | 健康检查 |
| `/` | 服务信息页面 |

---

## 常见场景

### 场景 1: 本地开发测试

```bash
# 最小配置运行
export IN_CLUSTER=false
export METRICS_PORT=9090
go run main.go
```

此时只会运行部分 collector，适合开发测试。

### 场景 2: Docker 快速体验

```bash
# 使用示例配置运行
cp .env.example .env
docker run -d -p 9090:9090 --env-file .env rainbond-health-console:latest
curl http://localhost:9090/metrics
```

### 场景 3: 生产环境部署

```bash
# 1. 准备配置
cd deploy/kubernetes
cp secret.yaml.template secret.yaml
vim secret.yaml  # 填写真实配置

# 2. 更新镜像
vim deploy.yaml  # 修改镜像地址

# 3. 调整资源限制（可选）
vim deploy.yaml  # 修改 resources 部分

# 4. 部署
./deploy.sh

# 5. 配置 Prometheus 告警
kubectl apply -f prometheus-rules.yaml
```

### 场景 4: 监控多个数据库

```env
# 配置 3 个数据库实例
DB_1_NAME=mysql-primary
DB_1_HOST=mysql-1.database.svc
DB_1_PASSWORD=pass1

DB_2_NAME=mysql-secondary
DB_2_HOST=mysql-2.database.svc
DB_2_PASSWORD=pass2

DB_3_NAME=mysql-external
DB_3_HOST=192.168.1.100
DB_3_PASSWORD=pass3
```

所有实例的指标都会带 `instance` 标签区分：
```
mysql_up{instance="mysql-primary"} 1
mysql_up{instance="mysql-secondary"} 1
mysql_up{instance="mysql-external"} 0
```

### 场景 5: 跳过某些监控

```env
# 不配置 MinIO（跳过 MinIO 监控）
# MINIO_ENDPOINT=

# 不配置数据库（跳过数据库监控）
# DB_1_NAME=
# DB_1_HOST=

# 不配置镜像仓库（跳过镜像仓库监控）
# REGISTRY_1_NAME=
# REGISTRY_1_URL=
```

---

## Prometheus 集成

### 自动发现（Prometheus Operator）

如果使用 Prometheus Operator，ServiceMonitor 会自动注册：

```bash
kubectl get servicemonitor -n rbd-system health-console
```

Prometheus 会自动抓取指标。

### 手动配置

在 Prometheus 配置中添加：

```yaml
scrape_configs:
  - job_name: 'health-console'
    static_configs:
      - targets: ['health-console.rbd-system.svc.cluster.local:9090']
```

### 告警规则

```bash
# 应用告警规则
kubectl apply -f deploy/kubernetes/prometheus-rules.yaml

# 验证
kubectl get prometheusrule -n rbd-system
```

---

## 故障排查

### 1. 服务无法启动

```bash
# Docker
docker logs health-console

# Kubernetes
kubectl logs -n rbd-system -l app=health-console
kubectl describe pod -n rbd-system -l app=health-console
```

常见原因：
- 配置错误（数据库连接失败等）
- 镜像拉取失败
- RBAC 权限不足

### 2. 某些指标缺失

检查日志中的 Warning：

```bash
kubectl logs -n rbd-system -l app=health-console | grep Warning
```

可能原因：
- 对应组件未配置
- 连接失败
- Kubernetes 权限不足

### 3. 健康检查失败

```bash
# 进入容器测试
kubectl exec -n rbd-system -it <pod-name> -- sh
wget -qO- http://localhost:9090/health
```

### 4. 指标未被 Prometheus 采集

```bash
# 检查 ServiceMonitor
kubectl get servicemonitor -n rbd-system

# 检查 Prometheus targets
# 访问 Prometheus UI，查看 Targets 页面
```

---

## 性能优化

### 调整采集间隔

```env
# 降低采集频率（适合大集群）
COLLECT_INTERVAL=60s

# 提高采集频率（适合小集群）
COLLECT_INTERVAL=15s
```

### 调整资源限制

```yaml
# deploy.yaml
resources:
  requests:
    memory: "256Mi"  # 根据集群规模调整
    cpu: "200m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

### 跳过不需要的监控

通过不配置相应的环境变量来跳过监控：

```env
# 只监控数据库和 K8s，跳过其他
DB_1_NAME=mysql
DB_1_HOST=mysql.svc
# 不配置 REGISTRY、MINIO 等
```

---

## 安全建议

1. **密码管理**
   - 使用 Kubernetes Secret
   - 不要在代码中硬编码密码
   - 定期轮换密码

2. **网络隔离**
   - 使用 NetworkPolicy 限制访问
   - 仅开放必要端口

3. **最小权限**
   - 审查 RBAC 权限
   - 使用非 root 用户运行

4. **TLS 加密**
   - 使用 HTTPS 连接外部服务
   - 配置 `MINIO_USE_SSL=true`

---

## 更多信息

- [完整 README](README.md)
- [Kubernetes 部署指南](deploy/kubernetes/README.md)
- [配置模板](.env.example)
- [Prometheus 告警规则](deploy/kubernetes/prometheus-rules.yaml)

---

## 获取帮助

如果遇到问题：

1. 查看日志排查问题
2. 检查配置是否正确
3. 查看 [故障排查](#故障排查) 章节
4. 提交 Issue 到 GitHub
