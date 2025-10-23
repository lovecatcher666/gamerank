package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"game-leaderboard/pkg/logger"
)

type Config struct {
	// 服务器配置
	Environment string `json:"environment"`
	Port        string `json:"port"`
	LogLevel    string `json:"logLevel"`

	// MySQL 配置
	MySQLDSN       string `json:"mysqlDSN"`
	MySQLMaxConns  int    `json:"mysqlMaxConns"`
	MySQLIdleConns int    `json:"mysqlIdleConns"`

	// Redis 配置
	RedisAddr     string `json:"redisAddr"`
	RedisPassword string `json:"redisPassword"`
	RedisDB       int    `json:"redisDB"`
	RedisPoolSize int    `json:"redisPoolSize"`

	// 排行榜配置
	RankingMethod  string `json:"rankingMethod"`
	EnableCache    bool   `json:"enableCache"`
	CacheSize      int    `json:"cacheSize"`
	ShardCount     int    `json:"shardCount"`
	RebuildOnStart bool   `json:"rebuildOnStart"`

	// 性能配置
	SnapshotInterval time.Duration `json:"snapshotInterval"`
	WriteTimeout     time.Duration `json:"writeTimeout"`
	ReadTimeout      time.Duration `json:"readTimeout"`

	// 监控配置
	MetricsEnabled bool   `json:"metricsEnabled"`
	MetricsPort    string `json:"metricsPort"`
}

// LoadConfig 从环境变量加载配置
func LoadConfig() *Config {
	cfg := &Config{
		// 服务器配置
		Environment: getEnv("ENVIRONMENT", "development"),
		Port:        getEnv("PORT", "8080"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),

		// MySQL 配置
		MySQLDSN:       getEnv("MYSQL_DSN", "root:root@tcp(localhost:3306)/360?parseTime=true"),
		MySQLMaxConns:  getEnvAsInt("MYSQL_MAX_CONNS", 100),
		MySQLIdleConns: getEnvAsInt("MYSQL_IDLE_CONNS", 10),

		// Redis 配置
		RedisAddr:     getEnv("REDIS_ADDR", "127.0.0.1:11307"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvAsInt("REDIS_DB", 0),
		RedisPoolSize: getEnvAsInt("REDIS_POOL_SIZE", 100),

		// 排行榜配置
		RankingMethod:  getEnv("RANKING_METHOD", "standard"), // standard or dense
		EnableCache:    getEnvAsBool("ENABLE_CACHE", true),
		CacheSize:      getEnvAsInt("CACHE_SIZE", 10000),
		ShardCount:     getEnvAsInt("SHARD_COUNT", 16),
		RebuildOnStart: getEnvAsBool("REBUILD_ON_START", false),

		// 性能配置
		SnapshotInterval: getEnvAsDuration("SNAPSHOT_INTERVAL", 1*time.Hour),
		WriteTimeout:     getEnvAsDuration("WRITE_TIMEOUT", 10*time.Second),
		ReadTimeout:      getEnvAsDuration("READ_TIMEOUT", 5*time.Second),

		// 监控配置
		MetricsEnabled: getEnvAsBool("METRICS_ENABLED", false),
		MetricsPort:    getEnv("METRICS_PORT", "9090"),
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		logger.NewLogger("config").Warn("Configuration validation warning", "error", err)
	}

	return cfg
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("PORT is required")
	}

	if c.MySQLDSN == "" {
		return fmt.Errorf("MYSQL_DSN is required")
	}

	if c.RedisAddr == "" {
		return fmt.Errorf("REDIS_ADDR is required")
	}

	if c.RankingMethod != "standard" && c.RankingMethod != "dense" {
		return fmt.Errorf("RANKING_METHOD must be 'standard' or 'dense'")
	}

	if c.CacheSize <= 0 {
		return fmt.Errorf("CACHE_SIZE must be positive")
	}

	if c.ShardCount <= 0 {
		return fmt.Errorf("SHARD_COUNT must be positive")
	}

	return nil
}

// IsProduction 检查是否为生产环境
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// IsDevelopment 检查是否为开发环境
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// 辅助函数
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		logger.NewLogger("config").Warn(
			"Failed to parse environment variable as integer, using default",
			"key", key,
			"value", valueStr,
			"default", defaultValue,
			"error", err,
		)
		return defaultValue
	}

	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		logger.NewLogger("config").Warn(
			"Failed to parse environment variable as boolean, using default",
			"key", key,
			"value", valueStr,
			"default", defaultValue,
			"error", err,
		)
		return defaultValue
	}

	return value
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := time.ParseDuration(valueStr)
	if err != nil {
		logger.NewLogger("config").Warn(
			"Failed to parse environment variable as duration, using default",
			"key", key,
			"value", valueStr,
			"default", defaultValue,
			"error", err,
		)
		return defaultValue
	}

	return value
}
