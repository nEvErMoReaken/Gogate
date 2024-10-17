package internal

import (
	"context"
	"gateway/internal/connector"
	"gateway/internal/parser"
	"gateway/internal/pkg"
	"gateway/internal/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"testing"
	"time"
)

// Mock Logger for capturing log outputs
var logger, _ = zap.NewDevelopment()

// MockEagerConnector 模拟连接器
type MockEagerConnector struct {
	mock.Mock
	dataSource pkg.DataSource
}

// MockLazyConnector 模拟懒连接器
type MockLazyConnector struct {
	mock.Mock
	dataSource pkg.DataSource
	readyChan  chan pkg.DataSource
}

func (m *MockLazyConnector) GetDataSource() (pkg.DataSource, error) {
	args := m.Called()
	return args.Get(0).(pkg.DataSource), args.Error(1)
}

func (m *MockLazyConnector) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockLazyConnector) Start() {
	m.Called()
}

func (m *MockLazyConnector) Ready() chan pkg.DataSource {
	args := m.Called()
	if ch, ok := args.Get(0).(chan pkg.DataSource); ok {
		return ch
	}
	return m.readyChan
}

func (m *MockEagerConnector) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockEagerConnector) Start() {
	m.Called()
}

func (m *MockEagerConnector) Ready() chan pkg.DataSource {
	args := m.Called()
	if ch, ok := args.Get(0).(chan pkg.DataSource); ok {
		return ch
	}
	return nil
}

func (m *MockEagerConnector) GetDataSource() (pkg.DataSource, error) {
	args := m.Called()
	return args.Get(0).(pkg.DataSource), args.Error(1)
}

// MockStrategy 模拟策略
type MockStrategy struct {
	mock.Mock
	pointChan chan pkg.Point
}

func (m *MockStrategy) GetCore() strategy.Core {
	return strategy.Core{
		StrategyType: "mock",
		PointChan:    m.pointChan,
	}
}

func (m *MockStrategy) GetChan() chan pkg.Point {
	return m.pointChan
}

func (m *MockStrategy) Start() {
	m.Called()
}

// MockParser 模拟解析器
type mockParser struct {
	mock.Mock
	mapChan    map[string]chan pkg.Point
	dataSource pkg.DataSource
}

func (m *mockParser) Start() {
	data := <-m.dataSource.Source.(chan string)
	logger.Debug("Received data", zap.String("data", data))
	mockPoint := pkg.Point{
		DeviceName: "mockName",
		DeviceType: "mockType",
		Field: map[string]interface{}{
			"test": "ok",
		},
		Ts: time.Now(),
	}
	m.mapChan["mock"] <- mockPoint
	logger.Debug("Sending data to strategy", zap.Any("point", mockPoint))
	m.Called()
}

