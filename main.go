package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/rainbond/health-console/collectors"
	"github.com/rainbond/health-console/config"
)

func main() {
	log.Println("Starting Rainbond Health Console...")

	// Load configuration
	cfg := config.LoadConfig()
	log.Printf("Configuration loaded:")
	log.Printf("  - Metrics Port: %d", cfg.MetricsPort)
	log.Printf("  - Collect Interval: %s", cfg.CollectInterval)
	log.Printf("  - GRData Path: %s", cfg.GRDataPath)
	log.Printf("  - Database Instances: %d", len(cfg.Databases))
	log.Printf("  - Registry Instances: %d", len(cfg.Registries))

	// Initialize collectors
	var collectorList []interface{ Stop() }

	// Database collector
	if len(cfg.Databases) > 0 {
		dbCollector := collectors.NewDatabaseCollector(cfg)
		dbCollector.Start()
		collectorList = append(collectorList, dbCollector)
	} else {
		log.Println("No database instances configured, skipping database collector")
	}

	// Kubernetes collector
	k8sCollector, err := collectors.NewKubernetesCollector(cfg)
	if err != nil {
		log.Printf("Warning: Failed to initialize Kubernetes collector: %v", err)
	} else {
		k8sCollector.Start()
		collectorList = append(collectorList, k8sCollector)
	}

	// Registry collector
	if len(cfg.Registries) > 0 {
		registryCollector := collectors.NewRegistryCollector(cfg)
		registryCollector.Start()
		collectorList = append(collectorList, registryCollector)
	} else {
		log.Println("No registry instances configured, skipping registry collector")
	}

	// Storage (MinIO) collector
	storageCollector := collectors.NewStorageCollector(cfg)
	storageCollector.Start()
	collectorList = append(collectorList, storageCollector)

	// Setup HTTP server for metrics
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/", indexHandler)

	// Start HTTP server in a goroutine
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.MetricsPort),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Starting HTTP server on port %d", cfg.MetricsPort)
		log.Printf("Metrics available at http://localhost:%d/metrics", cfg.MetricsPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")

	// Stop all collectors
	for _, collector := range collectorList {
		collector.Stop()
	}

	// Shutdown HTTP server
	if err := server.Close(); err != nil {
		log.Printf("Error closing HTTP server: %v", err)
	}

	log.Println("Shutdown complete")
}

// healthHandler handles health check requests
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// indexHandler provides information about available endpoints
func indexHandler(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Rainbond Health Console</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        h1 { color: #333; }
        .endpoint { margin: 20px 0; padding: 10px; background: #f5f5f5; border-radius: 5px; }
        .endpoint a { color: #0066cc; text-decoration: none; }
        .endpoint a:hover { text-decoration: underline; }
        .description { color: #666; margin-top: 5px; }
    </style>
</head>
<body>
    <h1>Rainbond Health Console</h1>
    <p>Platform stability monitoring service for Rainbond</p>

    <div class="endpoint">
        <a href="/metrics">/metrics</a>
        <div class="description">Prometheus metrics endpoint</div>
    </div>

    <div class="endpoint">
        <a href="/health">/health</a>
        <div class="description">Health check endpoint</div>
    </div>

    <h2>Monitored Components</h2>
    <ul>
        <li>Database connectivity (MySQL)</li>
        <li>Kubernetes cluster (API Server, CoreDNS, Etcd, Storage)</li>
        <li>Container registry</li>
        <li>Object storage (MinIO/S3)</li>
    </ul>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
