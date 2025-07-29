package database

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"simplied-blockchain-data-monitor-alert-go/internal/config"
	"simplied-blockchain-data-monitor-alert-go/pkg/logger"
)

// RedisManager Redis 连接管理器
type RedisManager struct {
	// Redis 客户端
	client redis.UniversalClient
	// 配置
	config config.RedisConfig
	// 日志记录器
	logger *logger.Logger
	// 指标收集器
	metrics *RedisMetrics
}

// RedisMetrics Redis 指标
type RedisMetrics struct {
	// 连接指标
	// 活跃连接数
	ConnectionsActive prometheus.Gauge
	// 空闲连接数
	ConnectionsIdle prometheus.Gauge
	// 命令指标
	// 总命令数
	CommandsTotal *prometheus.CounterVec
	// 命令执行时长
	CommandDuration *prometheus.HistogramVec
	// 错误指标
	// 总错误数
	ErrorsTotal *prometheus.CounterVec
	// 内存指标
	// 内存使用量
	MemoryUsage prometheus.Gauge
}

// NewRedisManager 创建 Redis 管理器
func NewRedisManager(cfg config.RedisConfig, logger *logger.Logger) (*RedisManager, error) {
	// 创建 Redis 客户端配置
	options := &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,

		// 连接超时配置
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,

		// 连接池配置
		PoolTimeout:  4 * time.Second,
		IdleTimeout:  5 * time.Minute,
		MaxRetries:   3,
		MinIdleConns: 10,
	}

	// 创建 Redis 客户端
	client := redis.NewClient(options)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// 创建指标收集器
	metrics := &RedisMetrics{
		ConnectionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "redis_connections_active",
			Help: "Number of active Redis connections",
		}),
		ConnectionsIdle: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "redis_connections_idle",
			Help: "Number of idle Redis connections",
		}),
		CommandsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "redis_commands_total",
				Help: "Total number of Redis commands",
			},
			[]string{"command", "status"},
		),
		CommandDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "redis_command_duration_seconds",
				Help:    "Redis command duration in seconds",
				Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5},
			},
			[]string{"command"},
		),
		ErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "redis_errors_total",
				Help: "Total number of Redis errors",
			},
			[]string{"type"},
		),
		MemoryUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "redis_memory_usage_bytes",
			Help: "Redis memory usage in bytes",
		}),
	}

	// 注册指标
	prometheus.MustRegister(
		metrics.ConnectionsActive,
		metrics.ConnectionsIdle,
		metrics.CommandsTotal,
		metrics.CommandDuration,
		metrics.ErrorsTotal,
		metrics.MemoryUsage,
	)

	manager := &RedisManager{
		client:  client,
		config:  cfg,
		logger:  logger,
		metrics: metrics,
	}

	// 启动指标更新协程
	go manager.updateMetrics()

	logger.WithFields(logrus.Fields{
		"host": cfg.Host,
		"port": cfg.Port,
		"db":   cfg.DB,
	}).Info("Redis connection established")

	return manager, nil
}

// GetClient 获取 Redis 客户端
func (rm *RedisManager) GetClient() redis.UniversalClient {
	return rm.client
}

// Ping 检查 Redis 连接
func (rm *RedisManager) Ping(ctx context.Context) error {
	start := time.Now()
	err := rm.client.Ping(ctx).Err()
	duration := time.Since(start)

	if err != nil {
		rm.metrics.ErrorsTotal.WithLabelValues("ping").Inc()
		rm.logger.WithFields(logrus.Fields{
			"error":    err.Error(),
			"duration": duration.Milliseconds(),
		}).Error("Redis ping failed")
		return err
	}

	rm.logger.WithFields(logrus.Fields{
		"duration": duration.Milliseconds(),
	}).Debug("Redis ping successful")

	return nil
}

// Close 关闭 Redis 连接
func (rm *RedisManager) Close() error {
	if rm.client != nil {
		rm.logger.Info("Closing Redis connection")
		return rm.client.Close()
	}
	return nil
}

