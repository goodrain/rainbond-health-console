package collectors

import (
	"context"
	"database/sql"
	"fmt"
	"log"
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
		log.Printf("Failed to open database connection for %s (%s:%d): %v", dbConfig.Name, dbConfig.Host, dbConfig.Port, err)
		metrics.MySQLUp.WithLabelValues(dbConfig.Name, dbConfig.Host, fmt.Sprintf("%d", dbConfig.Port)).Set(0)
		metrics.HealthCheckErrors.WithLabelValues("database", "connection_failed").Inc()
		return
	}
	defer db.Close()

	// Set connection timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ping the database
	if err := db.PingContext(ctx); err != nil {
		log.Printf("Database %s (%s:%d) is unreachable: %v", dbConfig.Name, dbConfig.Host, dbConfig.Port, err)
		metrics.MySQLUp.WithLabelValues(dbConfig.Name, dbConfig.Host, fmt.Sprintf("%d", dbConfig.Port)).Set(0)
		metrics.HealthCheckErrors.WithLabelValues("database", "ping_failed").Inc()
		return
	}

	log.Printf("Database %s (%s:%d) is healthy", dbConfig.Name, dbConfig.Host, dbConfig.Port)
	metrics.MySQLUp.WithLabelValues(dbConfig.Name, dbConfig.Host, fmt.Sprintf("%d", dbConfig.Port)).Set(1)
}
