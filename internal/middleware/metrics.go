package middleware

import (
	"net/http"
	"simplied-blockchain-data-monitor-alert-go/pkg/metrics"
	"time"
)

// MetricsMiddleware 指标中间件
type MetricsMiddleware struct {
	// metrics *metrics.Metrics
	metrics *metrics.Metrics
}

// NewMetricsMiddleware 创建指标中间件
func NewMetricsMiddleware(metrics *metrics.Metrics) *MetricsMiddleware {
	return &MetricsMiddleware{
		metrics: metrics,
	}
}

// Middleware 指标中间件处理函数
func (m *MetricsMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 增加正在处理的请求数
		m.metrics.HTTPRequestsInFlight.Inc()
		defer m.metrics.HTTPRequestsInFlight.Dec()

		// 包装响应写入器以捕获状态码
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// 处理请求
		next.ServeHTTP(rw, r)

		// 记录指标
		duration := time.Since(start)
		m.metrics.RecordHTTPRequest(r.Method, r.URL.Path, rw.statusCode, duration)
	})
}
