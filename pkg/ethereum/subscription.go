package ethereum

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"
)

// SubscriptionType represents different types of subscriptions
type SubscriptionType string

const (
	SubscriptionTypeNewHeads         SubscriptionType = "newHeads"
	SubscriptionTypePendingTxs       SubscriptionType = "newPendingTransactions"
	SubscriptionTypeLogs             SubscriptionType = "logs"
	SubscriptionTypeSyncing          SubscriptionType = "syncing"
	SubscriptionTypeNewPendingTxs    SubscriptionType = "newPendingTransactionHashes"
)

// SubscriptionStatus represents the status of a subscription
type SubscriptionStatus int

const (
	SubscriptionStatusInactive SubscriptionStatus = iota
	SubscriptionStatusActive
	SubscriptionStatusError
	SubscriptionStatusReconnecting
)

func (s SubscriptionStatus) String() string {
	switch s {
	case SubscriptionStatusInactive:
		return "inactive"
	case SubscriptionStatusActive:
		return "active"
	case SubscriptionStatusError:
		return "error"
	case SubscriptionStatusReconnecting:
		return "reconnecting"
	default:
		return "unknown"
	}
}

// SubscriptionConfig holds subscription configuration
type SubscriptionConfig struct {
	Type       SubscriptionType `json:"type"`
	Parameters interface{}      `json:"parameters,omitempty"`
	AutoReconnect bool          `json:"auto_reconnect"`
	BufferSize    int           `json:"buffer_size"`
	MaxRetries    int           `json:"max_retries"`
	RetryInterval time.Duration `json:"retry_interval"`
}

// DefaultSubscriptionConfig returns default subscription configuration
func DefaultSubscriptionConfig(subType SubscriptionType) *SubscriptionConfig {
	return &SubscriptionConfig{
		Type:          subType,
		AutoReconnect: true,
		BufferSize:    1000,
		MaxRetries:    5,
		RetryInterval: 5 * time.Second,
	}
}

// Subscription represents an active subscription
type Subscription struct {
	ID            string              `json:"id"`
	Config        *SubscriptionConfig `json:"config"`
	Status        SubscriptionStatus  `json:"status"`
	CreatedAt     time.Time          `json:"created_at"`
	LastMessageAt time.Time          `json:"last_message_at"`
	MessageCount  int64              `json:"message_count"`
	ErrorCount    int                `json:"error_count"`
	LastError     string             `json:"last_error,omitempty"`
	
	// Channels
	dataChan   chan interface{}
	errorChan  chan error
	closeChan  chan struct{}
	
	// Internal
	manager    *SubscriptionManager
	retryCount int
	mutex      sync.RWMutex
}

// GetDataChannel returns the data channel for this subscription
func (s *Subscription) GetDataChannel() <-chan interface{} {
	return s.dataChan
}

// GetErrorChannel returns the error channel for this subscription
func (s *Subscription) GetErrorChannel() <-chan error {
	return s.errorChan
}

// Close closes the subscription
func (s *Subscription) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	if s.Status == SubscriptionStatusInactive {
		return nil
	}
	
	s.Status = SubscriptionStatusInactive
	close(s.closeChan)
	
	return s.manager.unsubscribe(s.ID)
}

// GetStats returns subscription statistics
func (s *Subscription) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	uptime := time.Duration(0)
	if !s.CreatedAt.IsZero() {
		uptime = time.Since(s.CreatedAt)
	}
	
	return map[string]interface{}{
		"id":              s.ID,
		"type":            s.Config.Type,
		"status":          s.Status.String(),
		"created_at":      s.CreatedAt,
		"last_message_at": s.LastMessageAt,
		"message_count":   s.MessageCount,
		"error_count":     s.ErrorCount,
		"last_error":      s.LastError,
		"uptime":          uptime,
		"retry_count":     s.retryCount,
	}
}

// SubscriptionManager manages WebSocket subscriptions
type SubscriptionManager struct {
	wsManager     *WSConnectionManager
	subscriptions map[string]*Subscription
	mutex         sync.RWMutex
	
	// Message routing
	messageRouter map[string]*Subscription
	routerMutex   sync.RWMutex
	
	// Context and cancellation
	ctx    context.Context
	cancel context.CancelFunc
	
	// Event handlers
	onSubscriptionCreated func(*Subscription)
	onSubscriptionClosed  func(string)
	onSubscriptionError   func(string, error)
	
	logger *logrus.Entry
}

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager(wsManager *WSConnectionManager) *SubscriptionManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	sm := &SubscriptionManager{
		wsManager:     wsManager,
		subscriptions: make(map[string]*Subscription),
		messageRouter: make(map[string]*Subscription),
		ctx:           ctx,
		cancel:        cancel,
		logger:        logrus.WithField("component", "subscription_manager"),
	}
	
	// Set up WebSocket event handlers
	wsManager.SetEventHandlers(
		sm.onWSConnect,
		sm.onWSDisconnect,
		sm.onWSMessage,
		sm.onWSError,
	)
	
	return sm
}

