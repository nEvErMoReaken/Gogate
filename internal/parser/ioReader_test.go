package parser

import (
	"context"
	"gateway/internal/pkg"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

func TestNewIoReader(t *testing.T) {
	// 模拟数据源
	reader := strings.NewReader("mock data")
	dataSource := pkg.DataSource{Source: reader, MetaData: map[string]interface{}{"deviceId": "testDevice"}}
	mapChan := make(map[string]chan pkg.Point)
	ctx := context.Background()

	// 模拟配置
	config := &pkg.Config{
		Parser: pkg.ParserConfig{
			Para: map[string]interface{}{
				"dir":       "./testdir",
				"protoFile": "testProtoFile",
			},
		},
		Others: map[string]interface{}{
			"testProtoFile": []interface{}{
				map[string]interface{}{
					"type":   "FixedLengthChunk",
					"length": 10,
					"sections": []interface{}{
						map[string]interface{}{
							"from": map[string]interface{}{
								"byte":   1,
								"repeat": 1,
							},
							"to": map[string]interface{}{
								"device": "device1",
								"type":   "type1",
								"fields": []string{"field1"},
							},
							"decoding": map[string]interface{}{
								"method": "mockDecode",
							},
						},
					},
				},
			},
		},
	}
	ctx = pkg.WithConfig(ctx, config)

	// 创建 IoReader
	parser, err := NewIoReader(dataSource, mapChan, ctx)
	assert.NoError(t, err, "解析器初始化不应出现错误")
	assert.NotNil(t, parser, "解析器应成功创建")
	assert.IsType(t, &IoReader{}, parser, "解析器类型应为 IoReader")
}

func TestFixedLengthChunk_Process(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "ts", time.Now())
	ctx = context.WithValue(ctx, "data_source", "mock_source")

	reader := strings.NewReader("mockdata") // 模拟输入流
	frame := make([]byte, 0)
	snapshotCollection := make(SnapshotCollection)

	// 定义解码函数
	mockDecodeFunc := func(data []byte) ([]interface{}, error) {
		return []interface{}{"decoded_value"}, nil
	}

	// 创建 FixedLengthChunk
	fixedChunk := FixedLengthChunk{
		length: 8,
		Sections: []Section{
			{
				Repeat:       1,
				Length:       1,
				Decoding:     mockDecodeFunc,
				ToDeviceName: "device1",
				ToDeviceType: "type1",
				ToFieldNames: []string{"field1"},
			},
		},
	}

	// 处理数据
	changedCtx, err := fixedChunk.Process(reader, &frame, snapshotCollection, ctx)
	assert.NoError(t, err, "应成功处理定长数据块")
	assert.NotNil(t, changedCtx, "上下文应被更新")

	// 检查帧数据
	assert.Equal(t, []byte("mockdata"), frame, "帧数据应正确追加")

	// 检查快照数据是否正确更新
	snapshot, err := snapshotCollection.GetDeviceSnapshot("device1", "type1")
	assert.NoError(t, err, "应成功获取设备快照")
	value, exists := snapshot.GetField("field1")
	assert.True(t, exists, "应存在字段 field1")
	assert.Equal(t, "decoded_value", value, "字段 field1 的值应为 'decoded_value'")
}

