package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"simplied-blockchain-data-monitor-alert-go/pkg/ethereum"

	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	fmt.Println("🚀 Testing Ethereum Client Integration...")
	fmt.Println("==================================================")

	// 测试配置 - 使用公共测试网络
	clientConfigs := []*ethereum.ClientConfig{
		{
			URL:            "https://ethereum-rpc.publicnode.com",
			Type:           ethereum.ClientTypeHTTP,
			Timeout:        30 * time.Second,
			RetryAttempts:  3,
			RetryDelay:     time.Second,
			MaxConcurrency: 10,
			Priority:       1,
			NetworkName:    "mainnet",
		},
		{
			URL:            "https://ethereum-rpc.publicnode.com",
			Type:           ethereum.ClientTypeHTTP,
			Timeout:        30 * time.Second,
			RetryAttempts:  3,
			RetryDelay:     time.Second,
			MaxConcurrency: 10,
			Priority:       1,
			NetworkName:    "mainnet",
		},
	}

	poolConfig := &ethereum.PoolConfig{
		Clients:             clientConfigs,
		LoadBalanceStrategy: ethereum.StrategyRoundRobin,
		HealthCheckInterval: 30 * time.Second,
		MaxRetries:          3,
		RetryDelay:          time.Second,
		MinHealthyClients:   1,
		EnableFailover:      true,
	}

	// 创建连接池
	fmt.Println("📡 Creating Ethereum client pool...")
	pool, err := ethereum.NewClientPool(poolConfig, logger)
	if err != nil {
		log.Fatal("Failed to create client pool:", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 测试基础连接
	fmt.Println("\n🔍 Testing basic connectivity...")
	testBasicConnectivity(ctx, pool)

	// 测试区块服务
	fmt.Println("\n📦 Testing block service...")
	testBlockService(ctx, pool, logger)

	// 测试交易服务
	fmt.Println("\n💸 Testing transaction service...")
	testTransactionService(ctx, pool, logger)

	// 测试Gas服务
	fmt.Println("\n⛽ Testing gas service...")
	testGasService(ctx, pool, logger)

	// 测试连接池统计
	fmt.Println("\n📊 Testing pool statistics...")
	testPoolStatistics(pool)

	fmt.Println("\n✅ All tests completed successfully!")
	fmt.Println("🎉 Ethereum client integration is working properly!")
}

func testBasicConnectivity(ctx context.Context, pool *ethereum.ClientPool) {
	// 获取最新区块
	block, err := pool.GetLatestBlock(ctx)
	if err != nil {
		log.Printf("❌ Failed to get latest block: %v", err)
		return
	}

	fmt.Printf("✅ Latest block number: %d\n", block.NumberU64())
	fmt.Printf("✅ Block hash: %s\n", block.Hash().Hex())
	fmt.Printf("✅ Block timestamp: %s\n", time.Unix(int64(block.Time()), 0).Format(time.RFC3339))

	// 获取Gas价格
	gasPrice, err := pool.GetGasPrice(ctx)
	if err != nil {
		log.Printf("❌ Failed to get gas price: %v", err)
		return
	}

	fmt.Printf("✅ Current gas price: %s Gwei\n", new(big.Int).Div(gasPrice, big.NewInt(1000000000)))
}

func testBlockService(ctx context.Context, pool *ethereum.ClientPool, logger *logrus.Logger) {
	blockService := ethereum.NewBlockService(pool, logger)

	// 获取最新区块号
	latestNumber, err := blockService.GetLatestBlockNumber(ctx)
	if err != nil {
		log.Printf("❌ Failed to get latest block number: %v", err)
		return
	}

	fmt.Printf("✅ Latest block number from service: %s\n", latestNumber.String())

	// 获取特定区块
	targetBlock := new(big.Int).Sub(latestNumber, big.NewInt(1)) // 前一个区块
	block, err := blockService.GetBlockByNumber(ctx, targetBlock)
	if err != nil {
		log.Printf("❌ Failed to get block by number: %v", err)
		return
	}

	fmt.Printf("✅ Retrieved block %d with %d transactions\n",
		block.NumberU64(), len(block.Transactions()))

	// 测试小范围区块获取
	from := new(big.Int).Sub(latestNumber, big.NewInt(3))
	to := new(big.Int).Sub(latestNumber, big.NewInt(1))

	options := &ethereum.BlockSyncOptions{
		BatchSize:       2,
		MaxConcurrency:  2,
		RetryAttempts:   2,
		RetryDelay:      time.Second,
		VerifyIntegrity: true,
	}

	blocks, err := blockService.GetBlockRange(ctx, from, to, options)
	if err != nil {
		log.Printf("❌ Failed to get block range: %v", err)
		return
	}

	fmt.Printf("✅ Retrieved %d blocks in range [%s, %s]\n",
		len(blocks), from.String(), to.String())
}

func testTransactionService(ctx context.Context, pool *ethereum.ClientPool, logger *logrus.Logger) {
	txService := ethereum.NewTransactionService(pool, logger)

	// 获取最新区块
	block, err := pool.GetLatestBlock(ctx)
	if err != nil {
		log.Printf("❌ Failed to get latest block: %v", err)
		return
	}

	if len(block.Transactions()) == 0 {
		fmt.Println("ℹ️  No transactions in latest block, skipping transaction tests")
		return
	}

	// 获取第一个交易
	firstTx := block.Transactions()[0]
	fmt.Printf("✅ Found transaction: %s\n", firstTx.Hash().Hex())

	// 获取交易详情
	txWithReceipt, err := txService.GetTransactionByHash(ctx, firstTx.Hash())
	if err != nil {
		log.Printf("❌ Failed to get transaction by hash: %v", err)
		return
	}

	fmt.Printf("✅ Transaction value: %s ETH\n",
		new(big.Int).Div(txWithReceipt.Transaction.Value(), big.NewInt(1000000000000000000)))

	if txWithReceipt.Receipt != nil {
		fmt.Printf("✅ Transaction gas used: %d\n", txWithReceipt.Receipt.GasUsed)
		fmt.Printf("✅ Transaction status: %d\n", txWithReceipt.Receipt.Status)
	}

	// 分析Gas使用情况
	gasAnalysis := txService.AnalyzeTransactionGas(txWithReceipt)
	if gasAnalysis != nil {
		fmt.Printf("✅ Gas analysis completed: %d fields\n", len(gasAnalysis))
	}
}

func testGasService(ctx context.Context, pool *ethereum.ClientPool, logger *logrus.Logger) {
	gasService := ethereum.NewGasService(pool, logger)

	// 获取当前Gas价格信息
	gasPriceInfo, err := gasService.GetGasPriceInfo(ctx)
	if err != nil {
		log.Printf("❌ Failed to get gas price info: %v", err)
		return
	}

	fmt.Printf("✅ Standard gas price: %s Gwei\n",
		new(big.Int).Div(gasPriceInfo.Standard, big.NewInt(1000000000)))
	fmt.Printf("✅ Fast gas price: %s Gwei\n",
		new(big.Int).Div(gasPriceInfo.Fast, big.NewInt(1000000000)))
	fmt.Printf("✅ Instant gas price: %s Gwei\n",
		new(big.Int).Div(gasPriceInfo.Instant, big.NewInt(1000000000)))

	if gasPriceInfo.BaseFee != nil {
		fmt.Printf("✅ Base fee: %s Gwei\n",
			new(big.Int).Div(gasPriceInfo.BaseFee, big.NewInt(1000000000)))
	}

	// 分析最近区块的Gas统计
	stats, err := gasService.AnalyzeRecentBlocks(ctx, 5)
	if err != nil {
		log.Printf("❌ Failed to analyze recent blocks: %v", err)
		return
	}

	fmt.Printf("✅ Gas statistics from %d samples:\n", stats.Samples)
	fmt.Printf("   Min: %s Gwei\n", new(big.Int).Div(stats.Min, big.NewInt(1000000000)))
	fmt.Printf("   Max: %s Gwei\n", new(big.Int).Div(stats.Max, big.NewInt(1000000000)))
	fmt.Printf("   Average: %s Gwei\n", new(big.Int).Div(stats.Average, big.NewInt(1000000000)))
	fmt.Printf("   Median: %s Gwei\n", new(big.Int).Div(stats.Median, big.NewInt(1000000000)))

	// 获取最优Gas价格
	optimalPrice, err := gasService.GetOptimalGasPrice(ctx, "fast")
	if err != nil {
		log.Printf("❌ Failed to get optimal gas price: %v", err)
		return
	}

	fmt.Printf("✅ Optimal gas price (fast): %s Gwei\n",
		new(big.Int).Div(optimalPrice, big.NewInt(1000000000)))
}

func testPoolStatistics(pool *ethereum.ClientPool) {
	stats := pool.GetStats()

	fmt.Printf("✅ Pool statistics:\n")
	fmt.Printf("   Total clients: %d\n", stats.TotalClients)
	fmt.Printf("   Healthy clients: %d\n", stats.HealthyClients)
	fmt.Printf("   Total requests: %d\n", stats.TotalRequests)
	fmt.Printf("   Failed requests: %d\n", stats.FailedRequests)
	fmt.Printf("   Last update: %s\n", stats.LastUpdate.Format(time.RFC3339))

	fmt.Println("✅ Client details:")
	for url, clientStats := range stats.ClientStats {
		fmt.Printf("   %s: healthy=%t, requests=%d, errors=%d, error_rate=%.2f%%\n",
			url, clientStats.IsHealthy, clientStats.RequestCount,
			clientStats.ErrorCount, clientStats.ErrorRate*100)
	}
}