// SetEventHandlers sets event handlers for subscription events
func (sm *SubscriptionManager) SetEventHandlers(
	onCreated func(*Subscription),
	onClosed func(string),
	onError func(string, error),
) {
	sm.onSubscriptionCreated = onCreated
	sm.onSubscriptionClosed = onClosed
	sm.onSubscriptionError = onError
}

// Subscribe creates a new subscription
func (sm *SubscriptionManager) Subscribe(config *SubscriptionConfig) (*Subscription, error) {
	if !sm.wsManager.IsConnected() {
		return nil, fmt.Errorf("WebSocket not connected")
	}
	
	// Create subscription request
	subscribeMsg := &WSMessage{
		ID:      generateSubscriptionID(),
		Method:  "eth_subscribe",
		Params:  []interface{}{string(config.Type), config.Parameters},
		JSONRPC: "2.0",
	}
	
	// Send subscription request
	if err := sm.wsManager.SendMessage(subscribeMsg); err != nil {
		return nil, fmt.Errorf("failed to send subscription request: %v", err)
	}
	
	// Create subscription object
	subscription := &Subscription{
		ID:        subscribeMsg.ID.(string),
		Config:    config,
		Status:    SubscriptionStatusActive,
		CreatedAt: time.Now(),
		dataChan:  make(chan interface{}, config.BufferSize),
		errorChan: make(chan error, 10),
		closeChan: make(chan struct{}),
		manager:   sm,
	}
	
	// Store subscription
	sm.mutex.Lock()
	sm.subscriptions[subscription.ID] = subscription
	sm.mutex.Unlock()
	
	if sm.onSubscriptionCreated != nil {
		sm.onSubscriptionCreated(subscription)
	}
	
	sm.logger.WithFields(logrus.Fields{
		"id":   subscription.ID,
		"type": config.Type,
	}).Info("Subscription created")
	
	return subscription, nil
}

// Unsubscribe removes a subscription
func (sm *SubscriptionManager) Unsubscribe(subscriptionID string) error {
	return sm.unsubscribe(subscriptionID)
}

// unsubscribe internal method to remove a subscription
func (sm *SubscriptionManager) unsubscribe(subscriptionID string) error {
	sm.mutex.Lock()
	_, exists := sm.subscriptions[subscriptionID]
	if !exists {
		sm.mutex.Unlock()
		return fmt.Errorf("subscription not found: %s", subscriptionID)
	}
	delete(sm.subscriptions, subscriptionID)
	sm.mutex.Unlock()
	
	// Remove from message router
	sm.routerMutex.Lock()
	delete(sm.messageRouter, subscriptionID)
	sm.routerMutex.Unlock()
	
	// Send unsubscribe request if WebSocket is connected
	if sm.wsManager.IsConnected() {
		unsubscribeMsg := &WSMessage{
			ID:      generateSubscriptionID(),
			Method:  "eth_unsubscribe",
			Params:  []interface{}{subscriptionID},
			JSONRPC: "2.0",
		}
		
		if err := sm.wsManager.SendMessage(unsubscribeMsg); err != nil {
			sm.logger.WithError(err).Warn("Failed to send unsubscribe request")
		}
	}
	
	if sm.onSubscriptionClosed != nil {
		sm.onSubscriptionClosed(subscriptionID)
	}
	
	sm.logger.WithField("id", subscriptionID).Info("Subscription removed")
	return nil
}

// GetSubscription returns a subscription by ID
func (sm *SubscriptionManager) GetSubscription(id string) (*Subscription, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	subscription, exists := sm.subscriptions[id]
	return subscription, exists
}

// GetAllSubscriptions returns all active subscriptions
func (sm *SubscriptionManager) GetAllSubscriptions() map[string]*Subscription {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	result := make(map[string]*Subscription)
	for id, sub := range sm.subscriptions {
		result[id] = sub
	}
	return result
}

// GetSubscriptionsByType returns subscriptions of a specific type
func (sm *SubscriptionManager) GetSubscriptionsByType(subType SubscriptionType) []*Subscription {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	var result []*Subscription
	for _, sub := range sm.subscriptions {
		if sub.Config.Type == subType {
			result = append(result, sub)
		}
	}
	return result
}

