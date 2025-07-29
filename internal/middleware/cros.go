package middleware

import (
	"net/http"
	"strings"
)

//CORS中间件 是一个HTTP中间件，用于处理跨域资源共享（CORS）请求。

// CORSMiddleware CORS中间件
type CORSMiddleware struct {
	// allowedOrigins: 允许的来源	a
	allowedOrigins []string
	// allowedMethods: 允许的HTTP方法
	allowedMethods []string
	// allowedHeaders: 允许的HTTP头
	allowedHeaders []string
}

// NewCORSMiddleware 创建CORS中间件
func NewCORSMiddleware(allowedOrigins []string) *CORSMiddleware {
	return &CORSMiddleware{
		allowedOrigins: allowedOrigins,
		allowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		allowedHeaders: []string{"Content-Type", "Authorization", "X-Requested-With"},
	}
}

// Middleware CORS中间件处理函数
func (m *CORSMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// 检查是否允许该来源
		if m.isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		w.Header().Set("Access-Control-Allow-Methods", strings.Join(m.allowedMethods, ", "))
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(m.allowedHeaders, ", "))
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// 处理预检请求
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isAllowedOrigin 检查是否允许该来源
func (m *CORSMiddleware) isAllowedOrigin(origin string) bool {
	for _, allowed := range m.allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}
