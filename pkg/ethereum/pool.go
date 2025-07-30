package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"
)

// LoadBalanceStrategy 负载均衡策略
type LoadBalanceStrategy string

const (
	StrategyRoundRobin LoadBalanceStrategy = "round_robin" // 轮询
	StrategyRandom     LoadBalanceStrategy = "random"      // 随机
	StrategyPriority   LoadBalanceStrategy = "priority"    // 优先级
	StrategyHealthy    LoadBalanceStrategy = "healthy"     // 最健康
)

// PoolConfig 连接池配置
type PoolConfig struct {
	// 客户端配置
	Clients []*ClientConfig `json:"clients"`
	// 负载均衡策略
	LoadBalanceStrategy LoadBalanceStrategy `json:"load_balance_strategy"`
	// 健康检查间隔
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	// 最大重试次数
	MaxRetries int `json:"max_retries"`
	// 重试延迟
	RetryDelay time.Duration `json:"retry_delay"`
	// 最小健康客户端数
	MinHealthyClients int `json:"min_healthy_clients"`
	// 是否启用故障转移
	EnableFailover bool `json:"enable_failover"`
	// 熔断器配置
	CircuitBreakerConfig *CircuitBreakerConfig `json:"circuit_breaker_config"`
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	// 失败阈值
	FailureThreshold int `json:"failure_threshold"`
	// 重置超时
	ResetTimeout time.Duration `json:"reset_timeout"`
	// 半开请求数
	HalfOpenRequests int `json:"half_open_requests"`
}

// CircuitBreakerState 熔断器状态
type CircuitBreakerState string

const (
	StateClosed   CircuitBreakerState = "closed"
	StateOpen     CircuitBreakerState = "open"
	StateHalfOpen CircuitBreakerState = "half_open"
)

// ClientPool 以太坊客户端连接池
type ClientPool struct {
	// 连接池配置
	config *PoolConfig
	// 客户端列表
	clients []*Client
	// 日志记录器
	logger *logrus.Logger
	// 读写锁
	mu sync.RWMutex
	// 轮询索引
	roundRobinIndex int
	// 健康检查器
	healthChecker *HealthChecker
	// 熔断器
	circuitBreaker *CircuitBreaker
	// 连接池统计信息
	stats *PoolStats
}

// PoolStats 连接池统计信息
type PoolStats struct {
	// 总客户端数
	TotalClients int `json:"total_clients"`
	// 健康客户端数
	HealthyClients int `json:"healthy_clients"`
	// 总请求数
	TotalRequests int64 `json:"total_requests"`
	// 失败请求数
	FailedRequests int64 `json:"failed_requests"`
	// 客户端统计信息
	ClientStats map[string]ClientStats `json:"client_stats"`
	// 最后更新时间
	LastUpdate time.Time `json:"last_update"`
}

// CircuitBreaker 熔断器实现
type CircuitBreaker struct {
	// 熔断器配置
	config *CircuitBreakerConfig
	// 熔断器状态
	state CircuitBreakerState
	// 失败次数
	failures int
	// 最后失败时间
	lastFailTime time.Time
	// 半开请求数
	halfOpenReqs int
	// 读写锁
	mu sync.RWMutex
}

// NewClientPool 创建新的客户端连接池
func NewClientPool(config *PoolConfig, logger *logrus.Logger) (*ClientPool, error) {
	if config == nil {
		return nil, fmt.Errorf("pool config cannot be nil")
	}

	if len(config.Clients) == 0 {
		return nil, fmt.Errorf("at least one client config is required")
	}

	// 设置默认值
	if config.LoadBalanceStrategy == "" {
		// 默认使用轮询策略
		config.LoadBalanceStrategy = StrategyRoundRobin
	}
	if config.HealthCheckInterval == 0 {
		// 默认健康检查间隔为30秒
		config.HealthCheckInterval = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		// 默认最大重试次数为3次
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		// 默认重试延迟为1秒
		config.RetryDelay = time.Second
	}
	if config.MinHealthyClients == 0 {
		// 默认最小健康客户端数为1
		config.MinHealthyClients = 1
	}

	if logger == nil {
		logger = logrus.New()
	}

	pool := &ClientPool{
		config: config,
		logger: logger,
		stats: &PoolStats{
			ClientStats: make(map[string]ClientStats),
		},
	}

	// 初始化熔断器
	if config.CircuitBreakerConfig != nil {
		pool.circuitBreaker = NewCircuitBreaker(config.CircuitBreakerConfig)
	}

	// 创建客户端
	if err := pool.initializeClients(); err != nil {
		return nil, fmt.Errorf("failed to initialize clients: %w", err)
	}

	// 启动健康检查
	pool.healthChecker = NewHealthChecker(pool, config.HealthCheckInterval, logger)
	pool.healthChecker.Start()

	return pool, nil
}

