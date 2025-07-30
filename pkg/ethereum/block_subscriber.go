package ethereum

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"
)

// BlockSubscriberConfig holds configuration for block subscription
type BlockSubscriberConfig struct {
	AutoReconnect     bool          `json:"auto_reconnect"`
	BufferSize        int           `json:"buffer_size"`
	ProcessingTimeout time.Duration `json:"processing_timeout"`
	MaxRetries        int           `json:"max_retries"`
	RetryInterval     time.Duration `json:"retry_interval"`
	EnableFiltering   bool          `json:"enable_filtering"`
	BatchSize         int           `json:"batch_size"`
}

// DefaultBlockSubscriberConfig returns default configuration
func DefaultBlockSubscriberConfig() *BlockSubscriberConfig {
	return &BlockSubscriberConfig{
		AutoReconnect:     true,
		BufferSize:        1000,
		ProcessingTimeout: 30 * time.Second,
		MaxRetries:        3,
		RetryInterval:     5 * time.Second,
		EnableFiltering:   true,
		BatchSize:         10,
	}
}

// BlockEvent represents a block event with metadata
type BlockEvent struct {
	Header    *types.Header    `json:"header"`
	Matches   []*FilterMatch   `json:"matches,omitempty"`
	Timestamp time.Time        `json:"timestamp"`
	Source    string           `json:"source"`
	Processed bool             `json:"processed"`
}

// BlockEventHandler defines the interface for handling block events
type BlockEventHandler interface {
	HandleBlock(event *BlockEvent) error
	HandleError(err error)
	GetName() string
}

// BlockSubscriber manages real-time block subscriptions
type BlockSubscriber struct {
	config            *BlockSubscriberConfig
	subscriptionMgr   *SubscriptionManager
	eventFilter       *EventFilter
	subscription      *Subscription
	
	// Event handling
	handlers          []BlockEventHandler
	handlersMutex     sync.RWMutex
	
	// Channels
	blockEvents       chan *BlockEvent
	processedEvents   chan *BlockEvent
	errorEvents       chan error
	
	// State management
	isRunning         bool
	runningMutex      sync.RWMutex
	
	// Context and cancellation
	ctx               context.Context
	cancel            context.CancelFunc
	
	// Statistics
	stats             BlockSubscriberStats
	statsMutex        sync.RWMutex
	
	logger            *logrus.Entry
}

// BlockSubscriberStats holds subscription statistics
type BlockSubscriberStats struct {
	StartedAt           time.Time     `json:"started_at"`
	LastBlockAt         time.Time     `json:"last_block_at"`
	BlocksReceived      int64         `json:"blocks_received"`
	BlocksProcessed     int64         `json:"blocks_processed"`
	BlocksFiltered      int64         `json:"blocks_filtered"`
	ProcessingErrors    int64         `json:"processing_errors"`
	AverageProcessTime  time.Duration `json:"average_process_time"`
	LastBlockNumber     uint64        `json:"last_block_number"`
	LastBlockHash       string        `json:"last_block_hash"`
	FilterMatches       int64         `json:"filter_matches"`
	HandlerCount        int           `json:"handler_count"`
	TotalUptime         time.Duration `json:"total_uptime"`
}

