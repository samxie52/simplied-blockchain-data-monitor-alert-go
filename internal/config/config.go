package config

import (
	"fmt"
	"time"
)

// Config 应用程序配置结构
type Config struct {
	// 应用程序配置
	App AppConfig `json:"app"`
	// 数据库配置
	Database DatabaseConfig `json:"database"`
	// Redis配置
	Redis RedisConfig `json:"redis"`
	// InfluxDB配置
	InfluxDB InfluxDBConfig `json:"influxdb"`
	// 以太坊配置
	Ethereum EthereumConfig `json:"ethereum"`
	// Telegram配置
	Telegram TelegramConfig `json:"telegram"`
	// 监控配置
	Monitor MonitorConfig `json:"monitor"`
	// 日志配置
	Logging LoggingConfig `json:"logging"`
	// 安全配置
	Security SecurityConfig `json:"security"`
	// 限流配置
	RateLimit RateLimitConfig `json:"rate_limit"`
	// 告警配置
	Alert AlertConfig `json:"alert"`
	// 工作进程配置
	Worker WorkerConfig `json:"worker"`
}

// AppConfig 应用程序基础配置
type AppConfig struct {
	// 应用程序名称
	Name string `json:"name" env:"APP_NAME" validate:"required"`
	// 应用程序版本
	Version string `json:"version" env:"APP_VERSION" validate:"required"`
	// 应用程序运行环境
	Environment string `json:"environment" env:"APP_ENV" validate:"required,oneof=development staging production"`
	// 应用程序监听端口
	Port int `json:"port" env:"APP_PORT" validate:"required,min=1,max=65535"`
	// 应用程序监听主机
	Host string `json:"host" env:"APP_HOST" validate:"required"`
	// 是否启用调试模式
	Debug bool `json:"debug" env:"APP_DEBUG"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	// 数据库主机
	Host string `json:"host" env:"DB_HOST" validate:"required"`
	// 数据库端口
	Port int `json:"port" env:"DB_PORT" validate:"required,min=1,max=65535"`
	// 数据库名称
	Name string `json:"name" env:"DB_NAME" validate:"required"`
	// 数据库用户
	User string `json:"user" env:"DB_USER" validate:"required"`
	// 数据库密码
	Password string `json:"password" env:"DB_PASSWORD" validate:"required"`
	// 数据库SSL模式
	SSLMode string `json:"ssl_mode" env:"DB_SSL_MODE" validate:"oneof=disable require verify-ca verify-full"`
	// 数据库最大打开连接数
	MaxOpenConns int `json:"max_open_conns" env:"DB_MAX_OPEN_CONNS" validate:"min=1"`
	// 数据库最大空闲连接数
	MaxIdleConns int `json:"max_idle_conns" env:"DB_MAX_IDLE_CONNS" validate:"min=1"`
	// 数据库连接最大生命周期
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	// Redis主机
	Host string `json:"host" env:"REDIS_HOST" validate:"required"`
	// Redis端口
	Port int `json:"port" env:"REDIS_PORT" validate:"required,min=1,max=65535"`
	// Redis密码
	Password string `json:"password" env:"REDIS_PASSWORD"`
	// Redis数据库
	DB int `json:"db" env:"REDIS_DB" validate:"min=0,max=15"`
	// Redis连接池大小
	PoolSize int `json:"pool_size" env:"REDIS_POOL_SIZE" validate:"min=1"`
}

// InfluxDBConfig InfluxDB配置
type InfluxDBConfig struct {
	// InfluxDB URL
	URL string `json:"url" env:"INFLUX_URL" validate:"required,url"`
	// InfluxDB Token
	Token string `json:"token" env:"INFLUX_TOKEN" validate:"required"`
	// InfluxDB组织
	Org string `json:"org" env:"INFLUX_ORG" validate:"required"`
	// InfluxDB存储桶
	Bucket string `json:"bucket" env:"INFLUX_BUCKET" validate:"required"`
}

// EthereumConfig 以太坊配置
type EthereumConfig struct {
	// 以太坊RPC URL
	RPCURL string `json:"rpc_url" env:"ETH_RPC_URL" validate:"required"`
	// 以太坊HTTP URL
	HTTPURL string `json:"http_url" env:"ETH_HTTP_URL" validate:"required,url"`
	// 以太坊网络
	Network string `json:"network" env:"ETH_NETWORK" validate:"required,oneof=mainnet goerli sepolia"`
	// 以太坊链ID
	ChainID int64 `json:"chain_id" env:"ETH_CHAIN_ID" validate:"required"`
	// 以太坊超时时间
	Timeout time.Duration `json:"timeout" env:"ETH_TIMEOUT"`
}

// TelegramConfig Telegram Bot配置
type TelegramConfig struct {
	// Telegram Bot Token
	BotToken string `json:"bot_token" env:"TELEGRAM_BOT_TOKEN" validate:"required"`
	// Telegram Webhook URL
	WebhookURL string `json:"webhook_url" env:"TELEGRAM_WEBHOOK_URL" validate:"url"`
	// Telegram超时时间
	Timeout time.Duration `json:"timeout" env:"TELEGRAM_TIMEOUT"`
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	// Prometheus端口
	PrometheusPort int `json:"prometheus_port" env:"PROMETHEUS_PORT" validate:"min=1,max=65535"`
	// Prometheus指标路径
	MetricsPath string `json:"metrics_path" env:"METRICS_PATH" validate:"required"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	// 日志级别
	Level string `json:"level" env:"LOG_LEVEL" validate:"required,oneof=debug info warn error fatal panic"`
	// 日志格式
	Format string `json:"format" env:"LOG_FORMAT" validate:"required,oneof=json text"`
	// 日志输出
	Output string `json:"output" env:"LOG_OUTPUT" validate:"required,oneof=stdout stderr file"`
	// 日志文件路径
	FilePath string `json:"file_path" env:"LOG_FILE_PATH"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	// JWT密钥
	JWTSecret string `json:"jwt_secret" env:"JWT_SECRET" validate:"required,min=32"`
	// API密钥
	APIKey string `json:"api_key" env:"API_KEY" validate:"required"`
	// CORS允许的来源
	CORSAllowedOrigins []string `json:"cors_allowed_origins" env:"CORS_ALLOWED_ORIGINS"`
	// 加密密钥
	EncryptionKey string `json:"encryption_key" env:"ENCRYPTION_KEY" validate:"min=32"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	// 限流请求次数
	Requests int `json:"requests" env:"RATE_LIMIT_REQUESTS" validate:"min=1"`
	// 限流窗口时间
	Window time.Duration `json:"window" env:"RATE_LIMIT_WINDOW" validate:"required"`
}

