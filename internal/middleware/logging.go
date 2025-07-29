package middleware

import (
	"net/http"
	"simplied-blockchain-data-monitor-alert-go/pkg/logger"
	"time"
)

// LoggingMiddleware 日志中间件
type LoggingMiddleware struct {
	logger *logger.Logger
}

// NewLoggingMiddleware 创建日志中间件
func NewLoggingMiddleware(logger *logger.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger: logger,
	}
}

// responseWriter 包装响应写入器以捕获状态码
type responseWriter struct {
	// http.ResponseWriter: HTTP响应写入器接口
	http.ResponseWriter
	// statusCode: HTTP状态码
	statusCode int
}

// WriteHeader 设置HTTP状态码
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Middleware 日志中间件处理函数
func (m *LoggingMiddleware) Middleware(next http.Handler) http.Handler {
	// http.HandlerFunc: HTTP处理函数
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 包装响应写入器
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// 处理请求
		next.ServeHTTP(rw, r)

		// 计算处理时间
		duration := time.Since(start)

		// 记录日志
		m.logger.LogHTTPRequest(r.Method, r.URL.Path, r.UserAgent(), r.RemoteAddr, rw.statusCode, duration)
	})
}

// getClientIP 获取客户端IP地址
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}
