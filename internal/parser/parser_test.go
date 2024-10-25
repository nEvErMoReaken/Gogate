package parser

import (
	"context"
	"testing"
	"time"

	"gateway/internal/pkg"
	"github.com/stretchr/testify/assert"
)

// 测试 checkFilter 函数对于通配符和部分匹配的行为
func TestCheckFilterWithWildcard(t *testing.T) {
	deviceType := "vobc.info"
	deviceName := "vobc0001.abc"
	telemetryName := "temperature"

	// 测试通配符匹配
	match, err := checkFilter(deviceType, deviceName, telemetryName, ".*:.*:.*")
	assert.NoError(t, err, "通配符过滤条件语法正确时不应报错")
	assert.True(t, match, "通配符应匹配所有情况")

	// 测试部分匹配
	match, err = checkFilter(deviceType, deviceName, telemetryName, "vobc\\.info:.*:.*")
	assert.NoError(t, err, "部分匹配的过滤条件语法正确时不应报错")
	assert.True(t, match, "部分匹配条件应成功匹配")

	// 测试不匹配的情况
	match, err = checkFilter(deviceType, deviceName, telemetryName, "vobc\\.info:wrongDevice:.*")
	assert.NoError(t, err, "不匹配的过滤条件语法正确时不应报错")
	assert.False(t, match, "不匹配的设备名称应该返回 false")
}

// 测试策略启用和禁用的行为
func TestStrategyConfig_Enable(t *testing.T) {
	// 模拟策略配置
	strategy := pkg.StrategyConfig{
		Type:   "influxdb",
		Enable: true,
		Filter: []string{".*vobc.info:.*:.*"},
		Para: map[string]interface{}{
			"url":    "http://localhost:8086",
			"token":  "example-token",
			"org":    "test-org",
			"bucket": "test-bucket",
		},
	}

	// 测试策略是否启用
	assert.True(t, strategy.Enable, "策略应被启用")
	assert.Equal(t, "influxdb", strategy.Type, "策略类型应为 'influxdb'")

	// 禁用策略并测试
	strategy.Enable = false
	assert.False(t, strategy.Enable, "策略应被禁用")
}

// 测试策略的过滤条件
func TestStrategyConfig_Filter(t *testing.T) {
	deviceType := "vobc.info"
	deviceName := "vobc0001.abc"
	telemetryName := "temperature"

	strategy := pkg.StrategyConfig{
		Type:   "influxdb",
		Enable: true,
		Filter: []string{
			"vobc\\.info:vobc0001\\.abc:temperature",
			"vobc\\.info:.*:.*", // 通配符策略
		},
	}

	// 测试第一个过滤条件
	match, err := checkFilter(deviceType, deviceName, telemetryName, strategy.Filter[0])
	assert.NoError(t, err, "过滤条件语法正确时不应报错")
	assert.True(t, match, "应匹配第一个过滤条件")

	// 测试第二个过滤条件 (通配符)
	match, err = checkFilter(deviceType, deviceName, telemetryName, strategy.Filter[1])
	assert.NoError(t, err, "通配符过滤条件语法正确时不应报错")
	assert.True(t, match, "通配符应匹配")
}

// 测试策略的自定义配置项
func TestStrategyConfig_Para(t *testing.T) {
	strategy := pkg.StrategyConfig{
		Type:   "influxdb",
		Enable: true,
		Para: map[string]interface{}{
			"url":    "http://localhost:8086",
			"token":  "example-token",
			"org":    "test-org",
			"bucket": "test-bucket",
		},
	}

	// 检查各个自定义配置项
	assert.Equal(t, "http://localhost:8086", strategy.Para["url"], "URL 应该正确解析")
	assert.Equal(t, "example-token", strategy.Para["token"], "Token 应该正确解析")
	assert.Equal(t, "test-org", strategy.Para["org"], "Org 应该正确解析")
	assert.Equal(t, "test-bucket", strategy.Para["bucket"], "Bucket 应该正确解析")
}

