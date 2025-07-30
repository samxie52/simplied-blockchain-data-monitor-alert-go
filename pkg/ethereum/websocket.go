package ethereum

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// WSConnectionState represents the state of a WebSocket connection
// WSConnectionState 状态
type WSConnectionState int

const (
	WSStateDisconnected WSConnectionState = iota // 离线
	WSStateConnecting                            // 连接中
	WSStateConnected                             // 已连接
	WSStateReconnecting                          // 重新连接中
	WSStateClosed                                // 已关闭
)

// String returns the string representation of the connection state
func (s WSConnectionState) String() string {
	switch s {
	case WSStateDisconnected:
		return "disconnected"
	case WSStateConnecting:
		return "connecting"
	case WSStateConnected:
		return "connected"
	case WSStateReconnecting:
		return "reconnecting"
	case WSStateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// WSConfig holds WebSocket connection configuration
type WSConfig struct {
	// WebSocket URL
	URL string `json:"url"`
	//	重连间隔
	ReconnectInterval time.Duration `json:"reconnect_interval"`
	//	最大重连次数
	MaxReconnectAttempts int `json:"max_reconnect_attempts"`
	//	ping间隔
	PingInterval time.Duration `json:"ping_interval"`
	//	pong超时
	PongTimeout time.Duration `json:"pong_timeout"`
	//	写超时
	WriteTimeout time.Duration `json:"write_timeout"`
	//	读超时
	ReadTimeout time.Duration `json:"read_timeout"`
	//	缓冲区大小
	BufferSize int `json:"buffer_size"`
}

// DefaultWSConfig returns default WebSocket configuration
func DefaultWSConfig() *WSConfig {
	return &WSConfig{
		ReconnectInterval:    5 * time.Second,
		MaxReconnectAttempts: 10,
		PingInterval:         30 * time.Second,
		PongTimeout:          10 * time.Second,
		WriteTimeout:         10 * time.Second,
		ReadTimeout:          60 * time.Second,
		BufferSize:           1024,
	}
}

// WSMessage represents a WebSocket message
type WSMessage struct {
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	JSONRPC string      `json:"jsonrpc"`
}

// WSConnectionManager manages WebSocket connections to Ethereum nodes
type WSConnectionManager struct {
	// 配置
	config *WSConfig
	// 连接
	conn *websocket.Conn

	// 状态
	state WSConnectionState
	// 状态互斥锁
	stateMutex sync.RWMutex
	// 重连次数
	reconnectAttempts int
	// 最后错误
	lastError error

	// Channels
	// 收到的消息
	incomingMessages chan *WSMessage
	// 发送的消息
	outgoingMessages chan *WSMessage
	// 关闭信号
	closeSignal chan struct{}
	// 重连信号
	reconnectSignal chan struct{}

	// Context and cancellation
	// 上下文
	ctx context.Context
	// 取消函数
	cancel context.CancelFunc

	// Event handlers
	// 连接事件
	onConnect func()
	// 断开事件
	onDisconnect func(error)
	// 消息事件
	onMessage func(*WSMessage)
	// 错误事件
	onError func(error)

	// Statistics
	// 统计
	stats WSConnectionStats

	// 日志
	logger *logrus.Entry
}

// WSConnectionStats holds connection statistics
// 连接统计
type WSConnectionStats struct {
	// 连接时间
	ConnectedAt time.Time `json:"connected_at"`
	// 最后消息时间
	LastMessageAt time.Time `json:"last_message_at"`
	// 发送的消息数
	MessagesSent int64 `json:"messages_sent"`
	// 接收的消息数
	MessagesReceived int64 `json:"messages_received"`
	// 重连次数
	ReconnectCount int `json:"reconnect_count"`
	// 总在线时间
	TotalUptime time.Duration `json:"total_uptime"`
	// 当前在线时间
	CurrentUptime time.Duration `json:"current_uptime"`
	// 最后错误
	LastError string `json:"last_error,omitempty"`
	// 发送的字节数
	BytesSent int64 `json:"bytes_sent"`
	// 接收的字节数
	BytesReceived int64 `json:"bytes_received"`
}

// NewWSConnectionManager creates a new WebSocket connection manager
func NewWSConnectionManager(config *WSConfig) *WSConnectionManager {
	if config == nil {
		config = DefaultWSConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &WSConnectionManager{
		config:           config,
		state:            WSStateDisconnected,
		incomingMessages: make(chan *WSMessage, config.BufferSize),
		outgoingMessages: make(chan *WSMessage, config.BufferSize),
		closeSignal:      make(chan struct{}),
		reconnectSignal:  make(chan struct{}),
		ctx:              ctx,
		cancel:           cancel,
		logger:           logrus.WithField("component", "websocket_manager"),
	}
}

// SetEventHandlers sets event handlers for connection events
func (w *WSConnectionManager) SetEventHandlers(
	onConnect func(),
	onDisconnect func(error),
	onMessage func(*WSMessage),
	onError func(error),
) {
	w.onConnect = onConnect
	w.onDisconnect = onDisconnect
	w.onMessage = onMessage
	w.onError = onError
}

// Connect establishes a WebSocket connection
func (w *WSConnectionManager) Connect() error {
	w.setState(WSStateConnecting)

	u, err := url.Parse(w.config.URL)
	if err != nil {
		w.setError(fmt.Errorf("invalid WebSocket URL: %v", err))
		return err
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	w.logger.WithField("url", w.config.URL).Info("Connecting to WebSocket")

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		w.setError(fmt.Errorf("failed to connect: %v", err))
		w.setState(WSStateDisconnected)
		return err
	}

	w.conn = conn
	w.setState(WSStateConnected)
	w.stats.ConnectedAt = time.Now()
	w.reconnectAttempts = 0

	// Start connection management goroutines
	go w.readPump()
	go w.writePump()
	go w.pingPump()

	if w.onConnect != nil {
		w.onConnect()
	}

	w.logger.Info("WebSocket connection established")
	return nil
}

// Disconnect closes the WebSocket connection
func (w *WSConnectionManager) Disconnect() error {
	w.logger.Info("Disconnecting WebSocket")

	w.setState(WSStateClosed)

	if w.conn != nil {
		// Send close message
		err := w.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			w.logger.WithError(err).Warn("Error sending close message")
		}

		w.conn.Close()
		w.conn = nil
	}

	w.cancel()
	close(w.closeSignal)

	return nil
}

// SendMessage sends a message through the WebSocket
func (w *WSConnectionManager) SendMessage(msg *WSMessage) error {
	if w.getState() != WSStateConnected {
		return fmt.Errorf("connection not established")
	}

	select {
	case w.outgoingMessages <- msg:
		return nil
	case <-w.ctx.Done():
		return fmt.Errorf("connection closed")
	default:
		return fmt.Errorf("outgoing message buffer full")
	}
}

// GetState returns the current connection state
func (w *WSConnectionManager) GetState() WSConnectionState {
	return w.getState()
}

// GetStats returns connection statistics
func (w *WSConnectionManager) GetStats() WSConnectionStats {
	stats := w.stats
	if w.getState() == WSStateConnected && !w.stats.ConnectedAt.IsZero() {
		stats.CurrentUptime = time.Since(w.stats.ConnectedAt)
	}
	if w.lastError != nil {
		stats.LastError = w.lastError.Error()
	}
	return stats
}

// IsConnected returns true if the connection is established
func (w *WSConnectionManager) IsConnected() bool {
	return w.getState() == WSStateConnected
}

// readPump handles incoming messages
func (w *WSConnectionManager) readPump() {
	defer func() {
		if w.onDisconnect != nil {
			w.onDisconnect(w.lastError)
		}
		w.tryReconnect()
	}()

	if w.config.ReadTimeout > 0 {
		w.conn.SetReadDeadline(time.Now().Add(w.config.ReadTimeout))
	}

	w.conn.SetPongHandler(func(string) error {
		if w.config.ReadTimeout > 0 {
			w.conn.SetReadDeadline(time.Now().Add(w.config.ReadTimeout))
		}
		return nil
	})

	for {
		select {
		case <-w.ctx.Done():
			return
		default:
		}

		_, messageBytes, err := w.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				w.logger.WithError(err).Error("WebSocket read error")
				w.setError(err)
			}
			return
		}

		w.stats.MessagesReceived++
		w.stats.BytesReceived += int64(len(messageBytes))
		w.stats.LastMessageAt = time.Now()

		var msg WSMessage
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			w.logger.WithError(err).Warn("Failed to unmarshal message")
			continue
		}

		if w.onMessage != nil {
			w.onMessage(&msg)
		}

		select {
		case w.incomingMessages <- &msg:
		default:
			w.logger.Warn("Incoming message buffer full, dropping message")
		}
	}
}

