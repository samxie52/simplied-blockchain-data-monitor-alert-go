package ethereum

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"
)

// TxSubscriberConfig holds configuration for transaction subscription
type TxSubscriberConfig struct {
	SubscriptionType  SubscriptionType `json:"subscription_type"` // newPendingTransactions or newPendingTransactionHashes
	AutoReconnect     bool             `json:"auto_reconnect"`
	BufferSize        int              `json:"buffer_size"`
	ProcessingTimeout time.Duration    `json:"processing_timeout"`
	MaxRetries        int              `json:"max_retries"`
	RetryInterval     time.Duration    `json:"retry_interval"`
	EnableFiltering   bool             `json:"enable_filtering"`
	BatchSize         int              `json:"batch_size"`
	FetchFullTx       bool             `json:"fetch_full_tx"`    // Fetch full transaction data for hashes
	MaxConcurrency    int              `json:"max_concurrency"` // Max concurrent transaction fetches
}

// DefaultTxSubscriberConfig returns default configuration
func DefaultTxSubscriberConfig() *TxSubscriberConfig {
	return &TxSubscriberConfig{
		SubscriptionType:  SubscriptionTypePendingTxs,
		AutoReconnect:     true,
		BufferSize:        2000,
		ProcessingTimeout: 30 * time.Second,
		MaxRetries:        3,
		RetryInterval:     5 * time.Second,
		EnableFiltering:   true,
		BatchSize:         20,
		FetchFullTx:       true,
		MaxConcurrency:    10,
	}
}

// TxEvent represents a transaction event with metadata
type TxEvent struct {
	Hash        common.Hash      `json:"hash"`
	Transaction *types.Transaction `json:"transaction,omitempty"`
	Matches     []*FilterMatch   `json:"matches,omitempty"`
	Timestamp   time.Time        `json:"timestamp"`
	Source      string           `json:"source"`
	Processed   bool             `json:"processed"`
	IsPending   bool             `json:"is_pending"`
}

// TxEventHandler defines the interface for handling transaction events
type TxEventHandler interface {
	HandleTransaction(event *TxEvent) error
	HandleError(err error)
	GetName() string
}

// TxSubscriber manages real-time transaction subscriptions
type TxSubscriber struct {
	config            *TxSubscriberConfig
	subscriptionMgr   *SubscriptionManager
	eventFilter       *EventFilter
	clientPool        *ClientPool
	subscription      *Subscription
	
	// Event handling
	handlers          []TxEventHandler
	handlersMutex     sync.RWMutex
	
	// Channels
	txEvents          chan *TxEvent
	processedEvents   chan *TxEvent
	errorEvents       chan error
	hashQueue         chan common.Hash
	
	// State management
	isRunning         bool
	runningMutex      sync.RWMutex
	
	// Context and cancellation
	ctx               context.Context
	cancel            context.CancelFunc
	
	// Concurrency control
	semaphore         chan struct{}
	
	// Statistics
	stats             TxSubscriberStats
	statsMutex        sync.RWMutex
	
	logger            *logrus.Entry
}

// TxSubscriberStats holds subscription statistics
type TxSubscriberStats struct {
	StartedAt             time.Time     `json:"started_at"`
	LastTxAt              time.Time     `json:"last_tx_at"`
	TxReceived            int64         `json:"tx_received"`
	TxProcessed           int64         `json:"tx_processed"`
	TxFiltered            int64         `json:"tx_filtered"`
	ProcessingErrors      int64         `json:"processing_errors"`
	AverageProcessTime    time.Duration `json:"average_process_time"`
	LastTxHash            string        `json:"last_tx_hash"`
	FilterMatches         int64         `json:"filter_matches"`
	HandlerCount          int           `json:"handler_count"`
	TotalUptime           time.Duration `json:"total_uptime"`
	HashesReceived        int64         `json:"hashes_received"`
	FullTxFetched         int64         `json:"full_tx_fetched"`
	FetchErrors           int64         `json:"fetch_errors"`
	ConcurrentFetches     int           `json:"concurrent_fetches"`
	QueueSize             int           `json:"queue_size"`
}

