package ethereum

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// HealthChecker 健康检查器
type HealthChecker struct {
	// 客户端连接池
	pool *ClientPool
	// 健康检查间隔
	interval time.Duration
	// 日志记录器
	logger *logrus.Logger
	// 停止通道
	stopCh chan struct{}
	// 等待组
	wg sync.WaitGroup
	// 读写锁
	mu sync.RWMutex
	// 是否正在运行
	running bool
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	// 客户端URL
	ClientURL string `json:"client_url"`
	// 健康状态
	IsHealthy bool `json:"is_healthy"`
	// 错误信息
	Error string `json:"error,omitempty"`
	// 响应时间
	ResponseTime time.Duration `json:"response_time"`
	// 最新区块号
	BlockNumber uint64 `json:"block_number"`
	// 链ID
	ChainID string `json:"chain_id"`
	// 检查时间
	CheckTime time.Time `json:"check_time"`
}

// NewHealthChecker 创建新的健康检查器
func NewHealthChecker(pool *ClientPool, interval time.Duration, logger *logrus.Logger) *HealthChecker {
	if logger == nil {
		logger = logrus.New()
	}

	return &HealthChecker{
		pool:     pool,
		interval: interval,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动健康检查
func (hc *HealthChecker) Start() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.running {
		return
	}

	hc.running = true
	hc.wg.Add(1)

	go hc.run()

	hc.logger.WithField("interval", hc.interval).Info("Health checker started")
}

// Stop 停止健康检查
func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	if !hc.running {
		hc.mu.Unlock()
		return
	}
	hc.running = false
	hc.mu.Unlock()

	close(hc.stopCh)
	hc.wg.Wait()

	hc.logger.Info("Health checker stopped")
}

// run 运行健康检查循环
func (hc *HealthChecker) run() {
	defer hc.wg.Done()

	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	// 立即执行一次健康检查
	hc.performHealthCheck()

	for {
		select {
		case <-ticker.C:
			hc.performHealthCheck()
		case <-hc.stopCh:
			return
		}
	}
}

// performHealthCheck 执行健康检查
func (hc *HealthChecker) performHealthCheck() {
	hc.pool.mu.RLock()
	clients := make([]*Client, len(hc.pool.clients))
	copy(clients, hc.pool.clients)
	hc.pool.mu.RUnlock()

	var wg sync.WaitGroup
	results := make(chan *HealthCheckResult, len(clients))

	// 并发检查所有客户端
	for _, client := range clients {
		wg.Add(1)
		go func(c *Client) {
			defer wg.Done()
			result := hc.checkClient(c)
			results <- result
		}(client)
	}

	// 等待所有检查完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 处理检查结果
	healthyCount := 0
	totalCount := 0

	for result := range results {
		totalCount++
		if result.IsHealthy {
			healthyCount++
		}

		hc.logger.WithFields(logrus.Fields{
			"client_url":    result.ClientURL,
			"is_healthy":    result.IsHealthy,
			"response_time": result.ResponseTime,
			"block_number":  result.BlockNumber,
			"chain_id":      result.ChainID,
			"error":         result.Error,
		}).Debug("Health check result")
	}

	// 检查是否满足最小健康客户端数量要求
	if healthyCount < hc.pool.config.MinHealthyClients {
		hc.logger.WithFields(logrus.Fields{
			"healthy_clients": healthyCount,
			"min_required":    hc.pool.config.MinHealthyClients,
			"total_clients":   totalCount,
		}).Warn("Insufficient healthy clients")

		// 尝试重连不健康的客户端
		hc.attemptReconnection()
	}

	hc.logger.WithFields(logrus.Fields{
		"healthy_clients": healthyCount,
		"total_clients":   totalCount,
	}).Info("Health check completed")
}

// checkClient 检查单个客户端的健康状态
func (hc *HealthChecker) checkClient(client *Client) *HealthCheckResult {
	result := &HealthCheckResult{
		ClientURL: client.config.URL,
		CheckTime: time.Now(),
	}

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 检查连接状态
	if !client.IsHealthy() {
		result.Error = "Client marked as unhealthy"
		result.ResponseTime = time.Since(startTime)
		return result
	}

	// 尝试获取最新区块号
	block, err := client.GetLatestBlock(ctx)
	result.ResponseTime = time.Since(startTime)

	if err != nil {
		result.Error = err.Error()
		// 标记客户端为不健康
		client.mu.Lock()
		client.isHealthy = false
		client.lastError = err
		client.lastCheck = time.Now()
		client.mu.Unlock()
		return result
	}

	// 更新客户端健康状态
	client.mu.Lock()
	client.isHealthy = true
	client.lastError = nil
	client.lastCheck = time.Now()
	client.mu.Unlock()

	result.IsHealthy = true
	result.BlockNumber = block.NumberU64()
	if client.config.ChainID != nil {
		result.ChainID = client.config.ChainID.String()
	}

	return result
}

// attemptReconnection 尝试重连不健康的客户端
func (hc *HealthChecker) attemptReconnection() {
	hc.pool.mu.RLock()
	clients := make([]*Client, len(hc.pool.clients))
	copy(clients, hc.pool.clients)
	hc.pool.mu.RUnlock()

	var wg sync.WaitGroup

	for _, client := range clients {
		if !client.IsHealthy() {
			wg.Add(1)
			go func(c *Client) {
				defer wg.Done()
				hc.reconnectClient(c)
			}(client)
		}
	}

	wg.Wait()
}

// reconnectClient 重连单个客户端
func (hc *HealthChecker) reconnectClient(client *Client) {
	hc.logger.WithField("client_url", client.config.URL).Info("Attempting to reconnect client")

	// 关闭现有连接
	client.Close()

	// 尝试重新连接
	err := client.Connect()
	if err != nil {
		hc.logger.WithFields(logrus.Fields{
			"client_url": client.config.URL,
			"error":      err,
		}).Error("Failed to reconnect client")
		return
	}

	hc.logger.WithField("client_url", client.config.URL).Info("Client reconnected successfully")
}

// GetHealthStatus 获取所有客户端的健康状态
func (hc *HealthChecker) GetHealthStatus() []*HealthCheckResult {
	hc.pool.mu.RLock()
	clients := make([]*Client, len(hc.pool.clients))
	copy(clients, hc.pool.clients)
	hc.pool.mu.RUnlock()

	results := make([]*HealthCheckResult, 0, len(clients))

	for _, client := range clients {
		result := hc.checkClient(client)
		results = append(results, result)
	}

	return results
}

// IsRunning 检查健康检查器是否正在运行
func (hc *HealthChecker) IsRunning() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.running
}

// UpdateInterval 更新健康检查间隔
func (hc *HealthChecker) UpdateInterval(interval time.Duration) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.interval = interval
	hc.logger.WithField("new_interval", interval).Info("Health check interval updated")
}

// ForceHealthCheck 强制执行一次健康检查
func (hc *HealthChecker) ForceHealthCheck() []*HealthCheckResult {
	hc.logger.Info("Forcing health check")
	return hc.GetHealthStatus()
}
