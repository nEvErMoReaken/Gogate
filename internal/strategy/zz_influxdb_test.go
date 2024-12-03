package strategy

import (
	"context"
	"gateway/internal/pkg"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"testing"
	"time"
)

// Mock Logger for capturing log outputs
var logger, _ = zap.NewDevelopment()

// MockInfluxDBClient 是一个模拟的 InfluxDB 客户端
type MockInfluxDBClient struct {
	mock.Mock
	influxdb2.Client
}

func (m *MockInfluxDBClient) WriteAPI(org, bucket string) api.WriteAPI {
	args := m.Called(org, bucket)
	return args.Get(0).(api.WriteAPI)
}

func (m *MockInfluxDBClient) Close() {
	m.Called()
}

// MockWriteAPI 是一个模拟的 InfluxDB 写入 API
type MockWriteAPI struct {
	mock.Mock
	api.WriteAPI
}

func (m *MockWriteAPI) WritePoint(point *write.Point) {
	m.Called(point)
}

func (m *MockWriteAPI) Flush() {
	m.Called()
}

func TestNewInfluxDbStrategy(t *testing.T) {
	// 创建模拟配置
	config := &pkg.Config{
		Strategy: []pkg.StrategyConfig{
			{
				Enable: true,
				Type:   "influxdb",
				Para: map[string]interface{}{
					"url":        "http://localhost:8086",
					"org":        "my-org",
					"token":      "my-token",
					"bucket":     "my-bucket",
					"batch_size": uint(100),
					"tags":       []string{"tag1", "tag2"},
				},
			},
		},
	}

	// 创建模拟的 context 和 logger
	ctx := pkg.WithConfig(context.Background(), config)

	// 测试 NewInfluxDbStrategy
	strategy, err := NewInfluxDbStrategy(ctx)
	assert.NoError(t, err, "NewInfluxDbStrategy 应该成功初始化")
	assert.NotNil(t, strategy, "策略实例不应为 nil")

	// 验证返回的策略类型是否为 InfluxDbStrategy
	_, ok := strategy.(*InfluxDbStrategy)
	assert.True(t, ok, "返回的策略应为 InfluxDbStrategy 类型")
}
func TestInfluxDbStrategy_Publish(t *testing.T) {
	// 创建模拟的 InfluxDB 客户端和写入 API
	mockClient := new(MockInfluxDBClient)
	mockWriteAPI := new(MockWriteAPI)

	// 设置 InfluxDB 客户端返回的写入 API
	mockClient.On("WriteAPI", "my-org", "my-bucket").Return(mockWriteAPI)

	// 创建 InfluxDbStrategy
	strategy := &InfluxDbStrategy{
		client:   mockClient,
		writeAPI: mockWriteAPI,
		info: InfluxDbInfo{
			URL:    "http://localhost:8086",
			Org:    "my-org",
			Token:  "my-token",
			Bucket: "my-bucket",
			Tags:   []string{"tag1", "tag2"},
		},
		logger: zap.NewNop(),
	}

	// 模拟数据点
	point := pkg.Point{
		DeviceName: "device1",
		DeviceType: "type1",
		Field: map[string]interface{}{
			"tag1":   "value1",
			"field1": 10.5,
		},
		Ts: time.Now(),
	}

	// 设置对 WritePoint 的预期调用
	mockWriteAPI.On("WritePoint", mock.Anything).Return()

	// 执行 Publish 方法
	err := strategy.Publish(point)
	assert.NoError(t, err, "Publish 方法不应返回错误")

	// 验证 WritePoint 是否被调用
	mockWriteAPI.AssertCalled(t, "WritePoint", mock.Anything)
}

func TestInfluxDbStrategy_StartAndStop(t *testing.T) {
	// 创建模拟的 InfluxDB 客户端和写入 API
	mockClient := new(MockInfluxDBClient)
	mockWriteAPI := new(MockWriteAPI)
	// 创建模拟配置
	config := &pkg.Config{
		Strategy: []pkg.StrategyConfig{
			{
				Enable: true,
				Type:   "influxdb",
				Para: map[string]interface{}{
					"url":        "http://localhost:8086",
					"org":        "my-org",
					"token":      "my-token",
					"bucket":     "my-bucket",
					"batch_size": uint(100),
					"tags":       []string{"tag1", "tag2"},
				},
			},
		},
	}
	// 设置 InfluxDB 客户端返回的写入 API
	mockClient.On("WriteAPI", "my-org", "my-bucket").Return(mockWriteAPI)
	mockClient.On("Close").Return()
	mockWriteAPI.On("Flush").Return()
	// 创建模拟的 context 和 logger
	ctx := pkg.WithConfig(context.Background(), config)
	// 设置对 WritePoint 的预期调用
	mockWriteAPI.On("WritePoint", mock.Anything).Return()

	// 创建 InfluxDbStrategy
	strategy := &InfluxDbStrategy{
		client:   mockClient,
		writeAPI: mockWriteAPI,
		info: InfluxDbInfo{
			URL:    "http://localhost:8086",
			Org:    "my-org",
			Token:  "my-token",
			Bucket: "my-bucket",
			Tags:   []string{"tag1", "tag2"},
		},
		logger: zap.NewNop(),
		ctx:    ctx,
	}

	// 创建一个取消函数来停止策略
	ctx, cancel := context.WithCancel(strategy.ctx)
	strategy.ctx = ctx
	pc := make(chan pkg.Point, 200)
	// 启动策略
	go strategy.Start(pc)

	// 发送一个数据点到 channel
	pc <- pkg.Point{
		DeviceName: "device1",
		DeviceType: "type1",
		Field:      map[string]interface{}{"field1": 42},
		Ts:         time.Now(),
	}

	// 停止策略
	cancel()
	time.Sleep(100 * time.Millisecond) // 等待策略停止

	// 验证 Flush 和 Close 是否被调用
	mockWriteAPI.AssertCalled(t, "Flush")
	mockClient.AssertCalled(t, "Close")
}
