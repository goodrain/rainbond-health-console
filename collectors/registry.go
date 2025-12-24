package collectors

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rainbond/health-console/config"
	"github.com/rainbond/health-console/metrics"
)

// RegistryCollector monitors container registry health
type RegistryCollector struct {
	registries       []config.RegistryConfig
	interval         time.Duration
	ctx              context.Context
	cancel           context.CancelFunc
	lastErrorReasons map[string]string // 跟踪每个 registry 上一次的 error_reason
	mu               sync.Mutex        // 保护 lastErrorReasons 的并发访问
}

// NewRegistryCollector creates a new registry collector
func NewRegistryCollector(cfg *config.Config) *RegistryCollector {
	ctx, cancel := context.WithCancel(context.Background())
	return &RegistryCollector{
		registries:       cfg.Registries,
		interval:         cfg.CollectInterval,
		ctx:              ctx,
		cancel:           cancel,
		lastErrorReasons: make(map[string]string),
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

// setRegistryMetric 更新 registry_up 指标，并删除旧的 error_reason 标签组合
func (c *RegistryCollector) setRegistryMetric(name, url, errorReason string, value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 生成唯一的 key
	key := name + "|" + url

	// 如果之前有不同的 error_reason，删除旧的时间序列
	if lastReason, exists := c.lastErrorReasons[key]; exists && lastReason != errorReason {
		metrics.RegistryUp.DeleteLabelValues(name, url, lastReason)
		log.Printf("Deleted old metric: registry_up{instance=%s, url=%s, error_reason=%s}", name, url, lastReason)
	}

	// 设置新值
	metrics.RegistryUp.WithLabelValues(name, url, errorReason).Set(value)

	// 更新跟踪记录
	c.lastErrorReasons[key] = errorReason
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
		errorReason := "请求创建失败"
		log.Printf("Failed to create request for registry %s (%s): %v [reason: %s]", regConfig.Name, regConfig.URL, err, errorReason)
		c.setRegistryMetric(regConfig.Name, regConfig.URL, errorReason, 0)
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
		errorReason := classifyRegistryError(err)
		log.Printf("Registry %s (%s) is unreachable: %v [reason: %s]", regConfig.Name, regConfig.URL, err, errorReason)
		c.setRegistryMetric(regConfig.Name, regConfig.URL, errorReason, 0)
		metrics.HealthCheckErrors.WithLabelValues("registry", "unreachable").Inc()
		return
	}
	defer resp.Body.Close()

	// Check response status
	// Docker Registry API v2 should return 200 or 401 (authentication required)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
		log.Printf("Registry %s (%s) is healthy", regConfig.Name, regConfig.URL)
		c.setRegistryMetric(regConfig.Name, regConfig.URL, "正常", 1)
	} else {
		errorReason := classifyHTTPStatus(resp.StatusCode)
		log.Printf("Registry %s (%s) returned unexpected status: %d [reason: %s]", regConfig.Name, regConfig.URL, resp.StatusCode, errorReason)
		c.setRegistryMetric(regConfig.Name, regConfig.URL, errorReason, 0)
		metrics.HealthCheckErrors.WithLabelValues("registry", fmt.Sprintf("status_%d", resp.StatusCode)).Inc()
	}
}

// classifyRegistryError classifies registry connection errors for better troubleshooting
func classifyRegistryError(err error) string {
	if err == nil {
		return "正常"
	}

	errMsg := strings.ToLower(err.Error())

	// TLS/Certificate errors
	if strings.Contains(errMsg, "certificate signed by unknown authority") {
		return "证书由未知CA签发"
	}
	if strings.Contains(errMsg, "certificate has expired") || strings.Contains(errMsg, "certificate is not valid") {
		return "证书已过期"
	}
	if strings.Contains(errMsg, "certificate") || strings.Contains(errMsg, "tls") || strings.Contains(errMsg, "x509") {
		return "TLS证书错误"
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

	// SSL protocol errors
	if strings.Contains(errMsg, "ssl") {
		return "SSL协议错误"
	}

	// EOF errors (connection closed)
	if strings.Contains(errMsg, "eof") {
		return "连接意外关闭"
	}

	// Generic connection error
	if strings.Contains(errMsg, "connection") {
		return "连接错误"
	}

	// Unknown error
	return "未知错误"
}

// classifyHTTPStatus classifies HTTP status codes for better troubleshooting
func classifyHTTPStatus(statusCode int) string {
	switch statusCode {
	// 2xx Success (should not reach here in error path)
	case 200:
		return "正常"
	case 201:
		return "已创建"
	case 204:
		return "无内容"

	// 3xx Redirection
	case 301, 302, 303, 307, 308:
		return "重定向错误"

	// 4xx Client Errors
	case 400:
		return "请求格式错误"
	case 401:
		return "需要认证" // Should not reach here as 401 is considered healthy
	case 403:
		return "访问被禁止"
	case 404:
		return "服务不存在"
	case 405:
		return "请求方法不允许"
	case 408:
		return "请求超时"
	case 429:
		return "请求过于频繁"

	// 5xx Server Errors
	case 500:
		return "服务内部错误"
	case 501:
		return "功能未实现"
	case 502:
		return "后端服务不可达"
	case 503:
		return "服务不可用"
	case 504:
		return "后端服务超时"
	case 505:
		return "HTTP版本不支持"

	// Unknown status code
	default:
		if statusCode >= 400 && statusCode < 500 {
			return fmt.Sprintf("客户端错误%d", statusCode)
		} else if statusCode >= 500 && statusCode < 600 {
			return fmt.Sprintf("服务端错误%d", statusCode)
		}
		return fmt.Sprintf("未知状态码%d", statusCode)
	}
}
