# Rainbond 健康监控 Prometheus 查询指南

**生成时间**: 2025-12-19
**版本**: 1.0

---

## 目录

- [一、P0 级别监控指标（致命）](#一p0-级别监控指标致命)
  - [1.1 数据库监控](#11-数据库监控)
  - [1.2 Kubernetes 集群监控](#12-kubernetes-集群监控)
  - [1.3 镜像仓库监控](#13-镜像仓库监控)
  - [1.4 对象存储监控](#14-对象存储监控)
- [二、监控系统自身指标](#二监控系统自身指标)
- [三、复合查询与分析](#三复合查询与分析)
- [四、Grafana 仪表盘查询](#四grafana-仪表盘查询)

---

## 一、P0 级别监控指标（致命）

### 1.1 数据库监控

#### 指标：`mysql_up`
数据库可用性状态（1=正常，0=异常）

| 查询场景 | PromQL 查询语句 |
|---------|----------------|
| **查询所有数据库状态** | `mysql_up` |
| **查询特定数据库状态** | `mysql_up{instance="rbd-db-region"}` |
| **查询 console 数据库状态** | `mysql_up{instance="rbd-db-console"}` |
| **告警：数据库不可达** | `mysql_up == 0` |
| **统计不可用数据库数量** | `count(mysql_up == 0)` |
| **计算数据库可用率** | `avg(mysql_up) * 100` |
| **查询过去 1 小时宕机的数据库** | `mysql_up == 0 and changes(mysql_up[1h]) > 0` |
| **数据库可用性时间序列** | `mysql_up{instance="rbd-db-region"}[5m]` |
| **数据库在线时长（秒）** | `time() - mysql_up == 1` |

---

### 1.2 Kubernetes 集群监控

#### 指标：`kubernetes_apiserver_up`
Kubernetes API Server 可用性（1=正常，0=异常）

| 查询场景 | PromQL 查询语句 |
|---------|----------------|
| **查询 API Server 状态** | `kubernetes_apiserver_up` |
| **告警：API Server 不可用** | `kubernetes_apiserver_up == 0` |
| **API Server 可用率（24小时）** | `avg_over_time(kubernetes_apiserver_up[24h]) * 100` |
| **API Server 故障次数（1小时）** | `changes(kubernetes_apiserver_up[1h])` |

---

#### 指标：`coredns_up`
CoreDNS 服务可用性（1=正常，0=异常）

| 查询场景 | PromQL 查询语句 |
|---------|----------------|
| **查询 CoreDNS 状态** | `coredns_up` |
| **告警：CoreDNS 不可用** | `coredns_up == 0` |
| **CoreDNS 可用率（24小时）** | `avg_over_time(coredns_up[24h]) * 100` |
| **CoreDNS 重启次数（1小时）** | `resets(coredns_up[1h])` |

---

#### 指标：`etcd_up`
Etcd 集群可用性（1=正常，0=异常）

| 查询场景 | PromQL 查询语句 |
|---------|----------------|
| **查询 Etcd 状态** | `etcd_up` |
| **告警：Etcd 不可用** | `etcd_up == 0` |
| **Etcd 可用率（24小时）** | `avg_over_time(etcd_up[24h]) * 100` |
| **Etcd 故障次数（6小时）** | `changes(etcd_up[6h])` |

---

#### 指标：`cluster_storage_up`
存储类可用性（1=正常，0=异常）

| 查询场景 | PromQL 查询语句 |
|---------|----------------|
| **查询所有存储类状态** | `cluster_storage_up` |
| **查询特定存储类状态** | `cluster_storage_up{storage_class="local-path"}` |
| **告警：存储类不可用** | `cluster_storage_up == 0` |
| **统计可用存储类数量** | `count(cluster_storage_up == 1)` |
| **统计不可用存储类** | `count(cluster_storage_up == 0)` |
| **存储类可用率** | `avg(cluster_storage_up) * 100` |

---

### 1.3 镜像仓库监控

#### 指标：`registry_up`
容器镜像仓库可用性（1=正常，0=异常）

| 查询场景 | PromQL 查询语句 |
|---------|----------------|
| **查询所有镜像仓库状态** | `registry_up` |
| **查询特定镜像仓库状态** | `registry_up{instance="rbd-registry"}` |
| **告警：镜像仓库不可达** | `registry_up == 0` |
| **统计不可用仓库数量** | `count(registry_up == 0)` |
| **镜像仓库可用率（24小时）** | `avg_over_time(registry_up{instance="rbd-registry"}[24h]) * 100` |
| **镜像仓库故障次数（1小时）** | `changes(registry_up[1h])` |

---

### 1.4 对象存储监控

#### 指标：`minio_up`
MinIO/S3 对象存储可用性（1=正常，0=异常）

| 查询场景 | PromQL 查询语句 |
|---------|----------------|
| **查询 MinIO 状态** | `minio_up` |
| **告警：MinIO 不可用** | `minio_up == 0` |
| **MinIO 可用率（24小时）** | `avg_over_time(minio_up[24h]) * 100` |
| **MinIO 故障次数（1小时）** | `changes(minio_up[1h])` |

---

## 三、监控系统自身指标

#### 指标：`health_check_errors_total`
健康检查错误总数（Counter）

| 查询场景 | PromQL 查询语句 |
|---------|----------------|
| **查询所有错误总数** | `health_check_errors_total` |
| **按采集器分组统计错误** | `sum by (collector) (health_check_errors_total)` |
| **按错误类型分组统计** | `sum by (error_type) (health_check_errors_total)` |
| **查询特定采集器错误数** | `health_check_errors_total{collector="database"}` |
| **错误增长率（每分钟）** | `rate(health_check_errors_total[5m]) * 60` |
| **过去 1 小时错误增量** | `increase(health_check_errors_total[1h])` |
| **告警：错误率过高** | `rate(health_check_errors_total[5m]) > 0.1` |

---

#### 指标：`health_check_duration_seconds`
健康检查耗时（Histogram）

| 查询场景 | PromQL 查询语句 |
|---------|----------------|
| **查询平均检查耗时** | `rate(health_check_duration_seconds_sum[5m]) / rate(health_check_duration_seconds_count[5m])` |
| **按采集器分组查询平均耗时** | `sum by (collector) (rate(health_check_duration_seconds_sum[5m])) / sum by (collector) (rate(health_check_duration_seconds_count[5m]))` |
| **查询特定采集器耗时** | `rate(health_check_duration_seconds_sum{collector="kubernetes"}[5m]) / rate(health_check_duration_seconds_count{collector="kubernetes"}[5m])` |
| **查询 P95 延迟** | `histogram_quantile(0.95, rate(health_check_duration_seconds_bucket[5m]))` |
| **查询 P99 延迟** | `histogram_quantile(0.99, rate(health_check_duration_seconds_bucket[5m]))` |
| **告警：检查耗时过长** | `rate(health_check_duration_seconds_sum[5m]) / rate(health_check_duration_seconds_count[5m]) > 10` |

---

## 四、复合查询与分析

### 4.1 综合健康状态

| 查询场景 | PromQL 查询语句 |
|---------|----------------|
| **P0 级别所有组件状态** | `mysql_up and kubernetes_apiserver_up and coredns_up and etcd_up and minio_up` |
| **统计 P0 级别故障数量** | `count(mysql_up == 0) + count(kubernetes_apiserver_up == 0) + count(coredns_up == 0) + count(etcd_up == 0) + count(registry_up == 0) + count(minio_up == 0)` |
| **集群核心组件健康分数（0-100）** | `(count(mysql_up == 1) + count(kubernetes_apiserver_up == 1) + count(coredns_up == 1) + count(etcd_up == 1)) / (count(mysql_up) + 3) * 100` |

---

## 五、Grafana 仪表盘查询

### 5.1 概览面板

#### 面板：P0 级别组件状态矩阵
```promql
# 数据库状态
mysql_up

# K8s 核心组件
kubernetes_apiserver_up
coredns_up
etcd_up

# 基础设施
registry_up
minio_up
```

**可视化类型**: Stat / State Timeline
**阈值设置**:
- 红色: `value == 0` (故障)
- 绿色: `value == 1` (正常)

---

### 5.2 数据库监控面板

#### 面板：数据库可用性
```promql
mysql_up
```

**可视化类型**: Stat
**Legend**: `{{instance}}`

---

#### 面板：数据库可用率趋势（24小时）
```promql
avg_over_time(mysql_up{instance="rbd-db-region"}[24h]) * 100
avg_over_time(mysql_up{instance="rbd-db-console"}[24h]) * 100
```

**可视化类型**: Graph
**Y轴**: Percent (0-100)

---

### 5.3 告警面板

#### 面板：当前活跃告警
```promql
# P0 级别告警
ALERTS{severity="critical", priority="P0", alertstate="firing"}
```

**可视化类型**: Alert List / Table

---

#### 面板：告警历史
```promql
# 过去 24 小时告警次数
count_over_time(ALERTS{alertstate="firing"}[24h])
```

**可视化类型**: Bar Chart

---

## 六、实用查询示例

### 6.1 日常巡检查询

#### 每日健康检查清单
```promql
# 1. 检查所有 P0 组件是否正常
mysql_up
kubernetes_apiserver_up
coredns_up
etcd_up
registry_up
minio_up

# 2. 检查是否有错误
sum(rate(health_check_errors_total[5m]))
```

---

### 6.2 故障排查查询

#### 场景：平台核心服务异常
```promql
# 1. 检查 API Server 是否正常
kubernetes_apiserver_up

# 2. 检查 Etcd 状态
etcd_up

# 3. 检查数据库连接
mysql_up
```

---

#### 场景：应用无法部署
```promql
# 1. 检查镜像仓库
registry_up

# 2. 检查存储类
cluster_storage_up

# 3. 检查 API Server
kubernetes_apiserver_up
```

---

#### 场景：DNS 解析失败
```promql
# 1. 检查 CoreDNS 状态
coredns_up

# 2. 检查 API Server
kubernetes_apiserver_up

# 3. 查看 CoreDNS 历史状态
coredns_up[1h]
```

---

### 6.3 健康检查错误分析

#### 监控系统自身健康
```promql
# 1. 错误率统计
sum(rate(health_check_errors_total[5m])) by (collector)

# 2. 检查采集器性能
topk(5, rate(health_check_duration_seconds_sum[5m]) / rate(health_check_duration_seconds_count[5m]))
```

---

## 七、告警规则 PromQL

### 7.1 P0 级别告警查询

```promql
# 数据库不可达
mysql_up == 0

# API Server 不可用
kubernetes_apiserver_up == 0

# CoreDNS 不可用
coredns_up == 0

# Etcd 不可用
etcd_up == 0

# 存储类不可用
cluster_storage_up == 0

# 镜像仓库不可达
registry_up == 0

# MinIO 不可用
minio_up == 0
```

---

## 八、查询优化建议

### 8.1 性能优化

1. **使用时间范围限制**
   ```promql
   # 不推荐（查询所有历史数据）
   mysql_up

   # 推荐（限制时间范围）
   mysql_up[5m]
   ```

2. **使用聚合减少数据量**
   ```promql
   # 不推荐（返回所有数据库实例数据）
   mysql_up

   # 推荐（只返回计数）
   count(mysql_up)
   ```

3. **避免过度使用 subquery**
   ```promql
   # 不推荐（嵌套查询）
   avg_over_time((sum(mysql_up))[1h:1m])

   # 推荐（简化查询）
   avg_over_time(mysql_up[1h])
   ```

---

### 8.2 告警规则优化

1. **使用 `for` 子句避免瞬时抖动**
   ```yaml
   # 不推荐
   - alert: HighCPU
     expr: node_cpu_usage_percent > 90

   # 推荐（持续 5 分钟才告警）
   - alert: HighCPU
     expr: node_cpu_usage_percent > 90
     for: 5m
   ```

2. **合理设置告警阈值**
   ```promql
   # P0 级别：立即告警
   mysql_up == 0

   # P1 级别：持续一段时间后告警
   disk_space_usage_percent{path="/grdata"} > 90 for 5m
   ```

---

### 8.3 Grafana 变量

定义可重用的变量：

```promql
# 变量：节点列表
label_values(node_ready, node)

# 变量：数据库实例
label_values(mysql_up, instance)

# 变量：时间范围
$__range

# 使用变量的查询
node_cpu_usage_percent{node="$node"}
mysql_up{instance="$database"}
```

---

## 九、附录

### A. 指标类型说明

| 类型 | 说明 | 示例 |
|------|------|------|
| **Gauge** | 可增可减的瞬时值 | `mysql_up`, `node_cpu_usage_percent` |
| **Counter** | 只增不减的累积值 | `health_check_errors_total` |
| **Histogram** | 分布统计 | `health_check_duration_seconds` |

---

### B. PromQL 函数速查

| 函数 | 说明 | 示例 |
|------|------|------|
| `rate()` | 计算增长率 | `rate(health_check_errors_total[5m])` |
| `increase()` | 计算增量 | `increase(health_check_errors_total[1h])` |
| `avg_over_time()` | 时间范围内平均值 | `avg_over_time(mysql_up[24h])` |
| `max_over_time()` | 时间范围内最大值 | `max_over_time(node_cpu_usage_percent[1h])` |
| `min_over_time()` | 时间范围内最小值 | `min_over_time(cluster_cpu_available_percent[1h])` |
| `predict_linear()` | 线性预测 | `predict_linear(disk_space_usage_percent[1h], 3600)` |
| `topk()` | 取前 N 个最大值 | `topk(5, node_cpu_usage_percent)` |
| `bottomk()` | 取前 N 个最小值 | `bottomk(3, cluster_cpu_available_percent)` |
| `count()` | 计数 | `count(node_ready == 1)` |
| `sum()` | 求和 | `sum(disk_space_total_bytes)` |
| `avg()` | 平均值 | `avg(node_cpu_usage_percent)` |

---

### C. 时间单位

| 单位 | 说明 | 示例 |
|------|------|------|
| `s` | 秒 | `[30s]` |
| `m` | 分钟 | `[5m]` |
| `h` | 小时 | `[1h]` |
| `d` | 天 | `[7d]` |
| `w` | 周 | `[2w]` |
| `y` | 年 | `[1y]` |

---

### D. 标签匹配

| 操作符 | 说明 | 示例 |
|--------|------|------|
| `=` | 精确匹配 | `{instance="rbd-db-region"}` |
| `!=` | 不等于 | `{instance!="test"}` |
| `=~` | 正则匹配 | `{node=~"node.*"}` |
| `!~` | 正则不匹配 | `{node!~"test.*"}` |

---

**文档版本**: 1.0
**最后更新**: 2025-12-19
**维护者**: Rainbond Platform Team