// Close closes all subscriptions and the manager
func (sm *SubscriptionManager) Close() error {
	sm.logger.Info("Closing subscription manager")
	
	// Close all subscriptions
	sm.mutex.Lock()
	for id := range sm.subscriptions {
		sm.unsubscribe(id)
	}
	sm.mutex.Unlock()
	
	sm.cancel()
	return nil
}

// GetStats returns subscription manager statistics
func (sm *SubscriptionManager) GetStats() map[string]interface{} {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	stats := map[string]interface{}{
		"total_subscriptions": len(sm.subscriptions),
		"subscriptions_by_type": make(map[string]int),
		"subscriptions_by_status": make(map[string]int),
	}
	
	typeCount := make(map[string]int)
	statusCount := make(map[string]int)
	
	for _, sub := range sm.subscriptions {
		typeCount[string(sub.Config.Type)]++
		statusCount[sub.Status.String()]++
	}
	
	stats["subscriptions_by_type"] = typeCount
	stats["subscriptions_by_status"] = statusCount
	
	return stats
}

// WebSocket event handlers

func (sm *SubscriptionManager) onWSConnect() {
	sm.logger.Info("WebSocket connected, reestablishing subscriptions")
	sm.reestablishSubscriptions()
}

func (sm *SubscriptionManager) onWSDisconnect(err error) {
	sm.logger.WithError(err).Warn("WebSocket disconnected")
	
	// Mark all subscriptions as reconnecting
	sm.mutex.Lock()
	for _, sub := range sm.subscriptions {
		sub.mutex.Lock()
		if sub.Status == SubscriptionStatusActive {
			sub.Status = SubscriptionStatusReconnecting
		}
		sub.mutex.Unlock()
	}
	sm.mutex.Unlock()
}

func (sm *SubscriptionManager) onWSMessage(msg *WSMessage) {
	// Handle subscription responses
	if msg.Method == "" && msg.Result != nil {
		// This is a subscription confirmation
		sm.handleSubscriptionConfirmation(msg)
		return
	}
	
	// Handle subscription notifications
	if msg.Method == "eth_subscription" {
		sm.handleSubscriptionNotification(msg)
		return
	}
	
	// Handle unsubscribe confirmations
	if msg.Result != nil {
		sm.logger.WithField("message", msg).Debug("Received message response")
	}
}

func (sm *SubscriptionManager) onWSError(err error) {
	sm.logger.WithError(err).Error("WebSocket error")
	
	// Notify all subscriptions of the error
	sm.mutex.RLock()
	for _, sub := range sm.subscriptions {
		select {
		case sub.errorChan <- err:
		default:
		}
		
		sub.mutex.Lock()
		sub.ErrorCount++
		sub.LastError = err.Error()
		sub.mutex.Unlock()
		
		if sm.onSubscriptionError != nil {
			sm.onSubscriptionError(sub.ID, err)
		}
	}
	sm.mutex.RUnlock()
}

// handleSubscriptionConfirmation handles subscription confirmation messages
func (sm *SubscriptionManager) handleSubscriptionConfirmation(msg *WSMessage) {
	requestID := fmt.Sprintf("%v", msg.ID)
	
	sm.mutex.RLock()
	subscription, exists := sm.subscriptions[requestID]
	sm.mutex.RUnlock()
	
	if !exists {
		sm.logger.WithField("request_id", requestID).Warn("Received confirmation for unknown subscription")
		return
	}
	
	if msg.Error != nil {
		subscription.mutex.Lock()
		subscription.Status = SubscriptionStatusError
		subscription.ErrorCount++
		subscription.LastError = fmt.Sprintf("%v", msg.Error)
		subscription.mutex.Unlock()
		
		sm.logger.WithFields(logrus.Fields{
			"id":    requestID,
			"error": msg.Error,
		}).Error("Subscription failed")
		
		select {
		case subscription.errorChan <- fmt.Errorf("subscription failed: %v", msg.Error):
		default:
		}
		
		return
	}
	
	// Extract subscription ID from result
	subscriptionID := fmt.Sprintf("%v", msg.Result)
	
	// Update message router
	sm.routerMutex.Lock()
	sm.messageRouter[subscriptionID] = subscription
	sm.routerMutex.Unlock()
	
	sm.logger.WithFields(logrus.Fields{
		"request_id":      requestID,
		"subscription_id": subscriptionID,
	}).Info("Subscription confirmed")
}

