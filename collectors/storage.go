package collectors

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/rainbond/health-console/config"
	"github.com/rainbond/health-console/metrics"
)

// StorageCollector monitors MinIO/S3 storage health
type StorageCollector struct {
	minioConfig config.MinIOConfig
	interval    time.Duration
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewStorageCollector creates a new storage collector
func NewStorageCollector(cfg *config.Config) *StorageCollector {
	ctx, cancel := context.WithCancel(context.Background())
	return &StorageCollector{
		minioConfig: cfg.MinIO,
		interval:    cfg.CollectInterval,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start begins collecting storage metrics
func (c *StorageCollector) Start() {
	// Skip if MinIO is not configured
	if c.minioConfig.Endpoint == "" {
		log.Println("MinIO not configured, skipping storage collector...")
		return
	}

	log.Println("Starting storage collector...")

	// Initial check
	c.collect()

	// Periodic checks
	ticker := time.NewTicker(c.interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				c.collect()
			case <-c.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

// Stop stops the collector
func (c *StorageCollector) Stop() {
	log.Println("Stopping storage collector...")
	c.cancel()
}

// collect performs storage health check
func (c *StorageCollector) collect() {
	go c.checkMinIO()
}

// checkMinIO checks MinIO/S3 health
func (c *StorageCollector) checkMinIO() {
	start := time.Now()
	defer func() {
		metrics.HealthCheckDuration.WithLabelValues("minio").Observe(time.Since(start).Seconds())
	}()

	// Create MinIO client
	minioClient, err := minio.New(c.minioConfig.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.minioConfig.AccessKey, c.minioConfig.SecretKey, ""),
		Secure: c.minioConfig.UseSSL,
	})
	if err != nil {
		errorReason := classifyMinIOError(err)
		log.Printf("Failed to create MinIO client: %v [reason: %s]", err, errorReason)
		metrics.MinIOUp.WithLabelValues().Set(0)
		metrics.HealthCheckErrors.WithLabelValues("minio", "client_creation_failed").Inc()
		return
	}

	// Set timeout for health check
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if MinIO is online by listing buckets
	_, err = minioClient.ListBuckets(ctx)
	if err != nil {
		errorReason := classifyMinIOError(err)
		log.Printf("MinIO is unreachable: %v [reason: %s]", err, errorReason)
		metrics.MinIOUp.WithLabelValues().Set(0)
		metrics.HealthCheckErrors.WithLabelValues("minio", "unreachable").Inc()
		return
	}

	log.Printf("MinIO is healthy")
	metrics.MinIOUp.WithLabelValues().Set(1)
}

// classifyMinIOError classifies MinIO/S3 errors for better troubleshooting
func classifyMinIOError(err error) string {
	if err == nil {
		return "正常"
	}

	errMsg := strings.ToLower(err.Error())

	// Authentication errors
	if strings.Contains(errMsg, "access denied") || strings.Contains(errMsg, "invalid access key") {
		return "认证失败"
	}
	if strings.Contains(errMsg, "signature does not match") {
		return "签名不匹配"
	}

	// Connection timeout
	if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline exceeded") {
		return "连接超时"
	}

	// Network errors
	if strings.Contains(errMsg, "connection refused") {
		return "连接被拒绝"
	}
	if strings.Contains(errMsg, "no route to host") || strings.Contains(errMsg, "network unreachable") {
		return "网络不可达"
	}
	if strings.Contains(errMsg, "connection reset") {
		return "连接被重置"
	}

	// DNS resolution errors
	if strings.Contains(errMsg, "no such host") || strings.Contains(errMsg, "could not resolve") {
		return "DNS解析失败"
	}

	// TLS/Certificate errors
	if strings.Contains(errMsg, "certificate") || strings.Contains(errMsg, "tls") || strings.Contains(errMsg, "x509") {
		return "TLS证书错误"
	}

	// Bucket errors
	if strings.Contains(errMsg, "bucket") {
		return "存储桶错误"
	}

	// Generic connection error
	if strings.Contains(errMsg, "connection") {
		return "连接错误"
	}

	// Unknown error
	return "未知错误"
}
