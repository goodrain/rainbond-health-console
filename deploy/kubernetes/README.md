# Kubernetes Deployment Guide

本目录包含 Rainbond Health Console 的 Kubernetes 部署文件。

## 文件说明

| 文件 | 说明 |
|------|------|
| `deploy.yaml` | 完整部署配置（包含所有资源定义） |
| `secret.yaml.template` | Secret 配置模板（不包含真实密码） |
| `prometheus-rules.yaml` | Prometheus 告警规则 |
| `deploy.sh` | 快速部署脚本 |
| `README.md` | 本文档 |

## 快速部署（推荐）

### 步骤 1: 准备配置

```bash
# 复制 secret 模板
cp secret.yaml.template secret.yaml

# 编辑 secret.yaml，更新为实际的密码和配置
vim secret.yaml
```

**重要**：更新以下配置项：
- 数据库密码 (`DB_1_PASSWORD`)
- 镜像仓库密码 (`REGISTRY_1_PASSWORD`)
- MinIO 密钥 (`MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`)

### 步骤 2: 更新镜像地址

编辑 `deploy.yaml`，将镜像地址更新为你的实际镜像仓库：

```yaml
# 第 133 行附近
image: your-registry/health-console:latest
```

改为：

```yaml
image: registry.example.com/rainbond/health-console:latest
```

### 步骤 3: 执行部署

```bash
# 运行部署脚本
./deploy.sh
```

脚本会：
1. 检查 kubectl 连接
2. 显示当前集群信息并确认
3. 创建 namespace 和 RBAC
4. 部署应用
5. 应用 Secret
6. 等待部署就绪
7. 显示访问信息

## 手动部署

如果你想手动控制部署过程：

### 1. 创建 Secret

```bash
# 从模板创建 secret.yaml
cp secret.yaml.template secret.yaml
vim secret.yaml

# 应用 Secret
kubectl apply -f secret.yaml
```

### 2. 部署应用

```bash
# 应用所有资源（确保先创建了 secret.yaml）
kubectl apply -f deploy.yaml
```

### 3. 验证部署

```bash
# 检查 Pod 状态
kubectl get pods -n rbd-system -l app=health-console

# 查看日志
kubectl logs -n rbd-system -l app=health-console -f

# 检查服务
kubectl get svc -n rbd-system health-console
```

## 访问服务

### 端口转发访问

```bash
# 转发服务端口到本地
kubectl port-forward -n rbd-system svc/health-console 9090:9090

# 访问服务
curl http://localhost:9090/health
curl http://localhost:9090/metrics
```

### 集群内访问

服务地址：`http://health-console.rbd-system.svc.cluster.local:9090`

## 配置说明

### 必需配置

在 `deploy.yaml` 的 Secret 部分，以下配置是必需的：

```yaml
# 至少配置一个数据库
DB_1_NAME: "internal-mysql"
DB_1_HOST: "mysql.database.svc.cluster.local"
DB_1_PASSWORD: "your-password"

# 至少配置一个镜像仓库
REGISTRY_1_NAME: "internal-registry"
REGISTRY_1_URL: "registry.cluster.local"
REGISTRY_1_PASSWORD: "your-password"
```

### 可选配置

- **多数据库**：添加 `DB_2_*`、`DB_3_*` 等配置
- **多镜像仓库**：添加 `REGISTRY_2_*`、`REGISTRY_3_*` 等配置
- **MinIO 监控**：如果不需要，可以删除 `MINIO_*` 配置

### ConfigMap 调整

在 `deploy.yaml` 的 ConfigMap 部分可以调整：

```yaml
METRICS_PORT: "9090"           # Metrics 端口
COLLECT_INTERVAL: "30s"        # 采集间隔
NODE_CPU_THRESHOLD: "80"       # CPU 高负载阈值
NODE_MEMORY_THRESHOLD: "80"    # 内存高负载阈值
GRDATA_PATH: "/grdata"         # GRData 目录路径
```

### 挂载 /grdata 目录

