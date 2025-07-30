package main

import (
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"

	"simplied-blockchain-data-monitor-alert-go/pkg/ethereum"
)

// TestBlockHandler implements BlockEventHandler for testing
type TestBlockHandler struct {
	name         string
	blocksHandled int
}

func (h *TestBlockHandler) HandleBlock(event *ethereum.BlockEvent) error {
	h.blocksHandled++
	fmt.Printf("[%s] Received block #%d, hash: %s, matches: %d\n", 
		h.name, 
		event.Header.Number.Uint64(), 
		event.Header.Hash().Hex()[:10]+"...", 
		len(event.Matches))
	return nil
}

func (h *TestBlockHandler) HandleError(err error) {
	fmt.Printf("[%s] Error: %v\n", h.name, err)
}

func (h *TestBlockHandler) GetName() string {
	return h.name
}

// TestTxHandler implements TxEventHandler for testing
type TestTxHandler struct {
	name       string
	txsHandled int
}

func (h *TestTxHandler) HandleTransaction(event *ethereum.TxEvent) error {
	h.txsHandled++
	value := "N/A"
	gasPrice := "N/A"
	to := "N/A"
	
	if event.Transaction != nil {
		value = event.Transaction.Value().String()
		gasPrice = event.Transaction.GasPrice().String()
		if event.Transaction.To() != nil {
			to = event.Transaction.To().Hex()[:10] + "..."
		} else {
			to = "Contract Creation"
		}
	}
	
	fmt.Printf("[%s] Received tx %s, to: %s, value: %s wei, gasPrice: %s, matches: %d\n", 
		h.name, 
		event.Hash.Hex()[:10]+"...", 
		to,
		value,
		gasPrice,
		len(event.Matches))
	return nil
}

func (h *TestTxHandler) HandleError(err error) {
	fmt.Printf("[%s] Error: %v\n", h.name, err)
}

func (h *TestTxHandler) GetName() string {
	return h.name
}

func main() {
	// Set up logging
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	
	fmt.Println("🚀 Starting WebSocket Integration Test")
	fmt.Println("=====================================")
	
	// Test WebSocket URLs (using public endpoints)
	// Note: These are demo endpoints that may have rate limits
	wsURLs := []string{
		"wss://ethereum-rpc.publicnode.com",
		"wss://eth.merkle.io",
		"wss://eth-mainnet.g.alchemy.com/v2/demo",
	}
	
	// Test 1: WebSocket Connection Manager
	fmt.Println("\n📡 Testing WebSocket Connection Manager...")
	testWebSocketManager(wsURLs[0])
	
	// Test 2: Subscription Manager
	fmt.Println("\n📋 Testing Subscription Manager...")
	testSubscriptionManager(wsURLs[0])
	
	// Test 3: Event Filter
	fmt.Println("\n🔍 Testing Event Filter...")
	testEventFilter()
	
	// Test 4: Block Subscriber
	fmt.Println("\n📦 Testing Block Subscriber...")
	testBlockSubscriber(wsURLs[0])
	
	// Test 5: Transaction Subscriber
	fmt.Println("\n💸 Testing Transaction Subscriber...")
	testTransactionSubscriber(wsURLs[0])
	
	// Test 6: Full Integration Test
	fmt.Println("\n🔄 Testing Full Integration...")
	testFullIntegration(wsURLs)
	
	fmt.Println("\n✅ All tests completed!")
}