// 测试使用 SnapshotCollection 管理多个设备快照
func TestSnapshotCollection_MultiDevice(t *testing.T) {
	collection := make(SnapshotCollection)

	// 添加第一个设备
	snapshot1, err1 := collection.GetDeviceSnapshot("device1", "type1")
	assert.NoError(t, err1, "创建第一个设备快照时不应出错")
	_ = snapshot1.SetField(context.Background(), "temperature", 25)

	// 添加第二个设备
	snapshot2, err2 := collection.GetDeviceSnapshot("device2", "type2")
	assert.NoError(t, err2, "创建第二个设备快照时不应出错")
	_ = snapshot2.SetField(context.Background(), "humidity", 60)

	// 检查两个设备快照是否独立存在
	val1, exists1 := snapshot1.GetField("temperature")
	assert.True(t, exists1, "设备1的 'temperature' 字段应存在")
	assert.Equal(t, 25, val1, "设备1的 'temperature' 值应为 25")

	val2, exists2 := snapshot2.GetField("humidity")
	assert.True(t, exists2, "设备2的 'humidity' 字段应存在")
	assert.Equal(t, 60, val2, "设备2的 'humidity' 值应为 60")
}

func TestDeviceSnapshotToJSON(t *testing.T) {
	snapshot := &DeviceSnapshot{
		DeviceName: "device1",
		DeviceType: "type1",
		Fields: map[string]interface{}{
			"temperature": 25,
		},
		Ts: time.Now(),
	}

	// 将快照转换为 JSON 字符串
	jsonStr := snapshot.toJSON()

	// 检查是否成功转换为 JSON
	assert.Contains(t, jsonStr, "device1", "JSON 字符串应包含设备名称")
	assert.Contains(t, jsonStr, "type1", "JSON 字符串应包含设备类型")
	assert.Contains(t, jsonStr, "temperature", "JSON 字符串应包含字段")
}

func TestSetDeviceSnapshot(t *testing.T) {
	ctx := context.Background()
	collection := make(SnapshotCollection)

	// 设置一个设备快照中的字段
	err := collection.SetDeviceSnapshot("device1", "type1", "temperature", 25, ctx)

	// 检查是否成功设置字段
	assert.NoError(t, err, "设置字段不应报错")
	snapshot, exists := collection["device1:type1"]
	assert.True(t, exists, "设备快照应存在")
	value, ok := snapshot.GetField("temperature")
	assert.True(t, ok, "字段应存在")
	assert.Equal(t, 25, value, "字段值应为 25")
}

func TestInitDataSink(t *testing.T) {
	snapshot := &DeviceSnapshot{
		DeviceName: "device1",
		DeviceType: "type1",
		Fields:     map[string]interface{}{"temperature": 25},
		DataSink:   make(map[string][]string),
	}

	strategies := []pkg.StrategyConfig{
		{
			Type:   "influxdb",
			Filter: []string{"type1:device1:temperature"},
		},
	}

	// 初始化数据点映射
	err := snapshot.InitDataSink("temperature", &strategies)

	// 检查是否成功初始化数据点
	assert.NoError(t, err, "初始化数据点不应出错")
	assert.Contains(t, snapshot.DataSink, "influxdb", "应包含 influxdb 策略")
	assert.Contains(t, snapshot.DataSink["influxdb"], "temperature", "策略应包含 'temperature' 字段")
}

func TestMakePoint(t *testing.T) {
	snapshot := &DeviceSnapshot{
		DeviceName: "device1",
		DeviceType: "type1",
		Fields:     map[string]interface{}{"temperature": 25},
		DataSink:   map[string][]string{"influxdb": {"temperature"}},
		Ts:         time.Now(),
	}

	point := snapshot.makePoint("influxdb")

	// 检查生成的数据点是否正确
	assert.Equal(t, "device1", point.DeviceName, "设备名称应为 'device1'")
	assert.Equal(t, "type1", point.DeviceType, "设备类型应为 'type1'")
	assert.Equal(t, 25, point.Field["temperature"], "字段值应为 25")
}

