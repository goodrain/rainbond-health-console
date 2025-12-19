# Rainbond 环境部署指南

本指南说明如何在 Rainbond 集群环境中部署 Health Console。

## 特点

- 使用 `rainbond-operator` ServiceAccount（无需额外创建 RBAC）
- 遵循 Rainbond 组件命名规范
- 自动挂载 `/grdata` 目录
- 集成 Rainbond 数据库配置

## 快速部署

### 1. 修改配置

编辑 `rainbond-deploy.yaml` 中的 Secret 部分：

```yaml
stringData:
  # 数据库配置 - 使用你的实际密码
  DB_1_PASSWORD: "your-actual-password"  # 修改这里

  # 镜像仓库配置 - 根据实际情况配置
  REGISTRY_1_URL: "rbd-hub:5000"         # 修改为实际地址
  REGISTRY_1_PASSWORD: "your-password"    # 修改这里

  # MinIO 配置 - 如果有的话
  MINIO_ENDPOINT: "rbd-minio:9000"       # 修改为实际地址
  MINIO_ACCESS_KEY: "your-access-key"     # 修改这里
  MINIO_SECRET_KEY: "your-secret-key"     # 修改这里
```

### 2. 修改镜像地址

```yaml
# 第 88 行附近
image: registry.cn-hangzhou.aliyuncs.com/zhangqihang/health-console:latest
```

修改为你的实际镜像地址。

### 3. 应用部署

```bash
kubectl apply -f rainbond-deploy.yaml
```

### 4. 验证部署

```bash
# 检查 Pod 状态
kubectl get pods -n rbd-system -l name=health-console

# 查看日志
kubectl logs -n rbd-system -l name=health-console -f

# 检查服务
kubectl get svc -n rbd-system health-console
```

## 配置说明

### 默认配置

该部署文件已经配置好了适合 Rainbond 环境的默认值：

#### 数据库配置
- **Host**: `rbd-db-rw` (Rainbond 数据库服务)
- **Port**: `3306`
- **User**: `root`
- **Database**: `region`
- **Password**: 需要修改为实际密码

#### 镜像仓库配置
- **URL**: `rbd-hub:5000` (Rainbond 内置镜像仓库)
- **Insecure**: `true` (因为内部使用 HTTP)
- **Username/Password**: 需要修改

#### MinIO 配置（可选）
- **Endpoint**: `rbd-minio:9000`
- 如果不使用 MinIO，可以删除相关配置

#### 其他配置
- **GRData Path**: `/grdata` (自动挂载到宿主机 /grdata)
- **Metrics Port**: `9090`
- **Collect Interval**: `30s`
- **Node Thresholds**: CPU 80%, Memory 80%

### ServiceAccount

使用 Rainbond 的 `rainbond-operator` ServiceAccount，该账户已经具有：
- 访问节点信息的权限
- 访问 Pod 信息的权限
- 访问 StorageClass 的权限
- 访问 Metrics API 的权限

**无需额外创建 RBAC 配置！**

## 访问服务

### 集群内访问

```bash
# Metrics 端点
curl http://health-console.rbd-system.svc.cluster.local:9090/metrics

# 健康检查
curl http://health-console.rbd-system.svc.cluster.local:9090/health
```

### 端口转发访问

```bash
kubectl port-forward -n rbd-system svc/health-console 9090:9090

# 然后在本地访问
curl http://localhost:9090/metrics
```

### 浏览器访问

```bash
kubectl port-forward -n rbd-system svc/health-console 9090:9090
```

然后访问：http://localhost:9090

## Prometheus 集成

### 自动发现

如果 Rainbond 环境中有 Prometheus Operator，ServiceMonitor 会自动创建：

```bash
kubectl get servicemonitor -n rbd-system health-console
```

Prometheus 会自动抓取 `health-console` 的指标。

### 手动配置

如果没有 Prometheus Operator，在 Prometheus 配置中添加：

```yaml
scrape_configs:
  - job_name: 'health-console'
    static_configs:
      - targets: ['health-console.rbd-system.svc.cluster.local:9090']
```

## 监控指标说明

部署成功后，可以监控以下组件：

### P0 级别（基础设施）
- ✅ MySQL 数据库 (`rbd-db-rw`)
- ✅ Kubernetes API Server
- ✅ CoreDNS
- ✅ Etcd
- ✅ StorageClass
- ✅ 镜像仓库 (`rbd-hub`)
- ⚠️  MinIO（如果配置）