func testWebSocketManager(wsURL string) {
	fmt.Printf("Connecting to: %s\n", wsURL)
	
	config := ethereum.DefaultWSConfig()
	config.URL = wsURL
	config.PingInterval = 30 * time.Second
	
	wsManager := ethereum.NewWSConnectionManager(config)
	
	// Set event handlers
	wsManager.SetEventHandlers(
		func() { fmt.Println("✅ WebSocket connected") },
		func(err error) { fmt.Printf("❌ WebSocket disconnected: %v\n", err) },
		func(msg *ethereum.WSMessage) { 
			fmt.Printf("📨 Message received: method=%s\n", msg.Method) 
		},
		func(err error) { fmt.Printf("⚠️ WebSocket error: %v\n", err) },
	)
	
	// Connect
	if err := wsManager.Connect(); err != nil {
		fmt.Printf("❌ Failed to connect: %v\n", err)
		return
	}
	
	// Wait and check stats
	time.Sleep(3 * time.Second)
	stats := wsManager.GetStats()
	fmt.Printf("📊 Connection stats: connected_at=%s, state=%s\n", 
		stats.ConnectedAt.Format("15:04:05"), wsManager.GetState())
	
	// Disconnect
	wsManager.Disconnect()
	fmt.Println("✅ WebSocket manager test completed")
}

func testSubscriptionManager(wsURL string) {
	config := ethereum.DefaultWSConfig()
	config.URL = wsURL
	
	wsManager := ethereum.NewWSConnectionManager(config)
	subManager := ethereum.NewSubscriptionManager(wsManager)
	
	// Connect
	if err := wsManager.Connect(); err != nil {
		fmt.Printf("❌ Failed to connect: %v\n", err)
		return
	}
	
	time.Sleep(2 * time.Second)
	
	// Create a test subscription
	subConfig := ethereum.DefaultSubscriptionConfig(ethereum.SubscriptionTypeNewHeads)
	subConfig.BufferSize = 10
	
	subscription, err := subManager.Subscribe(subConfig)
	if err != nil {
		fmt.Printf("❌ Failed to create subscription: %v\n", err)
		wsManager.Disconnect()
		return
	}
	
	fmt.Printf("✅ Subscription created: %s\n", subscription.ID)
	
	// Wait for some data
	timeout := time.After(10 * time.Second)
	messageCount := 0
	
	for messageCount < 3 {
		select {
		case data := <-subscription.GetDataChannel():
			messageCount++
			if header, ok := data.(*types.Header); ok {
				fmt.Printf("📦 Block received: #%d\n", header.Number.Uint64())
			}
		case err := <-subscription.GetErrorChannel():
			fmt.Printf("⚠️ Subscription error: %v\n", err)
		case <-timeout:
			fmt.Println("⏰ Subscription test timeout")
			break
		}
	}
	
	// Get stats
	stats := subManager.GetStats()
	fmt.Printf("📊 Subscription stats: %+v\n", stats)
	
	// Clean up
	subscription.Close()
	subManager.Close()
	wsManager.Disconnect()
	fmt.Println("✅ Subscription manager test completed")
}

func testEventFilter() {
	filter := ethereum.NewEventFilter()
	
	// Create a test rule for large value transactions
	rule := &ethereum.FilterRule{
		ID:          "large_value_tx",
		Name:        "Large Value Transactions",
		Description: "Transactions with value > 10 ETH",
		Logic:       "AND",
		Enabled:     true,
		Priority:    1,
		Conditions: []*ethereum.FilterCondition{
			{
				Type:     ethereum.FilterTypeValue,
				Operator: ethereum.FilterOpGreaterThan,
				Value:    "10000000000000000000", // 10 ETH in wei
			},
		},
	}
	
	if err := filter.AddRule(rule); err != nil {
		fmt.Printf("❌ Failed to add filter rule: %v\n", err)
		return
	}
	
	// Create a test rule for specific contract
	contractRule := &ethereum.FilterRule{
		ID:          "uniswap_v3",
		Name:        "Uniswap V3 Router",
		Description: "Transactions to Uniswap V3 Router",
		Logic:       "OR",
		Enabled:     true,
		Priority:    2,
		Conditions: []*ethereum.FilterCondition{
			{
				Type:     ethereum.FilterTypeAddress,
				Operator: ethereum.FilterOpEqual,
				Value:    "0xE592427A0AEce92De3Edee1F18E0157C05861564", // Uniswap V3 Router
			},
		},
	}
	
	if err := filter.AddRule(contractRule); err != nil {
		fmt.Printf("❌ Failed to add contract rule: %v\n", err)
		return
	}
	
	// Test with a sample transaction
	testTx := types.NewTransaction(
		0,
		common.HexToAddress("0xE592427A0AEce92De3Edee1F18E0157C05861564"),
		big.NewInt(5000000000000000000), // 5 ETH
		21000,
		big.NewInt(20000000000), // 20 Gwei
		nil,
	)
	
	matches := filter.FilterTransaction(testTx)
	fmt.Printf("✅ Filter test: %d matches found\n", len(matches))
	
	for _, match := range matches {
		fmt.Printf("  - Rule: %s (priority: %d)\n", match.RuleName, match.Priority)
	}
	
	// Get filter stats
	stats := filter.GetStats()
	fmt.Printf("📊 Filter stats: %+v\n", stats)
	
	fmt.Println("✅ Event filter test completed")
}