如果需要监控宿主机的 `/grdata` 目录，在 `deploy.yaml` 中取消以下注释：

```yaml
# 第 168-177 行
volumeMounts:
- name: grdata
  mountPath: /grdata
  readOnly: true

volumes:
- name: grdata
  hostPath:
    path: /grdata
    type: DirectoryOrCreate
```

## Prometheus 集成

### ServiceMonitor（Prometheus Operator）

如果使用 Prometheus Operator，ServiceMonitor 会自动创建，Prometheus 将自动发现此服务。

验证：

```bash
kubectl get servicemonitor -n rbd-system health-console
```

### 手动配置（非 Operator）

如果没有 Prometheus Operator，需要手动添加 Prometheus 配置：

```yaml
scrape_configs:
  - job_name: 'health-console'
    kubernetes_sd_configs:
      - role: service
        namespaces:
          names:
            - rbd-system
    relabel_configs:
      - source_labels: [__meta_kubernetes_service_label_app]
        regex: health-console
        action: keep
```

### 告警规则

应用告警规则：

```bash
kubectl apply -f prometheus-rules.yaml
```

## 资源要求

默认资源配置：

```yaml
requests:
  memory: "128Mi"
  cpu: "100m"
limits:
  memory: "256Mi"
  cpu: "200m"
```

根据实际情况调整 `deploy.yaml` 中的资源配置。

## 故障排查

### Pod 无法启动

```bash
# 查看 Pod 详情
kubectl describe pod -n rbd-system -l app=health-console

# 查看日志
kubectl logs -n rbd-system -l app=health-console
```

常见问题：
- Secret 配置错误
- 镜像拉取失败
- RBAC 权限不足

### 健康检查失败

```bash
# 进入容器检查
kubectl exec -n rbd-system -it $(kubectl get pod -n rbd-system -l app=health-console -o jsonpath='{.items[0].metadata.name}') -- sh

# 测试健康端点
wget -qO- http://localhost:9090/health
```

### 指标未采集

检查配置：

```bash
# 查看环境变量
kubectl exec -n rbd-system -it $(kubectl get pod -n rbd-system -l app=health-console -o jsonpath='{.items[0].metadata.name}') -- env | grep -E "DB_|REGISTRY_|MINIO_"

# 查看日志中的错误
kubectl logs -n rbd-system -l app=health-console | grep -i error
```

## 更新部署

### 更新镜像

```bash
# 更新镜像
kubectl set image deployment/health-console health-console=your-registry/health-console:v1.0.1 -n rbd-system

# 查看滚动更新状态
kubectl rollout status deployment/health-console -n rbd-system
```

### 更新配置

```bash
# 修改 deploy.yaml 中的 ConfigMap 或 Secret

# 重新应用
kubectl apply -f deploy.yaml

# 重启 Pod 以加载新配置
kubectl rollout restart deployment/health-console -n rbd-system
```

## 卸载

### 完全卸载

```bash
# 删除所有资源
kubectl delete -f deploy.yaml

# 删除 ClusterRole 和 ClusterRoleBinding
kubectl delete clusterrole health-console
kubectl delete clusterrolebinding health-console
```

### 保留配置卸载

```bash
# 仅删除 Deployment 和 Service
kubectl delete deployment health-console -n rbd-system
kubectl delete service health-console -n rbd-system
kubectl delete servicemonitor health-console -n rbd-system
```

## 安全建议

1. **Secret 管理**
   - 不要将 `secret.yaml` 提交到版本控制
   - 建议使用外部密钥管理系统（如 Vault）
   - 定期轮换密码

2. **网络策略**
   - 限制 Pod 的网络访问
   - 仅允许必要的出站连接

3. **RBAC**
   - 审查 ClusterRole 权限
   - 根据实际需求调整权限范围

4. **资源限制**
   - 设置合理的资源限制
   - 防止资源耗尽

## 参考文档

- [项目 README](../../README.md)
- [Prometheus 告警规则](./prometheus-rules.yaml)
- [配置示例](../../.env.example)
