package collectors

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/rainbond/health-console/config"
	"github.com/rainbond/health-console/metrics"
)

// RegistryCollector monitors container registry health
type RegistryCollector struct {
	registries []config.RegistryConfig
	interval   time.Duration
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewRegistryCollector creates a new registry collector
func NewRegistryCollector(cfg *config.Config) *RegistryCollector {
	ctx, cancel := context.WithCancel(context.Background())
	return &RegistryCollector{
		registries: cfg.Registries,
		interval:   cfg.CollectInterval,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins collecting registry metrics
func (c *RegistryCollector) Start() {
	log.Println("Starting registry collector...")

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
func (c *RegistryCollector) Stop() {
	log.Println("Stopping registry collector...")
	c.cancel()
}

// collect performs registry health checks
func (c *RegistryCollector) collect() {
	for _, registry := range c.registries {
		go c.checkRegistry(registry)
	}
}

// checkRegistry checks a single registry instance
func (c *RegistryCollector) checkRegistry(regConfig config.RegistryConfig) {
	start := time.Now()
	defer func() {
		metrics.HealthCheckDuration.WithLabelValues("registry").Observe(time.Since(start).Seconds())
	}()

	// Normalize URL
	url := regConfig.URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		if regConfig.Insecure {
			url = "http://" + url
		} else {
			url = "https://" + url
		}
	}

	// Ensure URL ends with /v2/ for Docker Registry API
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	if !strings.HasSuffix(url, "/v2/") {
		url += "v2/"
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Allow insecure TLS if configured
	if regConfig.Insecure {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	// Create request
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		log.Printf("Failed to create request for registry %s (%s): %v", regConfig.Name, regConfig.URL, err)
		metrics.RegistryUp.WithLabelValues(regConfig.Name, regConfig.URL).Set(0)
		metrics.HealthCheckErrors.WithLabelValues("registry", "request_failed").Inc()
		return
	}

	// Add authentication if provided
	if regConfig.Username != "" && regConfig.Password != "" {
		req.SetBasicAuth(regConfig.Username, regConfig.Password)
	}

	// Perform request
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Registry %s (%s) is unreachable: %v", regConfig.Name, regConfig.URL, err)
		metrics.RegistryUp.WithLabelValues(regConfig.Name, regConfig.URL).Set(0)
		metrics.HealthCheckErrors.WithLabelValues("registry", "unreachable").Inc()
		return
	}
	defer resp.Body.Close()

	// Check response status
	// Docker Registry API v2 should return 200 or 401 (authentication required)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
		log.Printf("Registry %s (%s) is healthy", regConfig.Name, regConfig.URL)
		metrics.RegistryUp.WithLabelValues(regConfig.Name, regConfig.URL).Set(1)
	} else {
		log.Printf("Registry %s (%s) returned unexpected status: %d", regConfig.Name, regConfig.URL, resp.StatusCode)
		metrics.RegistryUp.WithLabelValues(regConfig.Name, regConfig.URL).Set(0)
		metrics.HealthCheckErrors.WithLabelValues("registry", fmt.Sprintf("status_%d", resp.StatusCode)).Inc()
	}
}