// NewTxSubscriber creates a new transaction subscriber
func NewTxSubscriber(config *TxSubscriberConfig, subscriptionMgr *SubscriptionManager, eventFilter *EventFilter, clientPool *ClientPool) *TxSubscriber {
	if config == nil {
		config = DefaultTxSubscriberConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &TxSubscriber{
		config:          config,
		subscriptionMgr: subscriptionMgr,
		eventFilter:     eventFilter,
		clientPool:      clientPool,
		txEvents:        make(chan *TxEvent, config.BufferSize),
		processedEvents: make(chan *TxEvent, config.BufferSize),
		errorEvents:     make(chan error, 100),
		hashQueue:       make(chan common.Hash, config.BufferSize),
		semaphore:       make(chan struct{}, config.MaxConcurrency),
		ctx:             ctx,
		cancel:          cancel,
		logger:          logrus.WithField("component", "tx_subscriber"),
	}
}

// AddHandler adds a transaction event handler
func (ts *TxSubscriber) AddHandler(handler TxEventHandler) {
	ts.handlersMutex.Lock()
	defer ts.handlersMutex.Unlock()
	
	ts.handlers = append(ts.handlers, handler)
	ts.logger.WithField("handler", handler.GetName()).Info("Transaction handler added")
}

// RemoveHandler removes a transaction event handler
func (ts *TxSubscriber) RemoveHandler(handlerName string) bool {
	ts.handlersMutex.Lock()
	defer ts.handlersMutex.Unlock()
	
	for i, handler := range ts.handlers {
		if handler.GetName() == handlerName {
			ts.handlers = append(ts.handlers[:i], ts.handlers[i+1:]...)
			ts.logger.WithField("handler", handlerName).Info("Transaction handler removed")
			return true
		}
	}
	
	return false
}

// Start starts the transaction subscription
func (ts *TxSubscriber) Start() error {
	ts.runningMutex.Lock()
	defer ts.runningMutex.Unlock()
	
	if ts.isRunning {
		return fmt.Errorf("transaction subscriber is already running")
	}
	
	ts.logger.Info("Starting transaction subscriber")
	
	// Create subscription
	subConfig := DefaultSubscriptionConfig(ts.config.SubscriptionType)
	subConfig.BufferSize = ts.config.BufferSize
	subConfig.AutoReconnect = ts.config.AutoReconnect
	subConfig.MaxRetries = ts.config.MaxRetries
	subConfig.RetryInterval = ts.config.RetryInterval
	
	subscription, err := ts.subscriptionMgr.Subscribe(subConfig)
	if err != nil {
		return fmt.Errorf("failed to create transaction subscription: %v", err)
	}
	
	ts.subscription = subscription
	ts.isRunning = true
	ts.stats.StartedAt = time.Now()
	
	// Start processing goroutines
	go ts.subscriptionProcessor()
	go ts.eventProcessor()
	go ts.errorProcessor()
	
	// Start hash fetcher if needed
	if ts.config.FetchFullTx && ts.config.SubscriptionType == SubscriptionTypeNewPendingTxs {
		for i := 0; i < ts.config.MaxConcurrency; i++ {
			go ts.hashFetcher()
		}
	}
	
	ts.logger.Info("Transaction subscriber started successfully")
	return nil
}

// Stop stops the transaction subscription
func (ts *TxSubscriber) Stop() error {
	ts.runningMutex.Lock()
	defer ts.runningMutex.Unlock()
	
	if !ts.isRunning {
		return nil
	}
	
	ts.logger.Info("Stopping transaction subscriber")
	
	ts.isRunning = false
	
	// Close subscription
	if ts.subscription != nil {
		if err := ts.subscription.Close(); err != nil {
			ts.logger.WithError(err).Warn("Error closing subscription")
		}
	}
	
	// Cancel context
	ts.cancel()
	
	// Update stats
	ts.statsMutex.Lock()
	if !ts.stats.StartedAt.IsZero() {
		ts.stats.TotalUptime += time.Since(ts.stats.StartedAt)
	}
	ts.statsMutex.Unlock()
	
	ts.logger.Info("Transaction subscriber stopped")
	return nil
}

// IsRunning returns true if the subscriber is running
func (ts *TxSubscriber) IsRunning() bool {
	ts.runningMutex.RLock()
	defer ts.runningMutex.RUnlock()
	return ts.isRunning
}

// GetStats returns subscription statistics
func (ts *TxSubscriber) GetStats() TxSubscriberStats {
	ts.statsMutex.RLock()
	defer ts.statsMutex.RUnlock()
	
	stats := ts.stats
	
	ts.handlersMutex.RLock()
	stats.HandlerCount = len(ts.handlers)
	ts.handlersMutex.RUnlock()
	
	if ts.isRunning && !ts.stats.StartedAt.IsZero() {
		stats.TotalUptime = ts.stats.TotalUptime + time.Since(ts.stats.StartedAt)
	}
	
	stats.QueueSize = len(ts.hashQueue)
	stats.ConcurrentFetches = ts.config.MaxConcurrency - len(ts.semaphore)
	
	return stats
}

// GetTxEvents returns the channel for processed transaction events
func (ts *TxSubscriber) GetTxEvents() <-chan *TxEvent {
	return ts.processedEvents
}

// GetErrorEvents returns the channel for error events
func (ts *TxSubscriber) GetErrorEvents() <-chan error {
	return ts.errorEvents
}

// subscriptionProcessor processes incoming subscription data
func (ts *TxSubscriber) subscriptionProcessor() {
	defer ts.logger.Info("Subscription processor stopped")
	
	for {
		select {
		case <-ts.ctx.Done():
			return
		case data := <-ts.subscription.GetDataChannel():
			ts.processSubscriptionData(data)
		case err := <-ts.subscription.GetErrorChannel():
			ts.logger.WithError(err).Error("Subscription error")
			select {
			case ts.errorEvents <- err:
			default:
				ts.logger.Warn("Error channel full, dropping error")
			}
		}
	}
}

// processSubscriptionData processes incoming subscription data
func (ts *TxSubscriber) processSubscriptionData(data interface{}) {
	switch ts.config.SubscriptionType {
	case SubscriptionTypeNewPendingTxs:
		// Data is transaction hash
		if hash, ok := data.(common.Hash); ok {
			ts.processTransactionHash(hash)
		} else if hashStr, ok := data.(string); ok {
			hash := common.HexToHash(hashStr)
			ts.processTransactionHash(hash)
		} else {
			ts.logger.WithField("type", fmt.Sprintf("%T", data)).Warn("Unexpected hash data type")
		}
	case SubscriptionTypePendingTxs:
		// Data is full transaction
		if tx, ok := data.(*types.Transaction); ok {
			ts.processTransaction(tx.Hash(), tx)
		} else {
			ts.logger.WithField("type", fmt.Sprintf("%T", data)).Warn("Unexpected transaction data type")
		}
	default:
		ts.logger.WithField("type", ts.config.SubscriptionType).Warn("Unsupported subscription type")
	}
}

// processTransactionHash processes a transaction hash
func (ts *TxSubscriber) processTransactionHash(hash common.Hash) {
	ts.statsMutex.Lock()
	ts.stats.HashesReceived++
	ts.statsMutex.Unlock()
	
	if ts.config.FetchFullTx {
		// Queue hash for fetching
		select {
		case ts.hashQueue <- hash:
		default:
			ts.logger.Warn("Hash queue full, dropping hash")
		}
	} else {
		// Process hash directly
		ts.processTransaction(hash, nil)
	}
}

// processTransaction processes a transaction
func (ts *TxSubscriber) processTransaction(hash common.Hash, tx *types.Transaction) {
	startTime := time.Now()
	
	// Update stats
	ts.statsMutex.Lock()
	ts.stats.TxReceived++
	ts.stats.LastTxAt = time.Now()
	ts.stats.LastTxHash = hash.Hex()
	ts.statsMutex.Unlock()
	
	// Create transaction event
	event := &TxEvent{
		Hash:        hash,
		Transaction: tx,
		Timestamp:   time.Now(),
		Source:      "subscription",
		Processed:   false,
		IsPending:   true,
	}
	
	// Apply filters if enabled and we have full transaction data
	if ts.config.EnableFiltering && ts.eventFilter != nil && tx != nil {
		matches := ts.eventFilter.FilterTransaction(tx)
		if len(matches) > 0 {
			event.Matches = matches
			ts.statsMutex.Lock()
			ts.stats.FilterMatches += int64(len(matches))
			ts.statsMutex.Unlock()
		} else {
			ts.statsMutex.Lock()
			ts.stats.TxFiltered++
			ts.statsMutex.Unlock()
			
			// Skip processing if no matches and filtering is strict
			return
		}
	}
	
	// Send to processing channel
	select {
	case ts.txEvents <- event:
	default:
		ts.logger.Warn("Transaction events channel full, dropping transaction")
		ts.statsMutex.Lock()
		ts.stats.ProcessingErrors++
		ts.statsMutex.Unlock()
	}
	
	// Update processing time
	processingTime := time.Since(startTime)
	ts.statsMutex.Lock()
	if ts.stats.AverageProcessTime == 0 {
		ts.stats.AverageProcessTime = processingTime
	} else {
		ts.stats.AverageProcessTime = (ts.stats.AverageProcessTime + processingTime) / 2
	}
	ts.statsMutex.Unlock()
}

// hashFetcher fetches full transaction data for hashes
func (ts *TxSubscriber) hashFetcher() {
	defer ts.logger.Info("Hash fetcher stopped")
	
	for {
		select {
		case <-ts.ctx.Done():
			return
		case hash := <-ts.hashQueue:
			ts.fetchAndProcessTransaction(hash)
		}
	}
}

// fetchAndProcessTransaction fetches full transaction data and processes it
func (ts *TxSubscriber) fetchAndProcessTransaction(hash common.Hash) {
	// Acquire semaphore
	select {
	case ts.semaphore <- struct{}{}:
		defer func() { <-ts.semaphore }()
	case <-ts.ctx.Done():
		return
	}
	
	// Get client from pool
	client, err := ts.clientPool.GetClient()
	if err != nil {
		ts.logger.WithError(err).Error("No available client for transaction fetch")
		ts.statsMutex.Lock()
		ts.stats.FetchErrors++
		ts.statsMutex.Unlock()
		return
	}
	
	// Fetch transaction
	tx, isPending, err := client.GetTransactionByHash(ts.ctx, hash)
	if err != nil {
		ts.logger.WithError(err).WithField("hash", hash.Hex()).Warn("Failed to fetch transaction")
		ts.statsMutex.Lock()
		ts.stats.FetchErrors++
		ts.statsMutex.Unlock()
		return
	}
	
	ts.statsMutex.Lock()
	ts.stats.FullTxFetched++
	ts.statsMutex.Unlock()
	
	// Process the full transaction
	event := &TxEvent{
		Hash:        hash,
		Transaction: tx,
		Timestamp:   time.Now(),
		Source:      "fetch",
		Processed:   false,
		IsPending:   isPending,
	}
	
	// Apply filters
	if ts.config.EnableFiltering && ts.eventFilter != nil {
		matches := ts.eventFilter.FilterTransaction(tx)
		if len(matches) > 0 {
			event.Matches = matches
			ts.statsMutex.Lock()
			ts.stats.FilterMatches += int64(len(matches))
			ts.statsMutex.Unlock()
		} else {
			ts.statsMutex.Lock()
			ts.stats.TxFiltered++
			ts.statsMutex.Unlock()
			return
		}
	}
	
	// Send to processing channel
	select {
	case ts.txEvents <- event:
	default:
		ts.logger.Warn("Transaction events channel full, dropping fetched transaction")
	}
}

// eventProcessor processes transaction events through handlers
func (ts *TxSubscriber) eventProcessor() {
	defer ts.logger.Info("Event processor stopped")
	
	for {
		select {
		case <-ts.ctx.Done():
			return
		case event := <-ts.txEvents:
			ts.processEvent(event)
		}
	}
}

// processEvent processes a single transaction event
func (ts *TxSubscriber) processEvent(event *TxEvent) {
	defer func() {
		if r := recover(); r != nil {
			ts.logger.WithField("panic", r).Error("Panic in event processing")
			ts.statsMutex.Lock()
			ts.stats.ProcessingErrors++
			ts.statsMutex.Unlock()
		}
	}()
	
	// Process with timeout
	ctx, cancel := context.WithTimeout(ts.ctx, ts.config.ProcessingTimeout)
	defer cancel()
	
	done := make(chan bool, 1)
	
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ts.logger.WithField("panic", r).Error("Panic in handler execution")
			}
			done <- true
		}()
		
		ts.executeHandlers(event)
	}()
	
	select {
	case <-ctx.Done():
		ts.logger.WithField("hash", event.Hash.Hex()).Warn("Event processing timeout")
		ts.statsMutex.Lock()
		ts.stats.ProcessingErrors++
		ts.statsMutex.Unlock()
	case <-done:
		event.Processed = true
		ts.statsMutex.Lock()
		ts.stats.TxProcessed++
		ts.statsMutex.Unlock()
		
		// Send to processed events channel
		select {
		case ts.processedEvents <- event:
		default:
			ts.logger.Warn("Processed events channel full, dropping event")
		}
	}
}

