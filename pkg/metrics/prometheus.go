package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//Prometheus 是一个开源的监控系统和时间序列数据库，用于监控和分析应用程序的性能和状态。
// Prometheus is an open-source monitoring system and time series database, used to monitor and analyze the performance and status of applications.

// Metrics Prometheus指标收集器
type Metrics struct {
	// HTTP相关指标
	// HTTP请求总数
	HTTPRequestsTotal *prometheus.CounterVec
	// HTTP请求耗时
	HTTPRequestDuration *prometheus.HistogramVec
	// HTTP请求并发数
	HTTPRequestsInFlight prometheus.Gauge

	// 数据库相关指标
	// 数据库连接数
	DatabaseConnectionsActive prometheus.Gauge
	// 数据库查询耗时
	DatabaseQueryDuration *prometheus.HistogramVec
	// 数据库查询总数
	DatabaseQueriesTotal *prometheus.CounterVec

	// 区块链相关指标
	// 区块链处理的区块总数
	BlockchainBlocksProcessed *prometheus.CounterVec
	// 区块链最新区块高度
	BlockchainLatestBlock prometheus.Gauge

	// 告警相关指标
	// 告警总数
	AlertsTotal *prometheus.CounterVec
	// 激活的告警数
	AlertsActive prometheus.Gauge

	// 系统相关指标
	// 应用程序信息
	ApplicationInfo *prometheus.GaugeVec
	// 应用程序运行时长
	ApplicationUptime prometheus.Gauge
}

// NewMetrics 创建新的指标收集器
func NewMetrics(namespace string) *Metrics {
	// 创建指标收集器
	return &Metrics{
		// HTTP相关指标
		HTTPRequestsTotal: prometheus.NewCounterVec(
			// prometheus.CounterOpts{}: 定义指标的名称、帮助信息和命名空间
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "path", "status_code"},
		),
		// HTTP请求耗时
		HTTPRequestDuration: prometheus.NewHistogramVec(
			// prometheus.HistogramOpts{}: 定义指标的名称、帮助信息和命名空间
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		// HTTP请求并发数
		HTTPRequestsInFlight: prometheus.NewGauge(
			// prometheus.GaugeOpts{}: 定义指标的名称、帮助信息和命名空间
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "http_requests_in_flight",
				Help:      "Number of HTTP requests currently being processed",
			},
		),
		// 数据库相关指标
		DatabaseConnectionsActive: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "database_connections_active",
				Help:      "Number of active database connections",
			},
		),
		// 数据库查询耗时
		DatabaseQueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "database_query_duration_seconds",
				Help:      "Database query duration in seconds",
				Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
			},
			[]string{"operation", "table"},
		),
		// 数据库查询总数
		DatabaseQueriesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "database_queries_total",
				Help:      "Total number of database queries",
			},
			[]string{"operation", "table", "status"},
		),
		// 区块链相关指标
		BlockchainBlocksProcessed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "blockchain_blocks_processed_total",
				Help:      "Total number of blockchain blocks processed",
			},
			[]string{"network"},
		),
		// 区块链最新区块高度
		BlockchainLatestBlock: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "blockchain_latest_block_number",
				Help:      "Latest block number processed",
			},
		),
		// 告警相关指标
		AlertsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "alerts_total",
				Help:      "Total number of alerts triggered",
			},
			[]string{"type", "severity"},
		),
		// 激活的告警数
		AlertsActive: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "alerts_active",
				Help:      "Number of active alerts",
			},
		),
		// 应用程序信息
		ApplicationInfo: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "application_info",
				Help:      "Application information",
			},
			[]string{"version", "environment"},
		),
		// 应用程序运行时长
		ApplicationUptime: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "application_uptime_seconds",
				Help:      "Application uptime in seconds",
			},
		),
	}
}

// Register 注册所有指标
func (m *Metrics) Register() error {
	// 注册所有指标
	//prometheus.Collector{}: 指标收集器接口
	collectors := []prometheus.Collector{
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.HTTPRequestsInFlight,
		m.DatabaseConnectionsActive,
		m.DatabaseQueryDuration,
		m.DatabaseQueriesTotal,
		m.BlockchainBlocksProcessed,
		m.BlockchainLatestBlock,
		m.AlertsTotal,
		m.AlertsActive,
		m.ApplicationInfo,
		m.ApplicationUptime,
	}

	for _, collector := range collectors {
		if err := prometheus.Register(collector); err != nil {
			return err
		}
	}

	return nil
}

// RecordHTTPRequest 记录HTTP请求指标
func (m *Metrics) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration) {
	m.HTTPRequestsTotal.WithLabelValues(method, path, strconv.Itoa(statusCode)).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// Handler 返回Prometheus HTTP处理器
func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}
