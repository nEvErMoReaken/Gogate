package connector_test

import (
	"context"
	"errors"
	"gateway/internal/pkg"
	"github.com/stretchr/testify/assert"
	"testing"
)

// MockConnector 是一个模拟的 Template，用于测试
type MockConnector struct{}

func (m *MockConnector) Start() {}

func (m *MockConnector) Ready() chan pkg.DataSource {
	return nil
}

func (m *MockConnector) Close() error {
	return nil
}

func (m *MockConnector) GetDataSource() (pkg.DataSource, error) {
	return pkg.DataSource{}, nil
}

// MockFactoryFunc 是一个用于测试的工厂函数
func MockFactoryFunc(ctx context.Context) (Template, error) {
	_ = ctx
	return &MockConnector{}, nil
}

func TestRegister(t *testing.T) {
	// 清空 Factories 映射，防止测试污染
	Factories = make(map[string]FactoryFunc)

	// 注册一个新的数据源类型
	Register("mock", MockFactoryFunc)

	// 验证是否正确注册
	factory, exists := Factories["mock"]
	assert.True(t, exists, "应该成功注册数据源类型 'mock'")

	// 调用注册的工厂函数，验证是否可以成功返回一个 Template
	connector, err := factory(context.Background())
	assert.NoError(t, err, "调用注册的工厂函数不应返回错误")
	assert.NotNil(t, connector, "工厂函数返回的 Template 不应为 nil")
}

func TestNew_Success(t *testing.T) {
	// 清空 Factories 映射，防止测试污染
	Factories = make(map[string]FactoryFunc)

	// 注册一个新的数据源类型
	Register("mock", MockFactoryFunc)

	// 模拟配置
	config := &pkg.Config{
		Connector: pkg.ConnectorConfig{
			Type: "mock",
		},
	}
	ctx := pkg.WithConfig(context.Background(), config)

	// 调用 New 函数
	connector, err := New(ctx)
	assert.NoError(t, err, "New 函数应成功返回")
	assert.NotNil(t, connector, "返回的 Template 不应为 nil")

	// 验证返回的 Template 是否为 MockConnector 类型
	_, ok := connector.(*MockConnector)
	assert.True(t, ok, "返回的 Template 应为 MockConnector 类型")
}

func TestNew_UnknownType(t *testing.T) {
	// 清空 Factories 映射，防止测试污染
	Factories = make(map[string]FactoryFunc)

	// 模拟配置，使用未注册的类型
	config := &pkg.Config{
		Connector: pkg.ConnectorConfig{
			Type: "unknown",
		},
	}
	ctx := pkg.WithConfig(context.Background(), config)

	// 调用 New 函数，预期应该失败
	_, err := New(ctx)
	assert.Error(t, err, "应返回错误，因为 'unknown' 类型未注册")
	assert.EqualError(t, err, "未找到数据源类型: unknown")
}

func TestNew_FactoryError(t *testing.T) {
	// 清空 Factories 映射，防止测试污染
	Factories = make(map[string]FactoryFunc)

	// 注册一个工厂函数，它返回错误
	Register("error", func(ctx context.Context) (Template, error) {
		return nil, errors.New("factory error")
	})

	// 模拟配置
	config := &pkg.Config{
		Connector: pkg.ConnectorConfig{
			Type: "error",
		},
	}
	ctx := pkg.WithConfig(context.Background(), config)

	// 调用 New 函数，预期应该返回初始化错误
	_, err := New(ctx)
	assert.Error(t, err, "应返回错误，因为工厂函数返回错误")
	assert.EqualError(t, err, "初始化数据源失败: factory error")
}
