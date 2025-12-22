package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// P0 - Critical infrastructure metrics

// MySQLUp indicates if MySQL database is reachable (1=up, 0=down)
var MySQLUp = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "mysql_up",
		Help: "MySQL database availability (1=up, 0=down)",
	},
	[]string{"instance", "host", "port", "error_reason"},
)

// KubernetesAPIServerUp indicates if Kubernetes API Server is reachable
var KubernetesAPIServerUp = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "kubernetes_apiserver_up",
		Help: "Kubernetes API Server availability (1=up, 0=down)",
	},
	[]string{"error_reason"},
)

// CoreDNSUp indicates if CoreDNS is working properly
var CoreDNSUp = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "coredns_up",
		Help: "CoreDNS availability (1=up, 0=down)",
	},
	[]string{"error_reason"},
)

// EtcdUp indicates if Etcd cluster is available
var EtcdUp = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "etcd_up",
		Help: "Etcd cluster availability (1=up, 0=down)",
	},
	[]string{"error_reason"},
)

// ClusterStorageUp indicates if cluster storage is available
var ClusterStorageUp = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "cluster_storage_up",
		Help: "Cluster storage class availability (1=up, 0=down)",
	},
	[]string{"storage_class", "error_reason"},
)

// RegistryUp indicates if container registry is reachable
var RegistryUp = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "registry_up",
		Help: "Container registry availability (1=up, 0=down)",
	},
	[]string{"instance", "url", "error_reason"},
)

// MinIOUp indicates if MinIO/S3 is reachable
var MinIOUp = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "minio_up",
		Help: "MinIO/S3 availability (1=up, 0=down)",
	},
	[]string{"error_reason"},
)

// HealthCheckErrors tracks errors during health checks
var HealthCheckErrors = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "health_check_errors_total",
		Help: "Total number of health check errors",
	},
	[]string{"collector", "error_type"},
)

// HealthCheckDuration tracks duration of health checks
var HealthCheckDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "health_check_duration_seconds",
		Help:    "Duration of health checks in seconds",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"collector"},
)