func testBlockSubscriber(wsURL string) {
	config := ethereum.DefaultWSConfig()
	config.URL = wsURL
	
	wsManager := ethereum.NewWSConnectionManager(config)
	subManager := ethereum.NewSubscriptionManager(wsManager)
	eventFilter := ethereum.NewEventFilter()
	
	// Connect
	if err := wsManager.Connect(); err != nil {
		fmt.Printf("❌ Failed to connect: %v\n", err)
		return
	}
	
	time.Sleep(2 * time.Second)
	
	// Create block subscriber
	blockConfig := ethereum.DefaultBlockSubscriberConfig()
	blockConfig.BufferSize = 50
	blockConfig.EnableFiltering = false // Disable filtering for this test
	
	blockSubscriber := ethereum.NewBlockSubscriber(blockConfig, subManager, eventFilter)
	
	// Add test handler
	handler := &TestBlockHandler{name: "TestHandler"}
	blockSubscriber.AddHandler(handler)
	
	// Start subscriber
	if err := blockSubscriber.Start(); err != nil {
		fmt.Printf("❌ Failed to start block subscriber: %v\n", err)
		wsManager.Disconnect()
		return
	}
	
	fmt.Println("⏳ Waiting for blocks...")
	
	// Wait for some blocks
	timeout := time.After(30 * time.Second)
	
	for handler.blocksHandled < 3 {
		select {
		case event := <-blockSubscriber.GetBlockEvents():
			fmt.Printf("📦 Processed block event: #%d\n", event.Header.Number.Uint64())
		case err := <-blockSubscriber.GetErrorEvents():
			fmt.Printf("⚠️ Block subscriber error: %v\n", err)
		case <-timeout:
			fmt.Println("⏰ Block subscriber test timeout")
			break
		}
	}
	
	// Get stats
	stats := blockSubscriber.GetStats()
	fmt.Printf("📊 Block subscriber stats: received=%d, processed=%d, handlers=%d\n", 
		stats.BlocksReceived, stats.BlocksProcessed, stats.HandlerCount)
	
	// Clean up
	blockSubscriber.Stop()
	subManager.Close()
	wsManager.Disconnect()
	fmt.Println("✅ Block subscriber test completed")
}