// initializeClients 初始化所有客户端
func (p *ClientPool) initializeClients() error {
	p.clients = make([]*Client, 0, len(p.config.Clients))

	for i, clientConfig := range p.config.Clients {
		client, err := NewClient(clientConfig, p.logger)
		if err != nil {
			p.logger.WithFields(logrus.Fields{
				"index": i,
				"url":   clientConfig.URL,
				"error": err,
			}).Warn("Failed to create client, skipping")
			continue
		}

		p.clients = append(p.clients, client)
		p.logger.WithFields(logrus.Fields{
			"index": i,
			"url":   clientConfig.URL,
			"type":  clientConfig.Type,
		}).Info("Client initialized successfully")
	}

	if len(p.clients) == 0 {
		return fmt.Errorf("no clients could be initialized")
	}

	p.updateStats()
	return nil
}

// GetClient 根据负载均衡策略获取客户端
func (p *ClientPool) GetClient() (*Client, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 检查熔断器状态
	if p.circuitBreaker != nil && !p.circuitBreaker.AllowRequest() {
		return nil, fmt.Errorf("circuit breaker is open")
	}

	healthyClients := p.getHealthyClients()
	if len(healthyClients) == 0 {
		return nil, fmt.Errorf("no healthy clients available")
	}

	var client *Client
	switch p.config.LoadBalanceStrategy {
	case StrategyRoundRobin:
		client = p.getRoundRobinClient(healthyClients)
	case StrategyRandom:
		client = p.getRandomClient(healthyClients)
	case StrategyPriority:
		client = p.getPriorityClient(healthyClients)
	case StrategyHealthy:
		client = p.getHealthiestClient(healthyClients)
	default:
		client = p.getRoundRobinClient(healthyClients)
	}

	return client, nil
}

// getHealthyClients 获取所有健康的客户端
func (p *ClientPool) getHealthyClients() []*Client {
	var healthy []*Client
	for _, client := range p.clients {
		if client.IsHealthy() {
			healthy = append(healthy, client)
		}
	}
	return healthy
}

// getRoundRobinClient 轮询获取客户端
func (p *ClientPool) getRoundRobinClient(clients []*Client) *Client {
	if len(clients) == 0 {
		return nil
	}

	p.roundRobinIndex = (p.roundRobinIndex + 1) % len(clients)
	return clients[p.roundRobinIndex]
}

// getRandomClient 随机获取客户端
func (p *ClientPool) getRandomClient(clients []*Client) *Client {
	if len(clients) == 0 {
		return nil
	}

	// 简单的伪随机选择
	index := int(time.Now().UnixNano()) % len(clients)
	return clients[index]
}

// getPriorityClient 根据优先级获取客户端
func (p *ClientPool) getPriorityClient(clients []*Client) *Client {
	if len(clients) == 0 {
		return nil
	}

	// 按优先级排序（数字越小优先级越高）
	sort.Slice(clients, func(i, j int) bool {
		return clients[i].config.Priority < clients[j].config.Priority
	})

	return clients[0]
}

// getHealthiestClient 获取最健康的客户端（错误率最低）
func (p *ClientPool) getHealthiestClient(clients []*Client) *Client {
	if len(clients) == 0 {
		return nil
	}

	var bestClient *Client
	var lowestErrorRate float64 = 1.0

	for _, client := range clients {
		stats := client.GetStats()
		if stats.ErrorRate < lowestErrorRate {
			lowestErrorRate = stats.ErrorRate
			bestClient = client
		}
	}

	if bestClient == nil {
		return clients[0]
	}

	return bestClient
}

