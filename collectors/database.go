package collectors

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/rainbond/health-console/config"
	"github.com/rainbond/health-console/metrics"
)

// DatabaseCollector monitors database health
type DatabaseCollector struct {
	databases []config.DatabaseConfig
	interval  time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewDatabaseCollector creates a new database collector
func NewDatabaseCollector(cfg *config.Config) *DatabaseCollector {
	ctx, cancel := context.WithCancel(context.Background())
	return &DatabaseCollector{
		databases: cfg.Databases,
		interval:  cfg.CollectInterval,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins collecting database metrics
func (c *DatabaseCollector) Start() {
	log.Println("Starting database collector...")

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
func (c *DatabaseCollector) Stop() {
	log.Println("Stopping database collector...")
	c.cancel()
}

// collect performs database health checks
func (c *DatabaseCollector) collect() {
	for _, db := range c.databases {
		go c.checkDatabase(db)
	}
}

// checkDatabase checks a single database instance
func (c *DatabaseCollector) checkDatabase(dbConfig config.DatabaseConfig) {
	start := time.Now()
	defer func() {
		metrics.HealthCheckDuration.WithLabelValues("database").Observe(time.Since(start).Seconds())
	}()

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=5s",
		dbConfig.Username,
		dbConfig.Password,
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		errorReason := classifyDatabaseError(err)
		log.Printf("Failed to open database connection for %s (%s:%d): %v [reason: %s]", dbConfig.Name, dbConfig.Host, dbConfig.Port, err, errorReason)
		metrics.MySQLUp.WithLabelValues(dbConfig.Name, dbConfig.Host, fmt.Sprintf("%d", dbConfig.Port), errorReason).Set(0)
		metrics.HealthCheckErrors.WithLabelValues("database", "connection_failed").Inc()
		return
	}
	defer db.Close()

	// Set connection timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ping the database
	if err := db.PingContext(ctx); err != nil {
		errorReason := classifyDatabaseError(err)
		log.Printf("Database %s (%s:%d) is unreachable: %v [reason: %s]", dbConfig.Name, dbConfig.Host, dbConfig.Port, err, errorReason)
		metrics.MySQLUp.WithLabelValues(dbConfig.Name, dbConfig.Host, fmt.Sprintf("%d", dbConfig.Port), errorReason).Set(0)
		metrics.HealthCheckErrors.WithLabelValues("database", "ping_failed").Inc()
		return
	}

	log.Printf("Database %s (%s:%d) is healthy", dbConfig.Name, dbConfig.Host, dbConfig.Port)
	metrics.MySQLUp.WithLabelValues(dbConfig.Name, dbConfig.Host, fmt.Sprintf("%d", dbConfig.Port), "正常").Set(1)
}

// classifyDatabaseError classifies database errors for better troubleshooting
func classifyDatabaseError(err error) string {
	if err == nil {
		return "正常"
	}

	errMsg := strings.ToLower(err.Error())

	// Authentication errors
	if strings.Contains(errMsg, "access denied") || strings.Contains(errMsg, "authentication") {
		return "认证失败"
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

	// Too many connections
	if strings.Contains(errMsg, "too many connections") {
		return "连接数过多"
	}

	// Database does not exist
	if strings.Contains(errMsg, "unknown database") {
		return "数据库不存在"
	}

	// SSL/TLS errors
	if strings.Contains(errMsg, "tls") || strings.Contains(errMsg, "ssl") || strings.Contains(errMsg, "certificate") {
		return "TLS证书错误"
	}

	// Generic connection error
	if strings.Contains(errMsg, "connection") {
		return "连接错误"
	}

	// Unknown error
	return "未知错误"
}