// 集成测试
func TestEager_StartPipeline(t *testing.T) {
	ctx := pkg.WithLogger(context.Background(), logger)
	errChan := make(chan error, 1)
	ctx = pkg.WithErrChan(ctx, errChan)

	// 创建模拟的连接器
	mockConnector := new(MockEagerConnector)
	dataSource := pkg.DataSource{
		Source:   make(chan string, 1),
		MetaData: nil,
	}
	mockConnector.On("Start").Return()
	mockConnector.On("Ready").Return(nil)
	mockConnector.On("GetDataSource").Return(dataSource, nil)

	// 替换 connector.New，为了返回我们的 mockConnector
	originalConnectorNew := connector.New
	connector.New = func(ctx context.Context) (connector.Connector, error) {
		return mockConnector, nil
	}
	defer func() { connector.New = originalConnectorNew }()

	// 创建模拟的策略
	mockStrategy := new(MockStrategy)
	mockStrategy.pointChan = make(chan pkg.Point, 1)
	mockStrategy.On("Start").Return()

	// 替换 strategy.New，使其返回我们的 mockStrategy
	originalStrategyNew := strategy.New
	strategy.New = func(ctx context.Context) (strategy.MapSendStrategy, error) {
		strategies := make(strategy.MapSendStrategy)
		strategies["mock"] = mockStrategy
		return strategies, nil
	}
	defer func() { strategy.New = originalStrategyNew }()

	mp := new(mockParser)
	mp.On("Start").Return()
	originalParserNew := parser.New
	parser.New = func(ctx context.Context, dataSource pkg.DataSource, mapChan map[string]chan pkg.Point) (parser.Parser, error) {
		mp.dataSource = dataSource
		mp.mapChan = mapChan
		return mp, nil
	}
	defer func() { parser.New = originalParserNew }()
	// 启动管道
	go StartPipeline(ctx)

	// 模拟连接器准备就绪，发送数据源
	ds, _ := mockConnector.GetDataSource()
	ds.Source.(chan string) <- "mockData"

	// 等待策略启动
	time.Sleep(100 * time.Millisecond)

	// 验证策略是否收到了正确的数据点
	select {
	case point := <-mockStrategy.pointChan:
		// 在这里验证 point 的内容是否正确
		assert.Equal(t, "mockName", point.DeviceName)
		assert.Equal(t, "mockType", point.DeviceType)
		assert.Equal(t, "ok", point.Field["test"])
		assert.NotNil(t, point.Ts)
	case err := <-errChan:
		t.Fatalf("Unexpected error: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for point")
	}

	// 验证连接器和策略的方法是否被正确调用
	mockConnector.AssertExpectations(t)
	mockStrategy.AssertExpectations(t)
}

func TestLazy_StartPipeline(t *testing.T) {
	ctx := pkg.WithLogger(context.Background(), logger)
	errChan := make(chan error, 1)
	ctx = pkg.WithErrChan(ctx, errChan)

	// 创建模拟的连接器
	mockConnector := new(MockLazyConnector)
	dataSource := pkg.DataSource{
		Source:   make(chan string, 1),
		MetaData: nil,
	}
	mockConnector.On("Start").Return()
	mockConnector.On("Ready").Return(make(chan pkg.DataSource, 1))

	// 替换 connector.New，为了返回我们的 mockConnector
	originalConnectorNew := connector.New
	connector.New = func(ctx context.Context) (connector.Connector, error) {
		return mockConnector, nil
	}
	defer func() { connector.New = originalConnectorNew }()

	// 创建模拟的策略
	mockStrategy := new(MockStrategy)
	mockStrategy.pointChan = make(chan pkg.Point, 1)
	mockStrategy.On("Start").Return()

	// 替换 strategy.New，使其返回我们的 mockStrategy
	originalStrategyNew := strategy.New
	strategy.New = func(ctx context.Context) (strategy.MapSendStrategy, error) {
		strategies := make(strategy.MapSendStrategy)
		strategies["mock"] = mockStrategy
		return strategies, nil
	}
	defer func() { strategy.New = originalStrategyNew }()

	mp := new(mockParser)
	mp.On("Start").Return()
	originalParserNew := parser.New
	parser.New = func(ctx context.Context, dataSource pkg.DataSource, mapChan map[string]chan pkg.Point) (parser.Parser, error) {
		mp.dataSource = dataSource
		mp.mapChan = mapChan
		return mp, nil
	}
	defer func() { parser.New = originalParserNew }()
	// 启动管道
	go StartPipeline(ctx)

	// 模拟连接器准备就绪，发送数据源
	mockConnector.Ready() <- dataSource
	dataSource.Source.(chan string) <- "mockData"

	// 等待策略启动
	time.Sleep(100 * time.Millisecond)

	// 验证策略是否收到了正确的数据点
	select {
	case point := <-mockStrategy.pointChan:
		// 在这里验证 point 的内容是否正确
		assert.Equal(t, "mockName", point.DeviceName)
		assert.Equal(t, "mockType", point.DeviceType)
		assert.Equal(t, "ok", point.Field["test"])
		assert.NotNil(t, point.Ts)
	case err := <-errChan:
		t.Fatalf("Unexpected error: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for point")
	}

	// 验证连接器和策略的方法是否被正确调用
	mockConnector.AssertExpectations(t)
	mockStrategy.AssertExpectations(t)
}
