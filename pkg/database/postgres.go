package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"simplied-blockchain-data-monitor-alert-go/internal/config"
	"simplied-blockchain-data-monitor-alert-go/pkg/logger"
)

// PostgresManager PostgreSQL 连接管理器
// PostgreSQL 是一个开源的关系型数据库管理系统，广泛用于存储和管理结构化数据。
// PostgreSQL is an open-source relational database management system that is widely used to store and manage structured data.
type PostgresManager struct {
	// 数据库连接
	db *sqlx.DB
	// 配置
	config config.DatabaseConfig
	// 日志记录器
	logger *logger.Logger
	// 指标收集器
	metrics *PostgresMetrics
}

// PostgresMetrics PostgreSQL 指标
type PostgresMetrics struct {
	// 连接池指标
	// 活跃连接数
	ConnectionsActive prometheus.Gauge
	// 空闲连接数
	ConnectionsIdle prometheus.Gauge
	// 等待连接数
	ConnectionsWaiting prometheus.Gauge
	// 查询指标
	// 查询总数
	QueriesTotal *prometheus.CounterVec
	// 查询耗时
	QueryDuration *prometheus.HistogramVec
	// 错误指标
	// 错误总数
	ErrorsTotal *prometheus.CounterVec
}

// NewPostgresManager 创建 PostgreSQL 管理器
func NewPostgresManager(cfg config.DatabaseConfig, logger *logger.Logger) (*PostgresManager, error) {
	// 构建数据库连接字符串
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode,
	)

	// 创建数据库连接
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// 配置连接池
	// 最大打开连接数
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	// 最大空闲连接数
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	// 连接最大生命周期
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// 创建指标收集器
	metrics := &PostgresMetrics{
		ConnectionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "postgres_connections_active",
			Help: "Number of active PostgreSQL connections",
		}),
		ConnectionsIdle: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "postgres_connections_idle",
			Help: "Number of idle PostgreSQL connections",
		}),
		ConnectionsWaiting: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "postgres_connections_waiting",
			Help: "Number of waiting PostgreSQL connections",
		}),
		QueriesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "postgres_queries_total",
				Help: "Total number of PostgreSQL queries",
			},
			[]string{"operation", "status"},
		),
		QueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "postgres_query_duration_seconds",
				Help:    "PostgreSQL query duration in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0},
			},
			[]string{"operation"},
		),
		ErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "postgres_errors_total",
				Help: "Total number of PostgreSQL errors",
			},
			[]string{"type"},
		),
	}

	// 注册指标
	prometheus.MustRegister(
		metrics.ConnectionsActive,
		metrics.ConnectionsIdle,
		metrics.ConnectionsWaiting,
		metrics.QueriesTotal,
		metrics.QueryDuration,
		metrics.ErrorsTotal,
	)

	manager := &PostgresManager{
		db:      db,
		config:  cfg,
		logger:  logger,
		metrics: metrics,
	}

	// 启动指标更新协程
	go manager.updateMetrics()

	logger.WithFields(logrus.Fields{
		"host": cfg.Host,
		"port": cfg.Port,
		"name": cfg.Name,
	}).Info("PostgreSQL connection established")

	return manager, nil
}

// GetDB 获取数据库连接
func (pm *PostgresManager) GetDB() *sqlx.DB {
	return pm.db
}

// Ping 检查数据库连接
func (pm *PostgresManager) Ping(ctx context.Context) error {
	start := time.Now()
	err := pm.db.PingContext(ctx)
	duration := time.Since(start)

	if err != nil {
		pm.metrics.ErrorsTotal.WithLabelValues("ping").Inc()
		pm.logger.WithFields(logrus.Fields{
			"error":    err.Error(),
			"duration": duration.Milliseconds(),
		}).Error("PostgreSQL ping failed")
		return err
	}

	pm.logger.WithFields(logrus.Fields{
		"duration": duration.Milliseconds(),
	}).Debug("PostgreSQL ping successful")

	return nil
}

// Close 关闭数据库连接
func (pm *PostgresManager) Close() error {
	if pm.db != nil {
		pm.logger.Info("Closing PostgreSQL connection")
		return pm.db.Close()
	}
	return nil
}

// ExecuteQuery 执行查询并记录指标
func (pm *PostgresManager) ExecuteQuery(ctx context.Context, operation, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()

	rows, err := pm.db.QueryContext(ctx, query, args...)
	duration := time.Since(start)

	// 记录指标
	pm.metrics.QueryDuration.WithLabelValues(operation).Observe(duration.Seconds())

	if err != nil {
		pm.metrics.QueriesTotal.WithLabelValues(operation, "error").Inc()
		pm.metrics.ErrorsTotal.WithLabelValues("query").Inc()

		pm.logger.WithFields(logrus.Fields{
			"operation": operation,
			"error":     err.Error(),
			"duration":  duration.Milliseconds(),
		}).Error("PostgreSQL query failed")

		return nil, err
	}

	pm.metrics.QueriesTotal.WithLabelValues(operation, "success").Inc()

	pm.logger.WithFields(logrus.Fields{
		"operation": operation,
		"duration":  duration.Milliseconds(),
	}).Debug("PostgreSQL query executed")

	return rows, nil
}

// BeginTx 开始事务
func (pm *PostgresManager) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	tx, err := pm.db.BeginTxx(ctx, nil)
	if err != nil {
		pm.metrics.ErrorsTotal.WithLabelValues("transaction").Inc()
		pm.logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Failed to begin PostgreSQL transaction")
		return nil, err
	}

	pm.logger.Debug("PostgreSQL transaction started")
	return tx, nil
}

// updateMetrics 更新连接池指标
func (pm *PostgresManager) updateMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := pm.db.Stats()
		pm.metrics.ConnectionsActive.Set(float64(stats.OpenConnections))
		pm.metrics.ConnectionsIdle.Set(float64(stats.Idle))
		pm.metrics.ConnectionsWaiting.Set(float64(stats.WaitCount))
	}
}

// GetStats 获取连接池统计信息
func (pm *PostgresManager) GetStats() sql.DBStats {
	return pm.db.Stats()
}
