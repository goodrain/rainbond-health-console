package collectors

import (
	"context"
	"log"
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
		log.Printf("Failed to create MinIO client: %v", err)
		metrics.MinIOUp.Set(0)
		metrics.HealthCheckErrors.WithLabelValues("minio", "client_creation_failed").Inc()
		return
	}

	// Set timeout for health check
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if MinIO is online by listing buckets
	_, err = minioClient.ListBuckets(ctx)
	if err != nil {
		log.Printf("MinIO is unreachable: %v", err)
		metrics.MinIOUp.Set(0)
		metrics.HealthCheckErrors.WithLabelValues("minio", "unreachable").Inc()
		return
	}

	log.Printf("MinIO is healthy")
	metrics.MinIOUp.Set(1)
}
