package pkg

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestWithErrChan 测试 WithErrChan 和 ErrChanFromContext 方法
func TestWithErrChan(t *testing.T) {
	// 定义一个错误通道，用于测试
	errChan := make(chan error, 1)

	// 创建一个上下文，将错误通道注入到上下文中
	ctx := context.Background()
	ctxWithErrChan := WithErrChan(ctx, errChan)

	// 从上下文中提取错误通道
	extractedErrChan := ErrChanFromContext(ctxWithErrChan)

	// 检查提取出来的通道是否与原始通道相同
	if extractedErrChan == nil {
		t.Errorf("期望从上下文中提取到错误通道，但提取结果为 nil")
	}

	// 验证错误通道的发送与接收
	testErr := make(chan bool)
	go func() {
		err := <-extractedErrChan
		if err.Error() == "测试错误" {
			testErr <- true
		}
	}()

	// 发送一个错误到通道
	errChan <- fmt.Errorf("测试错误")

	select {
	case <-testErr:
		// 成功接收到错误，测试通过
	case <-time.After(1 * time.Second):
		t.Errorf("在1秒内没有收到预期的错误")
	}
}

// TestErrChanFromContextWithoutErrChan 测试当上下文中没有错误通道时的情况
func TestErrChanFromContextWithoutErrChan(t *testing.T) {
	// 创建一个不包含错误通道的上下文
	ctx := context.Background()

	// 尝试从上下文中提取错误通道
	extractedErrChan := ErrChanFromContext(ctx)

	// 期望返回 nil，因为上下文中没有存储错误通道
	if extractedErrChan != nil {
		t.Errorf("期望提取结果为 nil，但提取到非空通道")
	}
}
