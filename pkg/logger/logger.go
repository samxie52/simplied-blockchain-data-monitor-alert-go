package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// Logger 结构化日志记录器
type Logger struct {
	// 嵌入 logrus.Logger
	*logrus.Logger
	// 组件名称
	component string
}

// Config 日志配置
type Config struct {
	// 日志级别
	Level string `json:"level"`
	// 日志格式
	Format string `json:"format"`
	// 日志输出
	Output string `json:"output"`
	// 日志文件路径
	FilePath string `json:"file_path,omitempty"`
	// 组件名称
	Component string `json:"component"`
}

// New 创建新的日志记录器
func New(config Config) (*Logger, error) {
	log := logrus.New()

	// 设置日志级别
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}
	log.SetLevel(level)

	// 设置日志格式
	switch config.Format {
	case "json":
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	case "text":
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
		})
	default:
		return nil, fmt.Errorf("unsupported log format: %s", config.Format)
	}

	// 设置日志输出
	output, err := getLogOutput(config.Output, config.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to setup log output: %w", err)
	}
	log.SetOutput(output)

	return &Logger{
		Logger:    log,
		component: config.Component,
	}, nil
}

// getLogOutput 获取日志输出
func getLogOutput(output, filePath string) (io.Writer, error) {
	switch output {
	case "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	case "file":
		if filePath == "" {
			return nil, fmt.Errorf("file path is required when output is 'file'")
		}

		// 确保目录存在
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		return file, nil
	default:
		return nil, fmt.Errorf("unsupported output type: %s", output)
	}
}

// WithContext 添加上下文
func (l *Logger) WithContext(ctx context.Context) *logrus.Entry {
	entry := l.Logger.WithContext(ctx).WithField("component", l.component)

	// 从上下文中提取请求ID
	if requestID := ctx.Value("request_id"); requestID != nil {
		entry = entry.WithField("request_id", requestID)
	}

	return entry
}

// LogHTTPRequest 记录HTTP请求日志
func (l *Logger) LogHTTPRequest(method, path, userAgent, clientIP string, statusCode int, duration time.Duration) {
	l.WithFields(logrus.Fields{
		"method":      method,
		"path":        path,
		"user_agent":  userAgent,
		"client_ip":   clientIP,
		"status_code": statusCode,
		"duration_ms": duration.Milliseconds(),
		"type":        "http_request",
		"component":   l.component,
	}).Info("HTTP request processed")
}
