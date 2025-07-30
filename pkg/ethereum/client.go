package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/sirupsen/logrus"
)

// ClientType 定义客户端连接类型
type ClientType string

const (
	ClientTypeHTTP      ClientType = "http"      // HTTP连接
	ClientTypeWebSocket ClientType = "websocket" // WebSocket连接
	ClientTypeIPC       ClientType = "ipc"       // IPC连接
)

// ClientConfig 客户端配置
type ClientConfig struct {
	// URL: 以太坊节点的URL
	URL string `json:"url"`
	// Type: 连接类型，支持http、websocket、ipc
	Type ClientType `json:"type"`
	// Timeout: 连接超时时间
	Timeout time.Duration `json:"timeout"`
	// RetryAttempts: 重试次数
	RetryAttempts int `json:"retry_attempts"`
	// RetryDelay: 重试延迟时间
	RetryDelay time.Duration `json:"retry_delay"`
	// MaxConcurrency: 最大并发数
	MaxConcurrency int `json:"max_concurrency"`
	// HealthCheckURL: 健康检查URL
	HealthCheckURL string `json:"health_check_url"`
	// ChainID: 区块链ID
	ChainID *big.Int `json:"chain_id"`
	// NetworkName: 区块链名称
	NetworkName string `json:"network_name"`
	// Priority: 节点优先级，数字越小优先级越高
	Priority int `json:"priority"`
}

// Client 以太坊客户端封装
type Client struct {
	// config: 客户端配置
	config *ClientConfig
	// ethClient: 以太坊客户端
	ethClient *ethclient.Client
	// rpcClient: RPC客户端
	rpcClient *rpc.Client
	// logger: 日志记录器
	logger *logrus.Logger
	// mu: 读写锁
	mu sync.RWMutex
	// isHealthy: 是否健康
	isHealthy bool
	// lastError: 最后一次错误
	lastError error
	// lastCheck: 最后一次检查时间
	lastCheck time.Time
	// connectedAt: 连接时间
	connectedAt time.Time
	// requestCount: 请求次数
	requestCount int64
	// errorCount: 错误次数
	errorCount int64
}

// ClientStats 客户端统计信息
type ClientStats struct {
	// URL: 客户端URL
	URL string `json:"url"`
	// Type: 客户端类型
	Type ClientType `json:"type"`
	// IsHealthy: 是否健康
	IsHealthy bool `json:"is_healthy"`
	// LastError: 最后一次错误
	LastError string `json:"last_error,omitempty"`
	// LastCheck: 最后一次检查时间
	LastCheck time.Time `json:"last_check"`
	// ConnectedAt: 连接时间
	ConnectedAt time.Time `json:"connected_at"`
	// RequestCount: 请求次数
	RequestCount int64 `json:"request_count"`
	// ErrorCount: 错误次数
	ErrorCount int64 `json:"error_count"`
	// Uptime: 运行时间
	Uptime time.Duration `json:"uptime"`
	// ErrorRate: 错误率
	ErrorRate float64 `json:"error_rate"`
}

// NewClient 创建新的以太坊客户端
func NewClient(config *ClientConfig, logger *logrus.Logger) (*Client, error) {
	if config == nil {
		return nil, fmt.Errorf("client config cannot be nil")
	}

	if config.URL == "" {
		return nil, fmt.Errorf("client URL cannot be empty")
	}

	// 设置默认值
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = time.Second
	}
	if config.MaxConcurrency == 0 {
		config.MaxConcurrency = 10
	}

	// 检测客户端类型
	if config.Type == "" {
		config.Type = detectClientType(config.URL)
	}

	if logger == nil {
		logger = logrus.New()
	}

	client := &Client{
		config: config,
		logger: logger,
	}

	// 建立连接
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to ethereum node: %w", err)
	}

	return client, nil
}

// Connect 建立与以太坊节点的连接
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 设置超时
	ctx, cancel := context.WithTimeout(context.Background(), c.config.Timeout)
	defer cancel()

	var err error

	// 创建RPC客户端
	// rpc.DialContext(ctx, c.config.URL) 创建一个RPC客户端
	c.rpcClient, err = rpc.DialContext(ctx, c.config.URL)
	if err != nil {
		c.lastError = err
		c.isHealthy = false
		return fmt.Errorf("failed to dial RPC: %w", err)
	}

	// 创建以太坊客户端
	c.ethClient = ethclient.NewClient(c.rpcClient)

	// 验证连接
	if err := c.validateConnection(ctx); err != nil {
		c.Close()
		c.lastError = err
		c.isHealthy = false
		return fmt.Errorf("connection validation failed: %w", err)
	}

	c.connectedAt = time.Now()
	c.isHealthy = true
	c.lastError = nil
	c.lastCheck = time.Now()

	c.logger.WithFields(logrus.Fields{
		"url":      c.config.URL,
		"type":     c.config.Type,
		"chain_id": c.config.ChainID,
	}).Info("Successfully connected to Ethereum node")

	return nil
}

// validateConnection 验证连接有效性
func (c *Client) validateConnection(ctx context.Context) error {
	// 获取网络ID
	networkID, err := c.ethClient.NetworkID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get network ID: %w", err)
	}

	// 如果配置了ChainID，验证是否匹配
	if c.config.ChainID != nil && networkID.Cmp(c.config.ChainID) != 0 {
		return fmt.Errorf("chain ID mismatch: expected %s, got %s",
			c.config.ChainID.String(), networkID.String())
	}

	// 如果没有配置ChainID，使用检测到的值
	if c.config.ChainID == nil {
		c.config.ChainID = networkID
	}

	// 获取最新区块号验证节点同步状态
	_, err = c.ethClient.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest block number: %w", err)
	}

	return nil
}