### P1 级别（资源）
- ✅ /grdata 磁盘空间
- ✅ 节点磁盘空间
- ✅ 集群 CPU/内存资源
- ✅ 节点状态和负载

## 查看监控数据

### 使用 curl

```bash
# 查看所有指标
kubectl exec -n rbd-system -it $(kubectl get pod -n rbd-system -l name=health-console -o jsonpath='{.items[0].metadata.name}') -- wget -qO- http://localhost:9090/metrics

# 查看数据库状态
kubectl exec -n rbd-system -it $(kubectl get pod -n rbd-system -l name=health-console -o jsonpath='{.items[0].metadata.name}') -- wget -qO- http://localhost:9090/metrics | grep mysql_up

# 查看磁盘使用率
kubectl exec -n rbd-system -it $(kubectl get pod -n rbd-system -l name=health-console -o jsonpath='{.items[0].metadata.name}') -- wget -qO- http://localhost:9090/metrics | grep disk_space_usage_percent
```

### 使用 Prometheus

在 Prometheus UI 中查询：

```promql
# 数据库状态
mysql_up{instance="rbd-db"}

# /grdata 磁盘使用率
disk_space_usage_percent{path="/grdata"}

# 集群可用内存百分比
cluster_memory_available_percent

# 节点就绪状态
node_ready
```

## 故障排查

### Pod 无法启动

```bash
# 查看 Pod 状态
kubectl describe pod -n rbd-system -l name=health-console

# 查看日志
kubectl logs -n rbd-system -l name=health-console
```

常见问题：
1. **镜像拉取失败**：检查镜像地址是否正确
2. **数据库连接失败**：检查 Secret 中的数据库密码
3. **权限不足**：确认 `rainbond-operator` ServiceAccount 存在

### 数据库连接失败

检查数据库密码是否正确：

```bash
# 获取 Rainbond 数据库密码
kubectl get secret -n rbd-system rbd-db -o jsonpath='{.data.password}' | base64 -d

# 更新 Secret
kubectl edit secret -n rbd-system health-console-secrets
```

### 镜像仓库连接失败

检查镜像仓库地址和凭证：

```bash
# 测试镜像仓库连接
kubectl exec -n rbd-system -it $(kubectl get pod -n rbd-system -l name=health-console -o jsonpath='{.items[0].metadata.name}') -- wget -qO- http://rbd-hub:5000/v2/
```

### 指标未采集

查看日志中的警告信息：

```bash
kubectl logs -n rbd-system -l name=health-console | grep -i warning
```

## 更新部署

### 更新镜像

```bash
kubectl set image deployment/health-console health-console=your-registry/health-console:new-version -n rbd-system

# 查看更新状态
kubectl rollout status deployment/health-console -n rbd-system
```

### 更新配置

```bash
# 编辑 ConfigMap
kubectl edit configmap health-console-config -n rbd-system

# 或编辑 Secret
kubectl edit secret health-console-secrets -n rbd-system

# 重启 Pod 以应用新配置
kubectl rollout restart deployment/health-console -n rbd-system
```

## 卸载

```bash
# 删除所有资源
kubectl delete -f rainbond-deploy.yaml
```

## 与标准部署的区别

| 特性 | 标准部署 (deploy.yaml) | Rainbond 部署 (rainbond-deploy.yaml) |
|-----|----------------------|-----------------------------------|
| ServiceAccount | 创建新的 `health-console` | 使用现有的 `rainbond-operator` |
| RBAC | 需要创建 ClusterRole/Binding | 无需创建（使用现有权限） |
| 标签风格 | 标准 K8s 标签 | Rainbond 风格标签 |
| /grdata 挂载 | 可选（需手动启用） | 默认启用 |
| 数据库配置 | 需手动配置 | 预配置 Rainbond 数据库 |
| 镜像仓库配置 | 需手动配置 | 预配置 rbd-hub |

## 推荐配置

对于生产环境，建议：

1. **资源限制**：根据集群规模调整
   ```yaml
   resources:
     requests:
       memory: "256Mi"
       cpu: "200m"
     limits:
       memory: "512Mi"
       cpu: "500m"
   ```

2. **采集间隔**：大集群建议增加间隔
   ```yaml
   COLLECT_INTERVAL: "60s"  # 60秒一次
   ```

3. **副本数**：建议保持 1 个副本（避免重复采集）
   ```yaml
   replicas: 1
   ```

## 参考文档

- [完整使用指南](../../USAGE.md)
- [配置说明](../../README.md)
- [标准 K8s 部署](./README.md)