// NewBlockSubscriber creates a new block subscriber
func NewBlockSubscriber(config *BlockSubscriberConfig, subscriptionMgr *SubscriptionManager, eventFilter *EventFilter) *BlockSubscriber {
	if config == nil {
		config = DefaultBlockSubscriberConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &BlockSubscriber{
		config:          config,
		subscriptionMgr: subscriptionMgr,
		eventFilter:     eventFilter,
		blockEvents:     make(chan *BlockEvent, config.BufferSize),
		processedEvents: make(chan *BlockEvent, config.BufferSize),
		errorEvents:     make(chan error, 100),
		ctx:             ctx,
		cancel:          cancel,
		logger:          logrus.WithField("component", "block_subscriber"),
	}
}

// AddHandler adds a block event handler
func (bs *BlockSubscriber) AddHandler(handler BlockEventHandler) {
	bs.handlersMutex.Lock()
	defer bs.handlersMutex.Unlock()
	
	bs.handlers = append(bs.handlers, handler)
	bs.logger.WithField("handler", handler.GetName()).Info("Block handler added")
}

// RemoveHandler removes a block event handler
func (bs *BlockSubscriber) RemoveHandler(handlerName string) bool {
	bs.handlersMutex.Lock()
	defer bs.handlersMutex.Unlock()
	
	for i, handler := range bs.handlers {
		if handler.GetName() == handlerName {
			bs.handlers = append(bs.handlers[:i], bs.handlers[i+1:]...)
			bs.logger.WithField("handler", handlerName).Info("Block handler removed")
			return true
		}
	}
	
	return false
}

// Start starts the block subscription
func (bs *BlockSubscriber) Start() error {
	bs.runningMutex.Lock()
	defer bs.runningMutex.Unlock()
	
	if bs.isRunning {
		return fmt.Errorf("block subscriber is already running")
	}
	
	bs.logger.Info("Starting block subscriber")
	
	// Create subscription
	subConfig := DefaultSubscriptionConfig(SubscriptionTypeNewHeads)
	subConfig.BufferSize = bs.config.BufferSize
	subConfig.AutoReconnect = bs.config.AutoReconnect
	subConfig.MaxRetries = bs.config.MaxRetries
	subConfig.RetryInterval = bs.config.RetryInterval
	
	subscription, err := bs.subscriptionMgr.Subscribe(subConfig)
	if err != nil {
		return fmt.Errorf("failed to create block subscription: %v", err)
	}
	
	bs.subscription = subscription
	bs.isRunning = true
	bs.stats.StartedAt = time.Now()
	
	// Start processing goroutines
	go bs.subscriptionProcessor()
	go bs.eventProcessor()
	go bs.errorProcessor()
	
	bs.logger.Info("Block subscriber started successfully")
	return nil
}

// Stop stops the block subscription
func (bs *BlockSubscriber) Stop() error {
	bs.runningMutex.Lock()
	defer bs.runningMutex.Unlock()
	
	if !bs.isRunning {
		return nil
	}
	
	bs.logger.Info("Stopping block subscriber")
	
	bs.isRunning = false
	
	// Close subscription
	if bs.subscription != nil {
		if err := bs.subscription.Close(); err != nil {
			bs.logger.WithError(err).Warn("Error closing subscription")
		}
	}
	
	// Cancel context
	bs.cancel()
	
	// Update stats
	bs.statsMutex.Lock()
	if !bs.stats.StartedAt.IsZero() {
		bs.stats.TotalUptime += time.Since(bs.stats.StartedAt)
	}
	bs.statsMutex.Unlock()
	
	bs.logger.Info("Block subscriber stopped")
	return nil
}

// IsRunning returns true if the subscriber is running
func (bs *BlockSubscriber) IsRunning() bool {
	bs.runningMutex.RLock()
	defer bs.runningMutex.RUnlock()
	return bs.isRunning
}

// GetStats returns subscription statistics
func (bs *BlockSubscriber) GetStats() BlockSubscriberStats {
	bs.statsMutex.RLock()
	defer bs.statsMutex.RUnlock()
	
	stats := bs.stats
	
	bs.handlersMutex.RLock()
	stats.HandlerCount = len(bs.handlers)
	bs.handlersMutex.RUnlock()
	
	if bs.isRunning && !bs.stats.StartedAt.IsZero() {
		stats.TotalUptime = bs.stats.TotalUptime + time.Since(bs.stats.StartedAt)
	}
	
	return stats
}

// GetBlockEvents returns the channel for processed block events
func (bs *BlockSubscriber) GetBlockEvents() <-chan *BlockEvent {
	return bs.processedEvents
}

// GetErrorEvents returns the channel for error events
func (bs *BlockSubscriber) GetErrorEvents() <-chan error {
	return bs.errorEvents
}

// subscriptionProcessor processes incoming subscription data
func (bs *BlockSubscriber) subscriptionProcessor() {
	defer bs.logger.Info("Subscription processor stopped")
	
	for {
		select {
		case <-bs.ctx.Done():
			return
		case data := <-bs.subscription.GetDataChannel():
			if header, ok := data.(*types.Header); ok {
				bs.processBlockHeader(header)
			} else {
				bs.logger.WithField("type", fmt.Sprintf("%T", data)).Warn("Unexpected data type received")
			}
		case err := <-bs.subscription.GetErrorChannel():
			bs.logger.WithError(err).Error("Subscription error")
			select {
			case bs.errorEvents <- err:
			default:
				bs.logger.Warn("Error channel full, dropping error")
			}
		}
	}
}

// processBlockHeader processes a new block header
func (bs *BlockSubscriber) processBlockHeader(header *types.Header) {
	startTime := time.Now()
	
	// Update stats
	bs.statsMutex.Lock()
	bs.stats.BlocksReceived++
	bs.stats.LastBlockAt = time.Now()
	bs.stats.LastBlockNumber = header.Number.Uint64()
	bs.stats.LastBlockHash = header.Hash().Hex()
	bs.statsMutex.Unlock()
	
	// Create block event
	event := &BlockEvent{
		Header:    header,
		Timestamp: time.Now(),
		Source:    "subscription",
		Processed: false,
	}
	
	// Apply filters if enabled
	if bs.config.EnableFiltering && bs.eventFilter != nil {
		matches := bs.eventFilter.FilterBlock(header)
		if len(matches) > 0 {
			event.Matches = matches
			bs.statsMutex.Lock()
			bs.stats.FilterMatches += int64(len(matches))
			bs.statsMutex.Unlock()
		} else {
			bs.statsMutex.Lock()
			bs.stats.BlocksFiltered++
			bs.statsMutex.Unlock()
			
			// Skip processing if no matches and filtering is strict
			return
		}
	}
	
	// Send to processing channel
	select {
	case bs.blockEvents <- event:
	default:
		bs.logger.Warn("Block events channel full, dropping block")
		bs.statsMutex.Lock()
		bs.stats.ProcessingErrors++
		bs.statsMutex.Unlock()
	}
	
	// Update processing time
	processingTime := time.Since(startTime)
	bs.statsMutex.Lock()
	if bs.stats.AverageProcessTime == 0 {
		bs.stats.AverageProcessTime = processingTime
	} else {
		bs.stats.AverageProcessTime = (bs.stats.AverageProcessTime + processingTime) / 2
	}
	bs.statsMutex.Unlock()
}

// eventProcessor processes block events through handlers
func (bs *BlockSubscriber) eventProcessor() {
	defer bs.logger.Info("Event processor stopped")
	
	for {
		select {
		case <-bs.ctx.Done():
			return
		case event := <-bs.blockEvents:
			bs.processEvent(event)
		}
	}
}

// processEvent processes a single block event
func (bs *BlockSubscriber) processEvent(event *BlockEvent) {
	defer func() {
		if r := recover(); r != nil {
			bs.logger.WithField("panic", r).Error("Panic in event processing")
			bs.statsMutex.Lock()
			bs.stats.ProcessingErrors++
			bs.statsMutex.Unlock()
		}
	}()
	
	// Process with timeout
	ctx, cancel := context.WithTimeout(bs.ctx, bs.config.ProcessingTimeout)
	defer cancel()
	
	done := make(chan bool, 1)
	
	go func() {
		defer func() {
			if r := recover(); r != nil {
				bs.logger.WithField("panic", r).Error("Panic in handler execution")
			}
			done <- true
		}()
		
		bs.executeHandlers(event)
	}()
	
	select {
	case <-ctx.Done():
		bs.logger.WithField("block", event.Header.Number).Warn("Event processing timeout")
		bs.statsMutex.Lock()
		bs.stats.ProcessingErrors++
		bs.statsMutex.Unlock()
	case <-done:
		event.Processed = true
		bs.statsMutex.Lock()
		bs.stats.BlocksProcessed++
		bs.statsMutex.Unlock()
		
		// Send to processed events channel
		select {
		case bs.processedEvents <- event:
		default:
			bs.logger.Warn("Processed events channel full, dropping event")
		}
	}
}

// executeHandlers executes all registered handlers for an event
func (bs *BlockSubscriber) executeHandlers(event *BlockEvent) {
	bs.handlersMutex.RLock()
	handlers := make([]BlockEventHandler, len(bs.handlers))
	copy(handlers, bs.handlers)
	bs.handlersMutex.RUnlock()
	
	for _, handler := range handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					bs.logger.WithFields(logrus.Fields{
						"handler": handler.GetName(),
						"panic":   r,
					}).Error("Handler panic")
				}
			}()
			
			if err := handler.HandleBlock(event); err != nil {
				bs.logger.WithFields(logrus.Fields{
					"handler": handler.GetName(),
					"error":   err,
				}).Error("Handler error")
				
				handler.HandleError(err)
				
				select {
				case bs.errorEvents <- err:
				default:
				}
			}
		}()
	}
}