// writePump handles outgoing messages
func (w *WSConnectionManager) writePump() {
	ticker := time.NewTicker(w.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case msg := <-w.outgoingMessages:
			if w.config.WriteTimeout > 0 {
				w.conn.SetWriteDeadline(time.Now().Add(w.config.WriteTimeout))
			}

			messageBytes, err := json.Marshal(msg)
			if err != nil {
				w.logger.WithError(err).Error("Failed to marshal message")
				continue
			}

			if err := w.conn.WriteMessage(websocket.TextMessage, messageBytes); err != nil {
				w.logger.WithError(err).Error("WebSocket write error")
				w.setError(err)
				return
			}

			w.stats.MessagesSent++
			w.stats.BytesSent += int64(len(messageBytes))
		}
	}
}

// pingPump sends periodic ping messages
func (w *WSConnectionManager) pingPump() {
	ticker := time.NewTicker(w.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			if w.getState() != WSStateConnected {
				continue
			}

			if w.config.WriteTimeout > 0 {
				w.conn.SetWriteDeadline(time.Now().Add(w.config.WriteTimeout))
			}

			if err := w.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				w.logger.WithError(err).Error("Failed to send ping")
				w.setError(err)
				return
			}
		}
	}
}

// tryReconnect attempts to reconnect the WebSocket
func (w *WSConnectionManager) tryReconnect() {
	if w.getState() == WSStateClosed {
		return
	}

	w.setState(WSStateReconnecting)

	for w.reconnectAttempts < w.config.MaxReconnectAttempts {
		w.reconnectAttempts++
		w.stats.ReconnectCount++

		w.logger.WithFields(logrus.Fields{
			"attempt": w.reconnectAttempts,
			"max":     w.config.MaxReconnectAttempts,
		}).Info("Attempting to reconnect WebSocket")

		select {
		case <-w.ctx.Done():
			return
		case <-time.After(w.config.ReconnectInterval):
		}

		if err := w.Connect(); err != nil {
			w.logger.WithError(err).Warn("Reconnection attempt failed")
			continue
		}

		w.logger.Info("WebSocket reconnected successfully")
		return
	}

	w.logger.Error("Max reconnection attempts reached, giving up")
	w.setState(WSStateDisconnected)
}

// setState sets the connection state thread-safely
func (w *WSConnectionManager) setState(state WSConnectionState) {
	w.stateMutex.Lock()
	defer w.stateMutex.Unlock()
	w.state = state
}

// getState gets the connection state thread-safely
func (w *WSConnectionManager) getState() WSConnectionState {
	w.stateMutex.RLock()
	defer w.stateMutex.RUnlock()
	return w.state
}

// setError sets the last error
func (w *WSConnectionManager) setError(err error) {
	w.lastError = err
	if w.onError != nil {
		w.onError(err)
	}
}

// GetIncomingMessages returns the channel for incoming messages
func (w *WSConnectionManager) GetIncomingMessages() <-chan *WSMessage {
	return w.incomingMessages
}

// GetOutgoingMessages returns the channel for outgoing messages
func (w *WSConnectionManager) GetOutgoingMessages() chan<- *WSMessage {
	return w.outgoingMessages
}