func testTransactionSubscriber(wsURL string) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	
	config := ethereum.DefaultWSConfig()
	config.URL = wsURL
	
	wsManager := ethereum.NewWSConnectionManager(config)
	subManager := ethereum.NewSubscriptionManager(wsManager)
	eventFilter := ethereum.NewEventFilter()
	
	// We need a client pool for transaction fetching
	// For this test, we'll create a simple pool with one client
	poolConfig := &ethereum.PoolConfig{
		Clients: []*ethereum.ClientConfig{
			{
				URL:           "https://ethereum-rpc.publicnode.com",
				Type:          ethereum.ClientTypeHTTP,
				RetryAttempts: 3,
				Timeout:       10 * time.Second,
			},
		},
		MaxRetries:          3,
		HealthCheckInterval: 30 * time.Second,
		LoadBalanceStrategy: ethereum.StrategyRoundRobin,
	}
	
	clientPool, err := ethereum.NewClientPool(poolConfig, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create client pool")
	}
	
	// Connect WebSocket
	if err := wsManager.Connect(); err != nil {
		fmt.Printf("❌ Failed to connect WebSocket: %v\n", err)
		return
	}
	
	time.Sleep(2 * time.Second)
	
	// Create transaction subscriber
	txConfig := ethereum.DefaultTxSubscriberConfig()
	txConfig.BufferSize = 100
	txConfig.MaxConcurrency = 5
	txConfig.EnableFiltering = false // Disable filtering for this test
	
	txSubscriber := ethereum.NewTxSubscriber(txConfig, subManager, eventFilter, clientPool)
	
	// Add test handler
	handler := &TestTxHandler{name: "TestTxHandler"}
	txSubscriber.AddHandler(handler)
	
	// Start subscriber
	if err := txSubscriber.Start(); err != nil {
		fmt.Printf("❌ Failed to start transaction subscriber: %v\n", err)
		wsManager.Disconnect()
		return
	}
	
	fmt.Println("⏳ Waiting for transactions...")
	
	// Wait for some transactions
	timeout := time.After(30 * time.Second)
	
	for handler.txsHandled < 5 {
		select {
		case event := <-txSubscriber.GetTxEvents():
			fmt.Printf("💸 Processed tx event: %s\n", event.Hash.Hex()[:10]+"...")
		case err := <-txSubscriber.GetErrorEvents():
			fmt.Printf("⚠️ Transaction subscriber error: %v\n", err)
		case <-timeout:
			fmt.Println("⏰ Transaction subscriber test timeout")
			break
		}
	}
	
	// Get stats
	stats := txSubscriber.GetStats()
	fmt.Printf("📊 Transaction subscriber stats: received=%d, processed=%d, handlers=%d\n", 
		stats.TxReceived, stats.TxProcessed, stats.HandlerCount)
	
	// Clean up
	txSubscriber.Stop()
	subManager.Close()
	wsManager.Disconnect()
	fmt.Println("✅ Transaction subscriber test completed")
}