// errorProcessor processes error events
func (bs *BlockSubscriber) errorProcessor() {
	defer bs.logger.Info("Error processor stopped")
	
	for {
		select {
		case <-bs.ctx.Done():
			return
		case err := <-bs.errorEvents:
			bs.handleError(err)
		}
	}
}

// handleError handles error events
func (bs *BlockSubscriber) handleError(err error) {
	bs.logger.WithError(err).Error("Processing error event")
	
	// Notify all handlers of the error
	bs.handlersMutex.RLock()
	handlers := make([]BlockEventHandler, len(bs.handlers))
	copy(handlers, bs.handlers)
	bs.handlersMutex.RUnlock()
	
	for _, handler := range handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					bs.logger.WithFields(logrus.Fields{
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
func (bs *BlockSubscriber) GetSubscription() *Subscription {
	return bs.subscription
}

// Restart restarts the block subscriber
func (bs *BlockSubscriber) Restart() error {
	bs.logger.Info("Restarting block subscriber")
	
	if err := bs.Stop(); err != nil {
		return fmt.Errorf("failed to stop subscriber: %v", err)
	}
	
	// Wait a bit before restarting
	time.Sleep(2 * time.Second)
	
	if err := bs.Start(); err != nil {
		return fmt.Errorf("failed to start subscriber: %v", err)
	}
	
	return nil
}

// SetFilter sets the event filter
func (bs *BlockSubscriber) SetFilter(filter *EventFilter) {
	bs.eventFilter = filter
	bs.logger.Info("Event filter updated")
}

// GetHandlers returns a copy of all registered handlers
func (bs *BlockSubscriber) GetHandlers() []BlockEventHandler {
	bs.handlersMutex.RLock()
	defer bs.handlersMutex.RUnlock()
	
	handlers := make([]BlockEventHandler, len(bs.handlers))
	copy(handlers, bs.handlers)
	return handlers
}