func TestIoReaderStart(t *testing.T) {
	// 模拟数据源
	errChan := make(chan error, 5)
	reader := strings.NewReader("mockdata")
	dataSource := pkg.DataSource{Source: reader, MetaData: map[string]interface{}{"deviceId": "testDevice", "ts": time.Now()}}
	mapChan := make(map[string]chan pkg.Point)
	mapChan["influxdb"] = make(chan pkg.Point, 1)
	ctx, cancel := context.WithCancel(context.Background())
	ctx = pkg.WithErrChan(ctx, errChan)

	// 模拟配置
	config := &pkg.Config{
		Parser: pkg.ParserConfig{
			Para: map[string]interface{}{
				"dir":       "./testdir",
				"protoFile": "testProtoFile",
			},
		},
		Strategy: []pkg.StrategyConfig{
			{
				Type:   "influxdb",
				Enable: true,
			},
		},
		Others: map[string]interface{}{
			"testProtoFile": []interface{}{
				map[string]interface{}{
					"type":   "FixedLengthChunk",
					"length": 8,
					"sections": []interface{}{
						map[string]interface{}{
							"from": map[string]interface{}{
								"byte":   1,
								"repeat": 1,
							},
							"to": map[string]interface{}{
								"device": "device1",
								"type":   "type1",
								"fields": []string{"field1"},
							},
							"decoding": map[string]interface{}{
								"method": "mockDecode",
							},
						},
					},
				},
			},
		},
	}
	ctx = pkg.WithConfig(ctx, config)

	// 模拟解码函数
	ByteScriptFuncCache["mockDecode"] = func(data []byte) ([]interface{}, error) {
		return []interface{}{"decoded_value"}, nil
	}

	// 创建 IoReader
	parser, _ := NewIoReader(dataSource, mapChan, ctx)
	ioReader := parser.(*IoReader)

	// 启动解析器
	go ioReader.Start()

	// 等待并消费数据
	select {
	case point := <-mapChan["influxdb"]:
		assert.Equal(t, "device1", point.DeviceName, "设备名称应为 'device1'")
		assert.Equal(t, "type1", point.DeviceType, "设备类型应为 'type1'")
		assert.Equal(t, "decoded_value", point.Field["field1"], "字段 'field1' 应为 'decoded_value'")
	case err := <-errChan:
		t.Fatalf("解析器发生错误: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("超时: 未能从 mapChan 获取到数据点")
	}

	// 取消上下文，停止解析器
	cancel()
}

func TestExpandFieldTemplate(t *testing.T) {
	fields, err := expandFieldTemplate("field${1..3}")
	assert.NoError(t, err, "应成功解析模板")
	assert.Equal(t, []string{"field1", "field2", "field3"}, fields, "应成功展开模板")
}

func TestParseVariable(t *testing.T) {
	isVar, varName, err := parseVariable("${var1}")
	assert.NoError(t, err, "应成功解析变量")
	assert.True(t, isVar, "应识别为变量")
	assert.Equal(t, "var1", varName, "变量名应为 'var1'")
}

func TestCreateFixedLengthChunk(t *testing.T) {
	// 模拟 FixedLengthChunk 的配置
	chunkConfig := map[string]interface{}{
		"type":   "FixedLengthChunk",
		"length": 10,
		"sections": []interface{}{
			map[string]interface{}{
				"from": map[string]interface{}{
					"byte":   1,
					"repeat": 1,
				},
				"to": map[string]interface{}{
					"device": "device1",
					"type":   "type1",
					"fields": []string{"field1"},
				},
				"decoding": map[string]interface{}{
					"method": "mockDecode",
				},
			},
		},
	}

	// 调用 createChunk 解析
	chunk, err := createChunk(chunkConfig)

	// 检查是否成功解析
	assert.NoError(t, err, "应成功解析 FixedLengthChunk 配置")
	assert.NotNil(t, chunk, "chunk 不应为 nil")

	// 检查返回的 chunk 是否为 FixedLengthChunk 类型
	fixedChunk, ok := chunk.(*FixedLengthChunk)
	assert.True(t, ok, "chunk 类型应为 FixedLengthChunk")

	// 验证 fixedChunk 的长度和 sections 的配置
	assert.Equal(t, 10, fixedChunk.length, "FixedLengthChunk 的 length 应为 10")
	assert.Equal(t, 1, fixedChunk.Sections[0].Length, "第一个 Section 的长度应为 1")
	assert.Equal(t, "device1", fixedChunk.Sections[0].ToDeviceName, "设备名称应为 'device1'")
	assert.Equal(t, "field1", fixedChunk.Sections[0].ToFieldNames[0], "字段名称应为 'field1'")
}

func TestParseToDeviceName(t *testing.T) {
	// 创建上下文，并设置变量
	ctx := context.Background()
	ctx = context.WithValue(ctx, "vobc_id", "1234")

	// 创建 Section 实例，并设置 ToDeviceName 带有模板变量
	section := Section{
		ToDeviceName: "ecc_${vobc_id}", // 需要解析 vobc_id
	}

	// 调用 parseToDeviceName 进行解析
	deviceName, err := section.parseToDeviceName(ctx)

	// 检查是否正确解析
	assert.NoError(t, err, "应成功解析模板变量")
	assert.Equal(t, "ecc_1234", deviceName, "设备名称应解析为 'ecc_1234'")

	// 测试没有模板变量的情况下
	section.ToDeviceName = "simple_device"
	deviceName, err = section.parseToDeviceName(ctx)
	assert.NoError(t, err, "应成功解析简单设备名称")
	assert.Equal(t, "simple_device", deviceName, "设备名称应为 'simple_device'")

	// 测试未找到模板变量的情况
	section.ToDeviceName = "ecc_${unknown_var}"
	deviceName, err = section.parseToDeviceName(ctx)
	assert.Error(t, err, "应返回错误，因变量不存在")
}

func TestFixedLengthChunkString(t *testing.T) {
	// 创建一个 FixedLengthChunk 实例，模拟 Sections 和 length
	fixedChunk := FixedLengthChunk{
		length: 8,
		Sections: []Section{
			{
				Repeat:       1,
				Length:       2,
				ToDeviceName: "device1",
				ToDeviceType: "type1",
				ToVarNames:   []string{"var1"},
				ToFieldNames: []string{"field1"},
			},
			{
				Repeat:       2,
				Length:       4,
				ToDeviceName: "device2",
				ToDeviceType: "type2",
				ToVarNames:   []string{"var2"},
				ToFieldNames: []string{"field2"},
			},
		},
	}

	// 调用 String 方法
	chunkString := fixedChunk.String()

	// 检查输出是否包含重要信息
	assert.Contains(t, chunkString, "DeviceName=device1", "输出应包含 Section 1 的设备名称")
	assert.Contains(t, chunkString, "FieldTarget=[field1]", "输出应包含 Section 1 的字段名称")

	assert.Contains(t, chunkString, "DeviceName=device2", "输出应包含 Section 2 的设备名称")
	assert.Contains(t, chunkString, "FieldTarget=[field2]", "输出应包含 Section 2 的字段名称")
}

func TestGetIntVar(t *testing.T) {
	// 创建带上下文变量的 context
	ctx := context.Background()
	ctx = context.WithValue(ctx, "var1", 123)
	ctx = context.WithValue(ctx, "var2", "not an int")

	// 1. 直接传递整数
	val, err := getIntVar(ctx, 10)
	assert.NoError(t, err, "直接传递整数不应返回错误")
	assert.Equal(t, 10, val, "返回的值应为传递的整数")

	// 2. 通过变量名从上下文获取整数
	val, err = getIntVar(ctx, "var1")
	assert.NoError(t, err, "应成功从上下文中获取变量 'var1'")
	assert.Equal(t, 123, val, "上下文变量 'var1' 的值应为 123")

	// 3. 传递不存在的变量名
	val, err = getIntVar(ctx, "var3")
	assert.Error(t, err, "应返回错误，因上下文中未找到 'var3'")
	assert.Equal(t, 0, val, "当变量不存在时，返回值应为 0")

	// 4. 上下文中的变量类型不匹配
	val, err = getIntVar(ctx, "var2")
	assert.Error(t, err, "应返回错误，因上下文变量类型错误")
	assert.Equal(t, 0, val, "当变量类型不匹配时，返回值应为 0")

	// 5. 传递未知类型
	val, err = getIntVar(ctx, 12.34) // 传递 float64 类型
	assert.Error(t, err, "应返回错误，因传递了未知类型")
	assert.Equal(t, 0, val, "当传递未知类型时，返回值应为 0")
}
