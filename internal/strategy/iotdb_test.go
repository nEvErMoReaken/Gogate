// strategy_test.go
package strategy

import (
	"context"
	"gateway/internal/pkg"
	iotdbclient "github.com/apache/iotdb-client-go/client"
	"github.com/apache/iotdb-client-go/rpc"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"testing"
	"time"
)

// MockSession 实现 SessionInterface
type MockSession struct {
	insertAlignedRecordsOfOneDeviceFunc func(deviceId string, timestamps []int64, measurements [][]string, dataTypes [][]iotdbclient.TSDataType, values [][]interface{}, isAligned bool) (*rpc.TSStatus, error)
}

func (m *MockSession) InsertAlignedRecordsOfOneDevice(deviceId string, timestamps []int64, measurements [][]string, dataTypes [][]iotdbclient.TSDataType, values [][]interface{}, isAligned bool) (*rpc.TSStatus, error) {
	if m.insertAlignedRecordsOfOneDeviceFunc != nil {
		return m.insertAlignedRecordsOfOneDeviceFunc(deviceId, timestamps, measurements, dataTypes, values, isAligned)
	}
	return &rpc.TSStatus{Code: 200}, nil
}

func (m *MockSession) Close() {
	// 模拟关闭连接的操作
}

// MockSessionPool 实现 SessionPoolInterface
type MockSessionPool struct {
	getSessionFunc func() (SessionInterface, error)
	putBackFunc    func(session SessionInterface)
	closeFunc      func()
}

func (m *MockSessionPool) GetSession() (SessionInterface, error) {
	if m.getSessionFunc != nil {
		return m.getSessionFunc()
	}
	return &MockSession{}, nil
}

func (m *MockSessionPool) PutBack(session SessionInterface) {
	if m.putBackFunc != nil {
		m.putBackFunc(session)
	}
}

func (m *MockSessionPool) Close() {
	if m.closeFunc != nil {
		m.closeFunc()
	}
}

// Test NewIoTDBStrategy
func TestNewIoTDBStrategy(t *testing.T) {
	ctx := context.Background()

	// Mock configuration
	config := &pkg.Config{
		Strategy: []pkg.StrategyConfig{
			{
				Enable: true,
				Type:   "iotdb",
				Para: map[string]interface{}{
					"host":       "127.0.0.1",
					"port":       "6667",
					"mode":       "single",
					"url":        "",
					"username":   "root",
					"password":   "root",
					"batch_size": 1024,
				},
			},
		},
	}

	ctx = pkg.WithConfig(ctx, config)
	ctx = pkg.WithLogger(ctx, zap.NewNop())

	strategy, err := NewIoTDBStrategy(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, strategy)

	iotdbStrategy, ok := strategy.(*IoTDBStrategy)
	assert.True(t, ok)
	assert.Equal(t, "127.0.0.1", iotdbStrategy.info.Host)
	assert.Equal(t, "6667", iotdbStrategy.info.Port)
	assert.Equal(t, "root", iotdbStrategy.info.UserName)
	assert.Equal(t, int32(1024), iotdbStrategy.info.BatchSize)
}

// Test Publish
func TestIoTDBStrategy_Publish(t *testing.T) {
	ctx := context.Background()
	ctx = pkg.WithLogger(ctx, logger)

	mockSessionPool := &MockSessionPool{}
	mockSession := &MockSession{}
	mockSessionPool.getSessionFunc = func() (SessionInterface, error) {
		return mockSession, nil
	}
	mockSessionPool.putBackFunc = func(session SessionInterface) {}

	iotdbStrategy := &IoTDBStrategy{
		sessionPool: mockSessionPool,
		info:        IotDBInfo{},
		core: Core{
			StrategyType: "iotdb",
			pointChan:    make(chan pkg.Point, 200),
			ctx:          ctx,
		},
		logger: zap.NewNop(),
	}

	point := pkg.Point{
		DeviceName: "device1",
		DeviceType: "type1",
		Field: map[string]interface{}{
			"temperature": 23.5,
			"humidity":    60,
			"status":      "OK",
			"active":      true,
		},
		Ts: time.Now(),
	}

	// 模拟 InsertAlignedRecordsOfOneDevice 方法
	mockSession.insertAlignedRecordsOfOneDeviceFunc = func(deviceId string, timestamps []int64, measurements [][]string, dataTypes [][]iotdbclient.TSDataType, values [][]interface{}, isAligned bool) (*rpc.TSStatus, error) {
		assert.Equal(t, "root.type1.device1", deviceId)
		assert.Len(t, timestamps, 1)

		// 使用 ElementsMatch 进行断言
		expectedMeasurements := []string{"temperature", "humidity", "status", "active"}
		expectedValues := []interface{}{23.5, 60, "OK", true}

		assert.ElementsMatch(t, expectedMeasurements, measurements[0])
		assert.ElementsMatch(t, expectedValues, values[0])

		return &rpc.TSStatus{Code: 200}, nil
	}

	err := iotdbStrategy.Publish(point)
	assert.NoError(t, err)
}

// Test Publish with Error
func TestIoTDBStrategy_Publish_Error(t *testing.T) {
	ctx := context.Background()
	ctx = pkg.WithLogger(ctx, logger)

	mockSessionPool := &MockSessionPool{}
	mockSession := &MockSession{}
	mockSessionPool.getSessionFunc = func() (SessionInterface, error) {
		return mockSession, nil
	}
	mockSessionPool.putBackFunc = func(session SessionInterface) {}

	iotdbStrategy := &IoTDBStrategy{
		sessionPool: mockSessionPool,
		info:        IotDBInfo{},
		core: Core{
			StrategyType: "iotdb",
			pointChan:    make(chan pkg.Point, 200),
			ctx:          ctx,
		},
		logger: zap.NewNop(),
	}

	point := pkg.Point{
		DeviceName: "device1",
		DeviceType: "type1",
		Field: map[string]interface{}{
			"unsupported": struct{}{}, // 不支持的数据类型
		},
		Ts: time.Now(),
	}

	err := iotdbStrategy.Publish(point)
	assert.NoError(t, err) // 因为不支持的字段被跳过，不应该返回错误
}

// Test Start and Stop
func TestIoTDBStrategy_StartStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ctx = pkg.WithLogger(ctx, zap.NewNop())

	mockSessionPool := &MockSessionPool{}
	mockSessionPool.closeFunc = func() {}

	iotdbStrategy := &IoTDBStrategy{
		sessionPool: mockSessionPool,
		info:        IotDBInfo{},
		core: Core{
			StrategyType: "iotdb",
			pointChan:    make(chan pkg.Point, 1),
			ctx:          ctx,
		},
		logger: zap.NewNop(),
	}

	// 在单独的 goroutine 中启动策略
	go func() {
		iotdbStrategy.Start()
	}()

	// 发送一个点
	point := pkg.Point{
		DeviceName: "device1",
		Field:      map[string]interface{}{"temperature": 25.0},
		Ts:         time.Now(),
	}
	iotdbStrategy.core.pointChan <- point

	// 停止策略
	cancel()

	// 等待以确保 goroutine 已经停止
	time.Sleep(100 * time.Millisecond)
}