// Close 关闭客户端连接
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.rpcClient != nil {
		c.rpcClient.Close()
		c.rpcClient = nil
	}

	c.ethClient = nil
	c.isHealthy = false

	c.logger.WithField("url", c.config.URL).Info("Ethereum client connection closed")
}

// IsHealthy 检查客户端是否健康
func (c *Client) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isHealthy
}

// GetStats 获取客户端统计信息
func (c *Client) GetStats() ClientStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := ClientStats{
		URL:          c.config.URL,
		Type:         c.config.Type,
		IsHealthy:    c.isHealthy,
		LastCheck:    c.lastCheck,
		ConnectedAt:  c.connectedAt,
		RequestCount: c.requestCount,
		ErrorCount:   c.errorCount,
	}

	if c.lastError != nil {
		stats.LastError = c.lastError.Error()
	}

	if !c.connectedAt.IsZero() {
		stats.Uptime = time.Since(c.connectedAt)
	}

	if c.requestCount > 0 {
		stats.ErrorRate = float64(c.errorCount) / float64(c.requestCount)
	}

	return stats
}

// GetEthClient 获取以太坊客户端实例
func (c *Client) GetEthClient() *ethclient.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ethClient
}

// GetRPCClient 获取RPC客户端实例
func (c *Client) GetRPCClient() *rpc.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rpcClient
}

// GetConfig 获取客户端配置
func (c *Client) GetConfig() *ClientConfig {
	return c.config
}

// ExecuteWithRetry 执行带重试的操作
func (c *Client) ExecuteWithRetry(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt <= c.config.RetryAttempts; attempt++ {
		c.mu.Lock()
		c.requestCount++
		c.mu.Unlock()

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err
		c.mu.Lock()
		c.errorCount++
		c.lastError = err
		c.mu.Unlock()

		// 如果是最后一次尝试，直接返回错误
		if attempt == c.config.RetryAttempts {
			break
		}

		// 等待重试延迟
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.config.RetryDelay * time.Duration(attempt+1)):
			// 指数退避
		}

		c.logger.WithFields(logrus.Fields{
			"attempt":      attempt + 1,
			"max_attempts": c.config.RetryAttempts,
			"error":        err,
		}).Warn("Operation failed, retrying...")
	}

	c.mu.Lock()
	c.isHealthy = false
	c.mu.Unlock()

	return fmt.Errorf("operation failed after %d attempts: %w", c.config.RetryAttempts+1, lastErr)
}

// GetLatestBlock 获取最新区块
func (c *Client) GetLatestBlock(ctx context.Context) (*types.Block, error) {
	var block *types.Block

	err := c.ExecuteWithRetry(ctx, func() error {
		var err error
		block, err = c.ethClient.BlockByNumber(ctx, nil)
		return err
	})

	return block, err
}

// GetBlockByNumber 根据区块号获取区块
func (c *Client) GetBlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	var block *types.Block

	err := c.ExecuteWithRetry(ctx, func() error {
		var err error
		block, err = c.ethClient.BlockByNumber(ctx, number)
		return err
	})

	return block, err
}

// GetBlockByHash 根据区块哈希获取区块
func (c *Client) GetBlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	var block *types.Block

	err := c.ExecuteWithRetry(ctx, func() error {
		var err error
		block, err = c.ethClient.BlockByHash(ctx, hash)
		return err
	})

	return block, err
}

// GetTransactionByHash 根据交易哈希获取交易
func (c *Client) GetTransactionByHash(ctx context.Context, hash common.Hash) (*types.Transaction, bool, error) {
	var tx *types.Transaction
	var isPending bool

	err := c.ExecuteWithRetry(ctx, func() error {
		var err error
		tx, isPending, err = c.ethClient.TransactionByHash(ctx, hash)
		return err
	})

	return tx, isPending, err
}

// GetTransactionReceipt 获取交易收据
func (c *Client) GetTransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	var receipt *types.Receipt

	err := c.ExecuteWithRetry(ctx, func() error {
		var err error
		receipt, err = c.ethClient.TransactionReceipt(ctx, hash)
		return err
	})

	return receipt, err
}

// SubscribeNewHead 订阅新区块头
func (c *Client) SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error) {
	if c.config.Type != ClientTypeWebSocket {
		return nil, fmt.Errorf("subscription requires WebSocket connection")
	}

	var sub ethereum.Subscription

	err := c.ExecuteWithRetry(ctx, func() error {
		var err error
		sub, err = c.ethClient.SubscribeNewHead(ctx, ch)
		return err
	})

	return sub, err
}

// GetGasPrice 获取当前Gas价格
func (c *Client) GetGasPrice(ctx context.Context) (*big.Int, error) {
	var gasPrice *big.Int

	err := c.ExecuteWithRetry(ctx, func() error {
		var err error
		gasPrice, err = c.ethClient.SuggestGasPrice(ctx)
		return err
	})

	return gasPrice, err
}

// detectClientType 检测客户端类型
func detectClientType(url string) ClientType {
	if strings.HasPrefix(url, "ws://") || strings.HasPrefix(url, "wss://") {
		return ClientTypeWebSocket
	}
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return ClientTypeHTTP
	}
	if strings.Contains(url, ".ipc") || strings.HasPrefix(url, "/") {
		return ClientTypeIPC
	}
	return ClientTypeHTTP
}

// ValidateURL 验证URL格式
func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	// 对于IPC连接，直接检查路径
	if strings.Contains(rawURL, ".ipc") || strings.HasPrefix(rawURL, "/") {
		return nil
	}

	// 对于HTTP/WebSocket连接，解析URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if parsedURL.Scheme == "" {
		return fmt.Errorf("URL scheme is required")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("URL host is required")
	}

	return nil
}
