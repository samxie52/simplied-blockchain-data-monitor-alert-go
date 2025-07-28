package config

import (
	"fmt"
	"os"
	"time"
)

// Loader 配置加载器
type Loader struct {
	// 环境变量加载器
	envLoader *EnvLoader
	// 配置验证器
	validator *Validator
}

// NewLoader 创建配置加载器
func NewLoader(envFile string) *Loader {
	return &Loader{
		envLoader: NewEnvLoader(envFile),
		validator: NewValidator(),
	}
}

// Load 加载配置
func (l *Loader) Load() (*Config, error) {
	cfg := &Config{}

	// 设置默认值
	l.setDefaults(cfg)

	// 加载环境变量
	if err := l.envLoader.Load(cfg); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// 验证配置
	if err := l.validator.Validate(cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// setDefaults 设置默认配置值
func (l *Loader) setDefaults(cfg *Config) {
	// 应用程序默认配置
	cfg.App.Name = "blockchain-monitor"
	cfg.App.Version = "v1.0.0"
	cfg.App.Environment = "development"
	cfg.App.Port = 8080
	cfg.App.Host = "localhost"
	cfg.App.Debug = false

	// 数据库默认配置
	cfg.Database.Host = "localhost"
	cfg.Database.Port = 5432
	cfg.Database.SSLMode = "disable"
	cfg.Database.MaxOpenConns = 25
	cfg.Database.MaxIdleConns = 5
	cfg.Database.ConnMaxLifetime = 300 * time.Second

	// Redis默认配置
	cfg.Redis.Host = "localhost"
	cfg.Redis.Port = 6379
	cfg.Redis.DB = 0
	cfg.Redis.PoolSize = 10

	// InfluxDB默认配置
	cfg.InfluxDB.URL = "http://localhost:8686"
	cfg.InfluxDB.Token = "your-influxdb-token"
	cfg.InfluxDB.Org = "your-influxdb-org"
	cfg.InfluxDB.Bucket = "your-influxdb-bucket"

	// 以太坊默认配置
	cfg.Ethereum.RPCURL = "https://mainnet.infura.io/v3/your-project-id"
	cfg.Ethereum.HTTPURL = "https://mainnet.infura.io/v3/your-project-id"
	cfg.Ethereum.Network = "mainnet"
	cfg.Ethereum.ChainID = 1
	cfg.Ethereum.Timeout = 30 * time.Second

	// Telegram默认配置
	cfg.Telegram.BotToken = "your-telegram-bot-token"
	cfg.Telegram.WebhookURL = "https://your-telegram-webhook-url"
	cfg.Telegram.Timeout = 30 * time.Second

	// 监控默认配置
	cfg.Monitor.PrometheusPort = 9090
	cfg.Monitor.MetricsPath = "/metrics"

	// 日志默认配置
	cfg.Logging.Level = "info"
	cfg.Logging.Format = "json"
	cfg.Logging.Output = "stdout"

	// 限流默认配置
	cfg.RateLimit.Requests = 100
	cfg.RateLimit.Window = 60 * time.Second

	// 告警默认配置
	cfg.Alert.Cooldown = 300 * time.Second
	cfg.Alert.MaxPerHour = 10
	cfg.Alert.RetryAttempts = 3
	cfg.Alert.RetryInterval = 30 * time.Second

	// 工作进程默认配置
	cfg.Worker.PoolSize = 10
	cfg.Worker.QueueSize = 1000
	cfg.Worker.Timeout = 30 * time.Second
	cfg.Worker.BatchSize = 100
}

// MustLoad 加载配置，失败时panic
func (l *Loader) MustLoad() *Config {
	cfg, err := l.Load()
	if err != nil {
		//panic() 用于在配置加载失败时停止程序
		panic(fmt.Sprintf("Failed to load configuration: %v", err))
	}
	return cfg
}

// LoadFromFile 从指定文件加载配置
func (l *Loader) LoadFromFile(envFile string) (*Config, error) {
	loader := NewLoader(envFile)
	return loader.Load()
}

// LoadFromEnv 从环境变量加载配置
func (l *Loader) LoadFromEnv() (*Config, error) {
	loader := NewLoader("")
	return loader.Load()
}

// MustLoadFromFile 从文件加载配置，失败时panic
func (l *Loader) MustLoadFromFile(envFile string) *Config {
	cfg, err := l.LoadFromFile(envFile)
	if err != nil {
		panic(err)
	}
	return cfg
}

// GetEnvFile 获取环境变量文件路径
func (l *Loader) GetEnvFile() string {
	envFile := os.Getenv("ENV_FILE")
	if envFile == "" {
		envFile = ".env"
	}
	return envFile
}