func TestLaunch(t *testing.T) {
	ctx := context.WithValue(context.Background(), "ts", time.Now())

	snapshot := &DeviceSnapshot{
		DeviceName: "device1",
		DeviceType: "type1",
		Fields:     map[string]interface{}{"temperature": 25},
		DataSink:   map[string][]string{"influxdb": {"temperature"}},
		Ts:         time.Now(),
	}

	mapChan := map[string]chan pkg.Point{
		"influxdb": make(chan pkg.Point, 1),
	}

	// 启动发射数据点
	go snapshot.launch(ctx, mapChan)

	// 检查数据点是否正确发射
	point := <-mapChan["influxdb"]
	assert.Equal(t, "device1", point.DeviceName, "设备名称应为 'device1'")
	assert.Equal(t, "type1", point.DeviceType, "设备类型应为 'type1'")
	assert.Equal(t, 25, point.Field["temperature"], "字段值应为 25")
}

func TestLaunchALL(t *testing.T) {
	ctx := context.WithValue(context.Background(), "ts", time.Now())

	collection := make(SnapshotCollection)

	// 创建两个快照
	snapshot1, _ := collection.GetDeviceSnapshot("device1", "type1")
	snapshot1.Fields["temperature"] = 25
	snapshot1.DataSink = map[string][]string{"influxdb": {"temperature"}}

	snapshot2, _ := collection.GetDeviceSnapshot("device2", "type2")
	snapshot2.Fields["humidity"] = 60
	snapshot2.DataSink = map[string][]string{"influxdb": {"humidity"}}

	mapChan := map[string]chan pkg.Point{
		"influxdb": make(chan pkg.Point, 2), // 设置缓冲区大小为 2
	}

	// 启动所有快照的发射
	go collection.LaunchALL(ctx, mapChan)

	// 创建一个 map 来存储接收到的数据点
	receivedPoints := make(map[string]pkg.Point)

	timeout := time.After(2 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case point := <-mapChan["influxdb"]:
			receivedPoints[point.DeviceName] = point
		case <-timeout:
			t.Fatal("Timed out waiting for points")
		}
	}

	// 检查是否收到了预期的设备数据点
	if point, ok := receivedPoints["device1"]; ok {
		assert.Equal(t, 25, point.Field["temperature"], "字段 'temperature' 应为 25")
	} else {
		t.Fatal("Did not receive data point from 'device1'")
	}

	if point, ok := receivedPoints["device2"]; ok {
		assert.Equal(t, 60, point.Field["humidity"], "字段 'humidity' 应为 60")
	} else {
		t.Fatal("Did not receive data point from 'device2'")
	}
}

func TestNew(t *testing.T) {
	ctx := context.Background()

	// 模拟数据源和通道
	dataSource := pkg.DataSource{}
	mapChan := make(map[string]chan pkg.Point)

	// 模拟配置
	config := &pkg.Config{
		Parser: pkg.ParserConfig{Type: "test_parser", Para: map[string]interface{}{"dir": "../../script"}},
	}
	ctx = pkg.WithConfig(ctx, config)

	// 注册一个简单的工厂函数
	Register("test_parser", func(ds pkg.DataSource, mapCh map[string]chan pkg.Point, ctx context.Context) (Parser, error) {
		return &mockParser{}, nil
	})
	// 测试 New 函数
	parser, err := New(ctx, dataSource, mapChan)
	assert.NoError(t, err, "初始化解析器不应报错")
	assert.NotNil(t, parser, "解析器应成功创建")
}

// 模拟一个 Parser
type mockParser struct{}

func (p *mockParser) Start() {}