// handleSubscriptionNotification handles subscription notification messages
func (sm *SubscriptionManager) handleSubscriptionNotification(msg *WSMessage) {
	params, ok := msg.Params.(map[string]interface{})
	if !ok {
		sm.logger.Warn("Invalid subscription notification format")
		return
	}
	
	subscriptionID, ok := params["subscription"].(string)
	if !ok {
		sm.logger.Warn("Missing subscription ID in notification")
		return
	}
	
	result := params["result"]
	
	// Find subscription
	sm.routerMutex.RLock()
	subscription, exists := sm.messageRouter[subscriptionID]
	sm.routerMutex.RUnlock()
	
	if !exists {
		sm.logger.WithField("subscription_id", subscriptionID).Warn("Received notification for unknown subscription")
		return
	}
	
	// Update subscription stats
	subscription.mutex.Lock()
	subscription.MessageCount++
	subscription.LastMessageAt = time.Now()
	subscription.mutex.Unlock()
	
	// Parse and send data based on subscription type
	var data interface{}
	switch subscription.Config.Type {
	case SubscriptionTypeNewHeads:
		data = sm.parseBlockHeader(result)
	case SubscriptionTypePendingTxs, SubscriptionTypeNewPendingTxs:
		data = sm.parseTransaction(result)
	case SubscriptionTypeLogs:
		data = sm.parseLog(result)
	case SubscriptionTypeSyncing:
		data = result
	default:
		data = result
	}
	
	// Send data to subscription channel
	select {
	case subscription.dataChan <- data:
	default:
		sm.logger.WithField("subscription_id", subscriptionID).Warn("Subscription data channel full, dropping message")
	}
}

// reestablishSubscriptions reestablishes all subscriptions after reconnection
func (sm *SubscriptionManager) reestablishSubscriptions() {
	sm.mutex.RLock()
	subscriptions := make([]*Subscription, 0, len(sm.subscriptions))
	for _, sub := range sm.subscriptions {
		if sub.Config.AutoReconnect && sub.Status == SubscriptionStatusReconnecting {
			subscriptions = append(subscriptions, sub)
		}
	}
	sm.mutex.RUnlock()
	
	for _, sub := range subscriptions {
		sm.logger.WithField("id", sub.ID).Info("Reestablishing subscription")
		
		// Create new subscription request
		subscribeMsg := &WSMessage{
			ID:      generateSubscriptionID(),
			Method:  "eth_subscribe",
			Params:  []interface{}{string(sub.Config.Type), sub.Config.Parameters},
			JSONRPC: "2.0",
		}
		
		if err := sm.wsManager.SendMessage(subscribeMsg); err != nil {
			sm.logger.WithError(err).Error("Failed to reestablish subscription")
			
			sub.mutex.Lock()
			sub.Status = SubscriptionStatusError
			sub.ErrorCount++
			sub.LastError = err.Error()
			sub.mutex.Unlock()
			
			continue
		}
		
		sub.mutex.Lock()
		sub.Status = SubscriptionStatusActive
		sub.retryCount++
		sub.mutex.Unlock()
	}
}

// Data parsing methods

func (sm *SubscriptionManager) parseBlockHeader(data interface{}) *types.Header {
	headerData, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}
	
	// Convert to JSON and back to properly parse the header
	headerJSON, err := json.Marshal(headerData)
	if err != nil {
		sm.logger.WithError(err).Error("Failed to marshal header data")
		return nil
	}
	
	var header types.Header
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		sm.logger.WithError(err).Error("Failed to unmarshal header")
		return nil
	}
	
	return &header
}

func (sm *SubscriptionManager) parseTransaction(data interface{}) interface{} {
	// For pending transactions, data might be just a hash string or full transaction
	switch v := data.(type) {
	case string:
		// Transaction hash
		return common.HexToHash(v)
	case map[string]interface{}:
		// Full transaction data
		txJSON, err := json.Marshal(v)
		if err != nil {
			sm.logger.WithError(err).Error("Failed to marshal transaction data")
			return v
		}
		
		var tx types.Transaction
		if err := json.Unmarshal(txJSON, &tx); err != nil {
			sm.logger.WithError(err).Error("Failed to unmarshal transaction")
			return v
		}
		
		return &tx
	default:
		return data
	}
}

func (sm *SubscriptionManager) parseLog(data interface{}) *types.Log {
	logData, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}
	
	logJSON, err := json.Marshal(logData)
	if err != nil {
		sm.logger.WithError(err).Error("Failed to marshal log data")
		return nil
	}
	
	var log types.Log
	if err := json.Unmarshal(logJSON, &log); err != nil {
		sm.logger.WithError(err).Error("Failed to unmarshal log")
		return nil
	}
	
	return &log
}

// generateSubscriptionID generates a unique subscription ID
func generateSubscriptionID() string {
	return fmt.Sprintf("sub_%d", time.Now().UnixNano())
}
