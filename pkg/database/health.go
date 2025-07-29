package database

import (
	"context"
	"sync"
	"time"

	"simplied-blockchain-data-monitor-alert-go/pkg/logger"
)

// HealthStatus 健康状态
type HealthStatus struct {
	// 健康状态
	Healthy bool `json:"healthy"`
	// 健康状态
	Status string `json:"status"`
	// 响应时间
	ResponseTime time.Duration `json:"response_time"`
	// 最后检查时间
	LastCheck time.Time `json:"last_check"`
	// 错误信息
	Error string `json:"error,omitempty"`
}

// DatabaseHealth 数据库健康状态
type DatabaseHealth struct {
	// PostgreSQL 健康状态
	PostgreSQL HealthStatus `json:"postgresql"`
	// Redis 健康状态
	Redis HealthStatus `json:"redis"`
	// 总体健康状态
	Overall HealthStatus `json:"overall"`
}

// HealthChecker 健康检查器
type HealthChecker struct {
	// PostgreSQL 管理器
	postgres *PostgresManager
	// Redis 管理器
	redis *RedisManager
	// 日志记录器
	logger *logger.Logger
	// 健康状态
	health *DatabaseHealth
	// 读写锁
	mutex sync.RWMutex
	// 检查间隔
	interval time.Duration
	// 停止通道
	stopChan chan struct{}
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(postgres *PostgresManager, redis *RedisManager, logger *logger.Logger) *HealthChecker {
	return &HealthChecker{
		postgres: postgres,
		redis:    redis,
		logger:   logger,
		health: &DatabaseHealth{
			PostgreSQL: HealthStatus{Status: "unknown"},
			Redis:      HealthStatus{Status: "unknown"},
			Overall:    HealthStatus{Status: "unknown"},
		},
		interval: 30 * time.Second,
		stopChan: make(chan struct{}),
	}
}

// Start 启动健康检查
func (hc *HealthChecker) Start() {
	hc.logger.Info("Starting database health checker")
	hc.checkHealth()

	ticker := time.NewTicker(hc.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				hc.checkHealth()
			case <-hc.stopChan:
				hc.logger.Info("Database health checker stopped")
				return
			}
		}
	}()
}

// GetHealth 获取健康状态
func (hc *HealthChecker) GetHealth() DatabaseHealth {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()
	return *hc.health
}

// checkHealth 执行健康检查
func (hc *HealthChecker) checkHealth() {
	/**
	1. 获取互斥锁
	2. 获取当前时间
	3. 检查 PostgreSQL 健康状态
	4. 检查 Redis 健康状态
	5. 计算总体健康状态
	*/
	hc.mutex.Lock()
	defer hc.mutex.Unlock()

	now := time.Now()

	pgStatus := hc.checkPostgreSQL()
	pgStatus.LastCheck = now
	hc.health.PostgreSQL = pgStatus

	redisStatus := hc.checkRedis()
	redisStatus.LastCheck = now
	hc.health.Redis = redisStatus

	overall := HealthStatus{
		Healthy:      pgStatus.Healthy && redisStatus.Healthy,
		LastCheck:    now,
		ResponseTime: (pgStatus.ResponseTime + redisStatus.ResponseTime) / 2,
	}

	if overall.Healthy {
		overall.Status = "healthy"
	} else {
		overall.Status = "unhealthy"
		if !pgStatus.Healthy && !redisStatus.Healthy {
			overall.Error = "Both PostgreSQL and Redis are unhealthy"
		} else if !pgStatus.Healthy {
			overall.Error = "PostgreSQL is unhealthy"
		} else {
			overall.Error = "Redis is unhealthy"
		}
	}

	hc.health.Overall = overall
}

// checkPostgreSQL 检查 PostgreSQL 健康状态
func (hc *HealthChecker) checkPostgreSQL() HealthStatus {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := hc.postgres.Ping(ctx)
	responseTime := time.Since(start)

	if err != nil {
		return HealthStatus{
			Healthy:      false,
			Status:       "unhealthy",
			ResponseTime: responseTime,
			Error:        err.Error(),
		}
	}

	return HealthStatus{
		Healthy:      true,
		Status:       "healthy",
		ResponseTime: responseTime,
	}
}

// checkRedis 检查 Redis 健康状态
func (hc *HealthChecker) checkRedis() HealthStatus {
	/**
	1. 设置超时时间
	2. 执行 ping 操作
	3. 计算响应时间
	4. 返回健康状态
	*/
	start := time.Now()
	// context.WithTimeout 设置超时时间 表示5秒内未完成则取消
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := hc.redis.Ping(ctx)
	responseTime := time.Since(start)

	if err != nil {
		return HealthStatus{
			Healthy:      false,
			Status:       "unhealthy",
			ResponseTime: responseTime,
			Error:        err.Error(),
		}
	}

	return HealthStatus{
		Healthy:      true,
		Status:       "healthy",
		ResponseTime: responseTime,
	}
}
