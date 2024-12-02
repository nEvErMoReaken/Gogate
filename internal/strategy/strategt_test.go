package strategy_test

import (
	"context"
	"errors"
	"fmt"
	"gateway/internal/pkg"
	"gateway/internal/strategy"
	"github.com/stretchr/testify/assert"
	"testing"
)

// MockStrategy 模拟一个策略，用于测试
type MockStrategy struct {
	strategy.Core
}

func (m *MockStrategy) GetCore() strategy.Core {
	return m.Core
}

func (m *MockStrategy) GetChan() chan pkg.Point {
	return m.PointChan
}

func (m *MockStrategy) Start() {
	// 模拟启动操作
	fmt.Println("MockStrategy started")
}

// 模拟工厂函数
func mockStrategyFactory(ctx context.Context) (strategy.Template, error) {
	return &MockStrategy{
		Core: strategy.Core{
			StrategyType: "mock",
			PointChan:    make(chan pkg.Point, 1),
			Ctx:          ctx,
		},
	}, nil
}

func failingStrategyFactory(ctx context.Context) (strategy.Template, error) {
	_ = ctx
	return nil, errors.New("failed to create strategy")
}

// 测试 Register 函数
func TestRegister(t *testing.T) {
	strategy.Register("mock", mockStrategyFactory)
	assert.Contains(t, strategy.Factories, "mock", "Factories should contain the registered strategy")
}

// 测试 New 函数成功的情况
func TestNew_Success(t *testing.T) {
	// 注册策略工厂
	strategy.Register("mock", mockStrategyFactory)

	// 创建测试配置
	testConfig := &pkg.Config{
		Strategy: []pkg.StrategyConfig{
			{Type: "mock", Enable: true},
		},
	}

	// 将配置放入上下文
	ctx := pkg.WithConfig(context.Background(), testConfig)

	// 初始化策略集
	strategies, err := strategy.New(ctx)
	assert.NoError(t, err, "Expected no error while initializing strategies")
	assert.Contains(t, strategies, "mock", "Expected strategies to contain 'mock'")

	// 验证策略集中的策略类型
	mockStrategy, ok := strategies["mock"].(*MockStrategy)
	assert.True(t, ok, "Expected strategy to be of type *MockStrategy")
	assert.Equal(t, "mock", mockStrategy.GetCore().StrategyType, "Expected strategy type to be 'mock'")
}

// 测试 New 函数失败的情况
func TestNew_Failure(t *testing.T) {
	// 注册一个失败的策略工厂
	strategy.Register("failing", failingStrategyFactory)

	// 创建测试配置
	testConfig := &pkg.Config{
		Strategy: []pkg.StrategyConfig{
			{Type: "failing", Enable: true},
		},
	}

	// 将配置放入上下文
	ctx := pkg.WithConfig(context.Background(), testConfig)

	// 尝试初始化策略集
	_, err := strategy.New(ctx)
	assert.Error(t, err, "Expected an error while initializing strategies")
	assert.Contains(t, err.Error(), "failed to create strategy", "Expected error message to contain 'failed to create strategy'")
}

// 测试策略集的初始化，当策略未启用时
func TestNew_StrategyDisabled(t *testing.T) {
	// 注册策略工厂
	strategy.Register("mock", mockStrategyFactory)

	// 创建测试配置，设置策略为未启用
	testConfig := &pkg.Config{
		Strategy: []pkg.StrategyConfig{
			{Type: "mock", Enable: false},
		},
	}

	// 将配置放入上下文
	ctx := pkg.WithConfig(context.Background(), testConfig)

	// 初始化策略集
	strategies, err := strategy.New(ctx)
	assert.NoError(t, err, "Expected no error while initializing strategies")
	assert.Empty(t, strategies, "Expected strategies to be empty since no strategies are enabled")
}