// ExecuteWithFailover 执行带故障转移的操作
func (p *ClientPool) ExecuteWithFailover(ctx context.Context, operation func(*Client) error) error {
	var lastErr error
	attempts := 0
	maxAttempts := p.config.MaxRetries + 1

	for attempts < maxAttempts {
		client, err := p.GetClient()
		if err != nil {
			lastErr = err
			attempts++
			continue
		}

		p.mu.Lock()
		p.stats.TotalRequests++
		p.mu.Unlock()

		err = operation(client)
		if err == nil {
			// 成功时通知熔断器
			if p.circuitBreaker != nil {
				p.circuitBreaker.RecordSuccess()
			}
			return nil
		}

		lastErr = err
		attempts++

		// 记录失败
		p.mu.Lock()
		p.stats.FailedRequests++
		p.mu.Unlock()

		// 通知熔断器失败
		if p.circuitBreaker != nil {
			p.circuitBreaker.RecordFailure()
		}

		p.logger.WithFields(logrus.Fields{
			"attempt":      attempts,
			"max_attempts": maxAttempts,
			"client_url":   client.config.URL,
			"error":        err,
		}).Warn("Operation failed, trying next client")

		// 如果不是最后一次尝试，等待重试延迟
		if attempts < maxAttempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(p.config.RetryDelay):
			}
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", attempts, lastErr)
}

// GetLatestBlock 获取最新区块（带故障转移）
func (p *ClientPool) GetLatestBlock(ctx context.Context) (*types.Block, error) {
	var block *types.Block

	err := p.ExecuteWithFailover(ctx, func(client *Client) error {
		var err error
		block, err = client.GetLatestBlock(ctx)
		return err
	})

	return block, err
}

// GetBlockByNumber 根据区块号获取区块（带故障转移）
func (p *ClientPool) GetBlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	var block *types.Block

	err := p.ExecuteWithFailover(ctx, func(client *Client) error {
		var err error
		block, err = client.GetBlockByNumber(ctx, number)
		return err
	})

	return block, err
}

// GetTransactionByHash 根据交易哈希获取交易（带故障转移）
func (p *ClientPool) GetTransactionByHash(ctx context.Context, hash common.Hash) (*types.Transaction, bool, error) {
	var tx *types.Transaction
	var isPending bool

	err := p.ExecuteWithFailover(ctx, func(client *Client) error {
		var err error
		tx, isPending, err = client.GetTransactionByHash(ctx, hash)
		return err
	})

	return tx, isPending, err
}

// GetTransactionReceipt 获取交易收据（带故障转移）
func (p *ClientPool) GetTransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	var receipt *types.Receipt

	err := p.ExecuteWithFailover(ctx, func(client *Client) error {
		var err error
		receipt, err = client.GetTransactionReceipt(ctx, hash)
		return err
	})

	return receipt, err
}

// GetGasPrice 获取Gas价格（带故障转移）
func (p *ClientPool) GetGasPrice(ctx context.Context) (*big.Int, error) {
	var gasPrice *big.Int

	err := p.ExecuteWithFailover(ctx, func(client *Client) error {
		var err error
		gasPrice, err = client.GetGasPrice(ctx)
		return err
	})

	return gasPrice, err
}

// GetStats 获取连接池统计信息
func (p *ClientPool) GetStats() *PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	p.updateStats()
	return p.stats
}

// updateStats 更新统计信息
func (p *ClientPool) updateStats() {
	p.stats.TotalClients = len(p.clients)
	p.stats.HealthyClients = 0
	p.stats.LastUpdate = time.Now()

	for _, client := range p.clients {
		stats := client.GetStats()
		p.stats.ClientStats[client.config.URL] = stats

		if stats.IsHealthy {
			p.stats.HealthyClients++
		}
	}
}

// Close 关闭连接池
func (p *ClientPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 停止健康检查
	if p.healthChecker != nil {
		p.healthChecker.Stop()
	}

	// 关闭所有客户端
	for _, client := range p.clients {
		client.Close()
	}

	p.logger.Info("Client pool closed")
}

// NewCircuitBreaker 创建新的熔断器
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

// AllowRequest 检查是否允许请求
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// 检查是否可以转换到半开状态
		if time.Since(cb.lastFailTime) > cb.config.ResetTimeout {
			cb.state = StateHalfOpen
			cb.halfOpenReqs = 0
			return true
		}
		return false
	case StateHalfOpen:
		// 半开状态允许有限的请求
		return cb.halfOpenReqs < cb.config.HalfOpenRequests
	default:
		return false
	}
}

// RecordSuccess 记录成功请求
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateHalfOpen {
		cb.halfOpenReqs++
		if cb.halfOpenReqs >= cb.config.HalfOpenRequests {
			cb.state = StateClosed
			cb.failures = 0
		}
	} else {
		cb.failures = 0
	}
}

// RecordFailure 记录失败请求
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailTime = time.Now()

	if cb.failures >= cb.config.FailureThreshold {
		cb.state = StateOpen
	}
}
