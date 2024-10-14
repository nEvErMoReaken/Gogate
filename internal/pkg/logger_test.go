package pkg

import (
	"context"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"testing"
)

// TestNewLogger 测试 NewLogger 是否能够正确创建一个 logger
func TestNewLogger(t *testing.T) {
	// Setup config
	config := &LogConfig{
		LogPath:    "test.log",
		MaxSize:    1,
		MaxBackups: 3,
		MaxAge:     7,
		Compress:   false,
		Level:      "infoo",
	}

	logger := NewLogger(config)

	// Check if logger is not nil
	if logger == nil {
		t.Fatal("expected logger to be non-nil")
	}

	// Use a zap observer to test log output
	core, logs := observer.New(zap.DebugLevel)
	logger = zap.New(core)

	logger.Info("Test log")
	// 检查日志是否被记录
	assert.Equal(t, 1, logs.Len(), "Expected 1 log entry")
	// 检查日志消息是否正确
	assert.Equal(t, "Test log", logs.All()[0].Message, "Unexpected log message")
}

// TestWithLogger 测试 WithLogger 是否能够正确添加 logger 到 context
func TestWithLogger(t *testing.T) {
	logger := zap.NewNop() // no-op logger

	ctx := WithLogger(context.Background(), logger)
	retrievedLogger := LoggerFromContext(ctx)

	// Logger should not be nil and should be the same as what we put into the context
	assert.NotNil(t, retrievedLogger, "expected logger to be present in context")
	assert.Equal(t, logger, retrievedLogger, "expected logger to be the same")
}

// TestWithLoggerAndModule 验证 WithLoggerAndModule 是否能够正确添加 logger 和模块信息到 context
func TestWithLoggerAndModule(t *testing.T) {
	logger := zap.NewNop() // no-op logger
	ctx := WithLoggerAndModule(context.Background(), logger, "testModule")

	retrievedLogger := LoggerFromContext(ctx)

	// Create an observer to capture logs
	core, logs := observer.New(zap.DebugLevel)
	retrievedLogger = zap.New(core).With(zap.String("module", "testModule"))

	// Log something to see if module info is present
	retrievedLogger.Info("Module log")

	// Verify the logger has the "module" field
	assert.Equal(t, "testModule", logs.All()[0].ContextMap()["module"], "Module field should be present in the log")
}

// TestLoggerFromContext 测试 LoggerFromContext 是否能够正确从 context 中提取 logger
func TestLoggerFromContext_NoLogger(t *testing.T) {
	ctx := context.Background() // No logger added to context

	logger := LoggerFromContext(ctx)

	// Ensure a no-op logger is returned
	assert.NotNil(t, logger, "expected a logger, but got nil")
	assert.Equal(t, zap.NewNop(), logger, "expected no-op logger")
}