// AlertConfig 告警配置
type AlertConfig struct {
	// 告警冷却时间
	Cooldown time.Duration `json:"cooldown" env:"ALERT_COOLDOWN" validate:"required"`
	// 每小时最大告警次数
	MaxPerHour int `json:"max_per_hour" env:"MAX_ALERTS_PER_HOUR" validate:"min=1"`
	// 告警重试次数
	RetryAttempts int `json:"retry_attempts" env:"ALERT_RETRY_ATTEMPTS"`
	// 告警重试间隔
	RetryInterval time.Duration `json:"retry_interval" env:"ALERT_RETRY_INTERVAL"`
}

// WorkerConfig 工作进程配置
type WorkerConfig struct {
	// 工作进程池大小
	PoolSize int `json:"pool_size" env:"WORKER_POOL_SIZE" validate:"min=1"`
	// 工作进程队列大小
	QueueSize int `json:"queue_size" env:"WORKER_QUEUE_SIZE" validate:"min=1"`
	// 工作进程超时时间
	Timeout time.Duration `json:"timeout" env:"WORKER_TIMEOUT" validate:"required"`
	// 工作进程批处理大小
	BatchSize int `json:"batch_size" env:"WORKER_BATCH_SIZE"`
}

// GetDSN 获取数据库连接字符串
func (d *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode)
}

// GetRedisAddr 获取Redis地址
func (r *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// IsProduction 判断是否为生产环境
func (a *AppConfig) IsProduction() bool {
	return a.Environment == "production"
}

// IsDevelopment 判断是否为开发环境
func (a *AppConfig) IsDevelopment() bool {
	return a.Environment == "development"
}