func testFullIntegration(wsURLs []string) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	
	fmt.Println("🔄 Starting full integration test with multiple endpoints...")
	
	// Create client pool with multiple endpoints
	poolConfig := &ethereum.PoolConfig{
		Clients: []*ethereum.ClientConfig{
			{
				URL:           "https://ethereum-rpc.publicnode.com",
				Type:          ethereum.ClientTypeHTTP,
				RetryAttempts: 3,
				Timeout:       10 * time.Second,
				Priority:      1,
			},
			{
				URL:           "https://eth.merkle.io",
				Type:          ethereum.ClientTypeHTTP,
				RetryAttempts: 3,
				Timeout:       10 * time.Second,
				Priority:      2,
			},
		},
		MaxRetries:          3,
		HealthCheckInterval: 30 * time.Second,
		LoadBalanceStrategy: ethereum.StrategyRoundRobin,
		MinHealthyClients:   1,
		EnableFailover:      true,
	}
	
	clientPool, err := ethereum.NewClientPool(poolConfig, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create client pool")
	}
	
	// Set up WebSocket connection to first endpoint
	config := ethereum.DefaultWSConfig()
	config.URL = wsURLs[0]
	config.ReconnectInterval = 5 * time.Second
	config.MaxReconnectAttempts = 3
	
	wsManager := ethereum.NewWSConnectionManager(config)
	subManager := ethereum.NewSubscriptionManager(wsManager)
	eventFilter := ethereum.NewEventFilter()
	
	// Add some filter rules
	largeValueRule := &ethereum.FilterRule{
		ID:          "large_value",
		Name:        "Large Value Transactions",
		Description: "Transactions > 1 ETH",
		Logic:       "AND",
		Enabled:     true,
		Priority:    1,
		Conditions: []*ethereum.FilterCondition{
			{
				Type:     ethereum.FilterTypeValue,
				Operator: ethereum.FilterOpGreaterThan,
				Value:    "1000000000000000000", // 1 ETH
			},
		},
	}
	eventFilter.AddRule(largeValueRule)
	
	// Connect
	if err := wsManager.Connect(); err != nil {
		fmt.Printf("❌ Failed to connect: %v\n", err)
		return
	}
	
	time.Sleep(2 * time.Second)
	
	// Create subscribers
	blockConfig := ethereum.DefaultBlockSubscriberConfig()
	blockConfig.EnableFiltering = true
	blockSubscriber := ethereum.NewBlockSubscriber(blockConfig, subManager, eventFilter)
	
	txConfig := ethereum.DefaultTxSubscriberConfig()
	txConfig.EnableFiltering = true
	txConfig.MaxConcurrency = 3
	txSubscriber := ethereum.NewTxSubscriber(txConfig, subManager, eventFilter, clientPool)
	
	// Add handlers
	blockHandler := &TestBlockHandler{name: "IntegrationBlockHandler"}
	txHandler := &TestTxHandler{name: "IntegrationTxHandler"}
	
	blockSubscriber.AddHandler(blockHandler)
	txSubscriber.AddHandler(txHandler)
	
	// Start subscribers
	if err := blockSubscriber.Start(); err != nil {
		fmt.Printf("❌ Failed to start block subscriber: %v\n", err)
		return
	}
	
	if err := txSubscriber.Start(); err != nil {
		fmt.Printf("❌ Failed to start transaction subscriber: %v\n", err)
		return
	}
	
	fmt.Println("⏳ Running integration test for 60 seconds...")
	
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Run for 60 seconds or until interrupted
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-sigChan:
			fmt.Println("\n🛑 Received interrupt signal, shutting down...")
			goto cleanup
		case <-timeout:
			fmt.Println("\n⏰ Integration test timeout reached")
			goto cleanup
		case <-ticker.C:
			// Print periodic stats
			blockStats := blockSubscriber.GetStats()
			txStats := txSubscriber.GetStats()
			wsStats := wsManager.GetStats()
			
			fmt.Printf("📊 Stats - Blocks: %d/%d, Txs: %d/%d, WS: %d msgs\n",
				blockStats.BlocksProcessed, blockStats.BlocksReceived,
				txStats.TxProcessed, txStats.TxReceived,
				wsStats.MessagesReceived)
		}
	}
	
cleanup:
	fmt.Println("🧹 Cleaning up...")
	
	// Stop subscribers
	blockSubscriber.Stop()
	txSubscriber.Stop()
	
	// Close connections
	subManager.Close()
	wsManager.Disconnect()
	
	// Final stats
	blockStats := blockSubscriber.GetStats()
	txStats := txSubscriber.GetStats()
	wsStats := wsManager.GetStats()
	filterStats := eventFilter.GetStats()
	
	fmt.Printf("\n📈 Final Integration Test Results:\n")
	fmt.Printf("  Block Subscriber: %d blocks processed, %d filtered\n", 
		blockStats.BlocksProcessed, blockStats.BlocksFiltered)
	fmt.Printf("  Transaction Subscriber: %d txs processed, %d filtered\n", 
		txStats.TxProcessed, txStats.TxFiltered)
	fmt.Printf("  WebSocket: %d messages received, %d reconnects\n", 
		wsStats.MessagesReceived, wsStats.ReconnectCount)
	fmt.Printf("  Event Filter: %+v\n", filterStats)
	
	fmt.Println("✅ Full integration test completed")
}
