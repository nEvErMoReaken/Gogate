package parser

import (
	"context"
	"gateway/internal/pkg"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewJsonParser(t *testing.T) {
	// 创建模拟的数据源和通道
	dataSource := pkg.DataSource{Source: make(chan string)}
	mapChan := make(map[string]chan pkg.Point)
	ctx := context.Background()

	// 模拟配置
	config := &pkg.Config{
		Parser: pkg.ParserConfig{
			Para: map[string]interface{}{"method": "test_method"},
		},
	}
	ctx = pkg.WithConfig(ctx, config)

	// 创建解析器
	parser, err := NewJsonParser(dataSource, mapChan, ctx)
	assert.NoError(t, err, "解析器初始化不应出现错误")
	assert.NotNil(t, parser, "解析器应成功创建")
}

func TestStart(t *testing.T) {
	// 创建模拟的数据源和通道
	dataChan := make(chan string, 1) // 缓冲区避免阻塞
	dataSource := pkg.DataSource{Source: dataChan}
	errChan := make(chan error, 5) // 缓冲区避免阻塞
	mapChan := make(map[string]chan pkg.Point)
	mapChan["influxdb"] = make(chan pkg.Point, 1) // 为 'influxdb' 策略设置缓冲区
	ctx, cancel := context.WithCancel(context.Background())
	ctx = pkg.WithErrChan(ctx, errChan)
	// 模拟配置
	config := &pkg.Config{
		Parser: pkg.ParserConfig{
			Para: map[string]interface{}{"method": "test_method"},
		},
		Strategy: []pkg.StrategyConfig{
			{
				Type:   "influxdb",
				Enable: true,
				Para: map[string]interface{}{
					"url":   "http://localhost:8086",
					"token": "example-token",
				},
			},
		},
	}
	ctx = pkg.WithConfig(ctx, config)

	// 模拟 JSON 脚本函数
	JsonScriptFuncCache["test_method"] = func(data map[string]interface{}) (string, string, map[string]interface{}, error) {
		return "device1", "type1", map[string]interface{}{"temperature": 25}, nil
	}

	// 创建解析器
	parser, _ := NewJsonParser(dataSource, mapChan, ctx)
	jParser := parser.(*jParser)

	// 启动解析器
	go jParser.Start()

	// 发送模拟 JSON 数据
	dataChan <- `{"temperature": 25}`

	// 等待数据发送到 mapChan 中并消费
	select {
	case point := <-mapChan["influxdb"]:
		// 检查数据点是否正确发射
		assert.Equal(t, "device1", point.DeviceName, "设备名称应为 'device1'")
		assert.Equal(t, "type1", point.DeviceType, "设备类型应为 'type1'")
		assert.Equal(t, 25, point.Field["temperature"], "字段 'temperature' 应为 25")
	case err := <-errChan:
		t.Fatalf("解析器发生错误: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("超时: 未能从 mapChan 获取到数据点")
	}

	// 取消上下文，停止解析器
	cancel()
}

func TestConversionToSnapshot(t *testing.T) {
	// 创建解析器
	ctx := context.Background()
	dataSource := pkg.DataSource{}
	mapChan := make(map[string]chan pkg.Point)
	jParser := &jParser{
		dataSource:         dataSource,
		mapChan:            mapChan,
		ctx:                ctx,
		SnapshotCollection: make(SnapshotCollection),
		jParserConfig:      jParserConfig{Method: "test_method"},
	}

	// 模拟 JSON 脚本函数
	JsonScriptFuncCache["test_method"] = func(data map[string]interface{}) (string, string, map[string]interface{}, error) {
		return "device1", "type1", map[string]interface{}{"temperature": 25}, nil
	}

	// 进行 JSON 转换
	jParser.ConversionToSnapshot(`{"temperature": 25}`)

	// 检查快照是否正确
	snapshot, err := jParser.SnapshotCollection.GetDeviceSnapshot("device1", "type1")
	assert.NoError(t, err, "应能获取设备快照")
	value, exists := snapshot.GetField("temperature")
	assert.True(t, exists, "应有 'temperature' 字段")
	assert.Equal(t, 25, value, "字段 'temperature' 应为 25")
}
