package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the health check service
type Config struct {
	// Service configuration
	MetricsPort      int
	CollectInterval  time.Duration

	// Database configurations (support multiple instances)
	Databases []DatabaseConfig

	// Registry configurations (support multiple instances)
	Registries []RegistryConfig

	// MinIO configuration
	MinIO MinIOConfig

	// Disk monitoring
	GRDataPath string

	// Node load thresholds
	NodeCPUThreshold    float64
	NodeMemoryThreshold float64

	// Kubernetes in-cluster mode
	InCluster bool
}

// DatabaseConfig represents a MySQL database configuration
type DatabaseConfig struct {
	Name     string // Instance name for metrics label
	Host     string
	Port     int
	Username string
	Password string
	Database string
}

// RegistryConfig represents a container registry configuration
type RegistryConfig struct {
	Name     string // Instance name for metrics label
	URL      string
	Username string
	Password string
	Insecure bool
}

// MinIOConfig represents MinIO/S3 configuration
type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	cfg := &Config{
		MetricsPort:         getEnvAsInt("METRICS_PORT", 9090),
		CollectInterval:     getEnvAsDuration("COLLECT_INTERVAL", 30*time.Second),
		GRDataPath:          getEnv("GRDATA_PATH", "/grdata"),
		NodeCPUThreshold:    getEnvAsFloat("NODE_CPU_THRESHOLD", 80.0),
		NodeMemoryThreshold: getEnvAsFloat("NODE_MEMORY_THRESHOLD", 80.0),
		InCluster:           getEnvAsBool("IN_CLUSTER", true),
	}

	// Load database configurations
	cfg.Databases = loadDatabaseConfigs()

	// Load registry configurations
	cfg.Registries = loadRegistryConfigs()

	// Load MinIO configuration
	cfg.MinIO = MinIOConfig{
		Endpoint:  getEnv("MINIO_ENDPOINT", ""),
		AccessKey: getEnv("MINIO_ACCESS_KEY", ""),
		SecretKey: getEnv("MINIO_SECRET_KEY", ""),
		UseSSL:    getEnvAsBool("MINIO_USE_SSL", false),
	}

	return cfg
}

// loadDatabaseConfigs loads database configurations from environment variables
// Format: DB_N_NAME, DB_N_HOST, DB_N_PORT, DB_N_USER, DB_N_PASSWORD, DB_N_DATABASE
// where N is the index (1, 2, 3, ...)
func loadDatabaseConfigs() []DatabaseConfig {
	var databases []DatabaseConfig

	// Try to load databases with index 1, 2, 3, etc.
	for i := 1; ; i++ {
		prefix := "DB_" + strconv.Itoa(i) + "_"
		name := os.Getenv(prefix + "NAME")
		host := os.Getenv(prefix + "HOST")

		// If no name or host, stop looking for more databases
		if name == "" || host == "" {
			break
		}

		databases = append(databases, DatabaseConfig{
			Name:     name,
			Host:     host,
			Port:     getEnvAsInt(prefix+"PORT", 3306),
			Username: getEnv(prefix+"USER", "root"),
			Password: getEnv(prefix+"PASSWORD", ""),
			Database: getEnv(prefix+"DATABASE", "mysql"),
		})
	}

	return databases
}

// loadRegistryConfigs loads registry configurations from environment variables
// Format: REGISTRY_N_NAME, REGISTRY_N_URL, REGISTRY_N_USER, REGISTRY_N_PASSWORD, REGISTRY_N_INSECURE
// where N is the index (1, 2, 3, ...)
func loadRegistryConfigs() []RegistryConfig {
	var registries []RegistryConfig

	for i := 1; ; i++ {
		prefix := "REGISTRY_" + strconv.Itoa(i) + "_"
		name := os.Getenv(prefix + "NAME")
		url := os.Getenv(prefix + "URL")

		// If no name or URL, stop looking for more registries
		if name == "" || url == "" {
			break
		}

		registries = append(registries, RegistryConfig{
			Name:     name,
			URL:      url,
			Username: getEnv(prefix+"USER", ""),
			Password: getEnv(prefix+"PASSWORD", ""),
			Insecure: getEnvAsBool(prefix+"INSECURE", false),
		})
	}

	return registries
}

// Helper functions to get environment variables with defaults

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(strings.ToLower(value)); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