// Set 设置键值对
func (rm *RedisManager) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	start := time.Now()

	err := rm.client.Set(ctx, key, value, expiration).Err()
	duration := time.Since(start)

	// 记录指标
	rm.metrics.CommandDuration.WithLabelValues("set").Observe(duration.Seconds())

	if err != nil {
		rm.metrics.CommandsTotal.WithLabelValues("set", "error").Inc()
		rm.metrics.ErrorsTotal.WithLabelValues("command").Inc()

		rm.logger.WithFields(logrus.Fields{
			"key":      key,
			"error":    err.Error(),
			"duration": duration.Milliseconds(),
		}).Error("Redis SET failed")

		return err
	}

	rm.metrics.CommandsTotal.WithLabelValues("set", "success").Inc()

	rm.logger.WithFields(logrus.Fields{
		"key":      key,
		"duration": duration.Milliseconds(),
	}).Debug("Redis SET executed")

	return nil
}

// Get 获取键值
func (rm *RedisManager) Get(ctx context.Context, key string) (string, error) {
	start := time.Now()

	result, err := rm.client.Get(ctx, key).Result()
	duration := time.Since(start)

	// 记录指标
	rm.metrics.CommandDuration.WithLabelValues("get").Observe(duration.Seconds())

	if err != nil {
		if err == redis.Nil {
			rm.metrics.CommandsTotal.WithLabelValues("get", "miss").Inc()
		} else {
			rm.metrics.CommandsTotal.WithLabelValues("get", "error").Inc()
			rm.metrics.ErrorsTotal.WithLabelValues("command").Inc()
		}

		rm.logger.WithFields(logrus.Fields{
			"key":      key,
			"error":    err.Error(),
			"duration": duration.Milliseconds(),
		}).Debug("Redis GET failed")

		return "", err
	}

	rm.metrics.CommandsTotal.WithLabelValues("get", "hit").Inc()

	rm.logger.WithFields(logrus.Fields{
		"key":      key,
		"duration": duration.Milliseconds(),
	}).Debug("Redis GET executed")

	return result, nil
}

// Delete 删除键
func (rm *RedisManager) Delete(ctx context.Context, keys ...string) error {
	start := time.Now()

	err := rm.client.Del(ctx, keys...).Err()
	duration := time.Since(start)

	// 记录指标
	rm.metrics.CommandDuration.WithLabelValues("del").Observe(duration.Seconds())

	if err != nil {
		rm.metrics.CommandsTotal.WithLabelValues("del", "error").Inc()
		rm.metrics.ErrorsTotal.WithLabelValues("command").Inc()

		rm.logger.WithFields(logrus.Fields{
			"keys":     keys,
			"error":    err.Error(),
			"duration": duration.Milliseconds(),
		}).Error("Redis DEL failed")

		return err
	}

	rm.metrics.CommandsTotal.WithLabelValues("del", "success").Inc()

	rm.logger.WithFields(logrus.Fields{
		"keys":     keys,
		"duration": duration.Milliseconds(),
	}).Debug("Redis DEL executed")

	return nil
}

// Exists 检查键是否存在
func (rm *RedisManager) Exists(ctx context.Context, keys ...string) (int64, error) {
	start := time.Now()

	result, err := rm.client.Exists(ctx, keys...).Result()
	duration := time.Since(start)

	// 记录指标
	rm.metrics.CommandDuration.WithLabelValues("exists").Observe(duration.Seconds())

	if err != nil {
		rm.metrics.CommandsTotal.WithLabelValues("exists", "error").Inc()
		rm.metrics.ErrorsTotal.WithLabelValues("command").Inc()

		rm.logger.WithFields(logrus.Fields{
			"keys":     keys,
			"error":    err.Error(),
			"duration": duration.Milliseconds(),
		}).Error("Redis EXISTS failed")

		return 0, err
	}

	rm.metrics.CommandsTotal.WithLabelValues("exists", "success").Inc()

	rm.logger.WithFields(logrus.Fields{
		"keys":     keys,
		"result":   result,
		"duration": duration.Milliseconds(),
	}).Debug("Redis EXISTS executed")

	return result, nil
}

// updateMetrics 更新 Redis 指标
func (rm *RedisManager) updateMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		// 获取连接池统计
		poolStats := rm.client.PoolStats()
		rm.metrics.ConnectionsActive.Set(float64(poolStats.TotalConns))
		rm.metrics.ConnectionsIdle.Set(float64(poolStats.IdleConns))

		// 获取内存使用情况
		if info, err := rm.client.Info(ctx, "memory").Result(); err == nil {
			// 解析内存使用信息（简化版本）
			rm.logger.WithField("memory_info", info).Debug("Redis memory info")
		}

		cancel()
	}
}

// GetPoolStats 获取连接池统计信息
func (rm *RedisManager) GetPoolStats() *redis.PoolStats {
	return rm.client.PoolStats()
}