// executeHandlers executes all registered handlers for an event
func (ts *TxSubscriber) executeHandlers(event *TxEvent) {
	ts.handlersMutex.RLock()
	handlers := make([]TxEventHandler, len(ts.handlers))
	copy(handlers, ts.handlers)
	ts.handlersMutex.RUnlock()
	
	for _, handler := range handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					ts.logger.WithFields(logrus.Fields{
						"handler": handler.GetName(),
						"panic":   r,
					}).Error("Handler panic")
				}
			}()
			
			if err := handler.HandleTransaction(event); err != nil {
				ts.logger.WithFields(logrus.Fields{
					"handler": handler.GetName(),
					"error":   err,
				}).Error("Handler error")
				
				handler.HandleError(err)
				
				select {
				case ts.errorEvents <- err:
				default:
				}
			}
		}()
	}
}

// errorProcessor processes error events
func (ts *TxSubscriber) errorProcessor() {
	defer ts.logger.Info("Error processor stopped")
	
	for {
		select {
		case <-ts.ctx.Done():
			return
		case err := <-ts.errorEvents:
			ts.handleError(err)
		}
	}
}

// handleError handles error events
func (ts *TxSubscriber) handleError(err error) {
	ts.logger.WithError(err).Error("Processing error event")
	
	// Notify all handlers of the error
	ts.handlersMutex.RLock()
	handlers := make([]TxEventHandler, len(ts.handlers))
	copy(handlers, ts.handlers)
	ts.handlersMutex.RUnlock()
	
	for _, handler := range handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					ts.logger.WithFields(logrus.Fields{
						"handler": handler.GetName(),
						"panic":   r,
					}).Error("Handler panic during error handling")
				}
			}()
			
			handler.HandleError(err)
		}()
	}
}

// GetSubscription returns the underlying subscription
func (ts *TxSubscriber) GetSubscription() *Subscription {
	return ts.subscription
}

// Restart restarts the transaction subscriber
func (ts *TxSubscriber) Restart() error {
	ts.logger.Info("Restarting transaction subscriber")
	
	if err := ts.Stop(); err != nil {
		return fmt.Errorf("failed to stop subscriber: %v", err)
	}
	
	// Wait a bit before restarting
	time.Sleep(2 * time.Second)
	
	if err := ts.Start(); err != nil {
		return fmt.Errorf("failed to start subscriber: %v", err)
	}
	
	return nil
}

// SetFilter sets the event filter
func (ts *TxSubscriber) SetFilter(filter *EventFilter) {
	ts.eventFilter = filter
	ts.logger.Info("Event filter updated")
}

// GetHandlers returns a copy of all registered handlers
func (ts *TxSubscriber) GetHandlers() []TxEventHandler {
	ts.handlersMutex.RLock()
	defer ts.handlersMutex.RUnlock()
	
	handlers := make([]TxEventHandler, len(ts.handlers))
	copy(handlers, ts.handlers)
	return handlers
}
