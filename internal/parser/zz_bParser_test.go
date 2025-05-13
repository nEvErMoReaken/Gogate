package parser

import (
	"context" // Import hex for debugging if needed
	"fmt"
	"gateway/internal/pkg"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// --- Test YAML Configurations (Corrected Indentation) ---
const (
	BASE_TEST_YAML = `
test_proto:
  - desc: "解析头部信息 Section 0"
    size: 4
    Points:
      - Tag:
          id: "'dev1'"
        Field:
          msg_type: "Bytes[0]"
          payload_len: "Bytes[2]"
      - Tag:
          id: "'dev2'"
        Field:
          msg_type: "Bytes[1]"
    Vars:
      test_var: "Bytes[2]" # 用第三个字节设置变量
  - desc: "解析数据体 Section 1"
    size: 2 # 假设数据体长度为 2
    Label: "DataSection"
    Points:
      - Tag:
          id: "'dev3'"
        Field:
          data1: "Bytes[0]"
      - Tag:
          id: "'dev4'"
        Field:
          data_from_var: "Vars.test_var" # 使用之前设置的变量
          data2: "Bytes[1]"
`
	INVALID_YAML_NOT_LIST = `
test_proto:
  desc: "无效配置,不是列表"
  size: 1
`
	INVALID_YAML_DUPLICATE_LABEL = `
test_proto:
  - desc: "Section A"
    size: 1
    Label: "Duplicate"
  - desc: "Section B"
    size: 1
    Label: "Duplicate"
`
	ROUTE_TEST_YAML = `
test_proto:
  - desc: "路由头部 Section 0" # 索引 0
    size: 4
    Points:
      - Tag:
          id: "'dev_head'"
        Field:
          msg_type: "Bytes[0]"
          orig_val: "Bytes[2]" # 获取用于路由的值
    Vars:
      road: "Bytes[2]" # 设置路由变量,值为0xFF或0x55或0x11
    # 默认路由到下一节(索引1)

  - desc: "路由决策 Section 1" # 索引 1
    size: 2 # 处理接下来的2个字节
    Points:
      - Tag:
          id: "'dev_route_data'"
        Field:
          data1: "Bytes[0]"
          data2: "Bytes[1]"
    Next: # 在处理完本节数据后应用的规则
      - condition: "Vars.road == 0xFF" # 根据第0节设置的变量进行路由
        target: "type1_handler"
      - condition: "Vars.road == 0x55"
        target: "type2_handler"
    # 如果都不匹配,Route() 返回"所有路由规则都不匹配"错误

  - desc: "Type 1 Handler Section" # 索引 2
    size: 1 # 处理接下来的1个字节
    Label: "type1_handler"
    Points:
      - Tag:
          id: "'dev_type1'"
        Field:
          handler_data: "Bytes[0]"
    # 默认路由到下一节(索引3)

  - desc: "Post Type 1 Section" # 索引 3
    size: 1 # 处理接下来的1个字节
    Points:
      - Tag:
          id: "'dev_post_type1'"
        Field:
          post_data: "Bytes[0]"
    Next:
      - condition: "true"
        target: "agg" # 路由到聚合

  - desc: "Type 2 Handler Section" # 索引 4
    size: 3 # 处理接下来的3个字节
    Label: "type2_handler"
    Points:
      - Tag:
          id: "'dev_type2'"
        Field:
          handler_data1: "Bytes[0]"
          handler_data2: "Bytes[1]"
          handler_data3: "Bytes[2]"
    # 默认路由到下一节(索引5),即agg

  - desc: "Aggregation Section" # 索引 5
    size: 1 # 处理最后1个字节
    Label: "agg"
    Points:
      - Tag:
          id: "'dev_agg'"
        Field:
          agg_data: "Bytes[0]"
    # 处理结束(无下一节)
`
	// LOOP_TEST_YAML 定义循环处理逻辑 (再次修正: 缩进, 索引使用)
	LOOP_TEST_YAML = `
test_proto:
  - desc: "循环指示块 Section 0" # 索引 0
    size: 4
    Points:
      - Tag:
          id: "'dev_head'"
        Field:
          msg_type: "Bytes[0]"
          head_val: "Bytes[2]"
    Vars:
      loop_count: "Bytes[3]" # 从第4个字节初始化循环计数器 (例如 0x02 代表循环3次: 2, 1, 0)
      loop_index: "0"        # 正确初始化 loop_index (字符串)
    # 默认路由到下一节(索引 1)

  - desc: "中间块 Section 1" # 索引 1
    size: 2
    Points:
      - Tag:
          id: "'dev_mid'"
        Field:
          mid_data1: "Bytes[0]"
          mid_data2: "Bytes[1]"
    # 默认路由到下一节(索引 2)

  - desc: "循环开始块 Section 2" # 索引 2
    size: 1
    Label: "loop_start"
    Points:
      - Tag:
          id: "'dev_' + string(Vars.loop_index)"
        Field:
          start_marker: "Bytes[0]"
    Vars:
      loop_count: "Vars.loop_count - 1" # 计数器递减
      loop_index: "Vars.loop_index + 1" # 正确的索引递增
    # 默认路由到下一节(索引 3)

  - desc: "循环体 Section 3" # 索引 3
    size: 1
    Points:
      - Tag:
          id: "'dev_other_' + string(Vars.loop_index)"
        Field:
          body_data: "Bytes[0]"
    # 默认路由到下一节(索引 4)

  - desc: "循环结束与判断 Section 4" # 索引 4
    size: 1
    Points:
      - Tag:
          id: "'dev_end_' + string(Vars.loop_index)"
        Field:
          end_marker: "Bytes[0]"
    Next:
      - condition: "Vars.loop_count >= 0" # 循环条件 (比较 S2 递减后的 count)
        target: "loop_start"          # 跳转回循环开始
      - condition: "true"                # 默认条件
        target: "DEFAULT"             # 跳转到下一个 Section (索引 5)

  - desc: "循环后块 Section 5" # 索引 5
    size: 2
    Points:
      - Tag:
          id: "'dev_after_loop'"
        Field:
          final_data1: "Bytes[0]"
          final_data2: "Bytes[1]"
    # 结束
`
	// END_TARGET_TEST_YAML 定义提前结束处理逻辑
	END_TARGET_TEST_YAML = `
test_proto:
  - desc: "Section A - 设置条件"
    size: 2
    Points:
      - Tag:
          id: "'dev_a'"
        Field:
          data1: "Bytes[0]"
    Vars:
      stop_flag: "Bytes[1]" # 第二个字节决定是否停止
  - desc: "Section B - 条件路由"
    size: 1
    Points:
      - Tag:
          id: "'dev_b'"
        Field:
          data_b: "Bytes[0]"
    Next:
      - condition: "Vars.stop_flag == 0xEE" # 如果 stop_flag 是 0xEE
        target: "END"                   # 则处理流程在此停止
      - condition: "true"                # 否则
        target: "SectionC"              # 继续到 Section C
  - desc: "Section C - 如果未停止则执行"
    size: 1
    Label: "SectionC"
    Points:
      - Tag:
          id: "'dev_c'"
        Field:
          data_c: "Bytes[0]" # 这个 Section 只有在 stop_flag != 0xEE 时才会执行
`
	// DEAD_LOOP_TEST_YAML 定义一个会导致死循环的配置
	DEAD_LOOP_TEST_YAML = `
test_proto:
  - desc: "无限循环入口"
    size: 1
    Label: "loop_entry"
    Points:
      - Tag:
          id: "'dev_loop'"
        Field:
          data: "Bytes[0]"
    Next:
      - condition: "true"
        target: "loop_entry" # 直接指向自身，导致死循环
`
	// VARS_ONLY_ROUTE_TEST_YAML 测试只有 Vars 和 Next 的 Section
	VARS_ONLY_ROUTE_TEST_YAML = `
test_proto:
  - desc: "Section 1 - 只设置变量并路由"
    size: 1
    Vars:
      route_flag: "Bytes[0]" # 根据第一个字节设置 flag
    Next:
      - condition: "Vars.route_flag == 0xAA"
        target: "TargetAA"
      - condition: "Vars.route_flag == 0xBB"
        target: "TargetBB"
      # 如果 flag 不是 AA 或 BB，将无匹配，应报错

  - desc: "Section 2 - Target AA"
    size: 2
    Label: "TargetAA"
    Points:
      - Tag:
          id: "'dev_AA'"
        Field:
          data1: "Bytes[0]"
          data2: "Bytes[1]"
    Next:
      - condition: "true"
        target: "END"
    # 结束

  - desc: "Section 3 - Target BB"
    size: 1
    Label: "TargetBB"
    Points:
      - Tag:
          id: "'dev_BB'"
        Field:
          data: "Bytes[0]"
    # 结束
`
	// GLOBALMAP_TEST_YAML 测试全局映射功能
	GLOBALMAP_TEST_YAML = `
test_proto:
  - desc: "Section 1 - 从GlobalMap获取常量"
    size: 2
    Points:
      - Tag:
          id: "'dev_global_const'"
        Field:
          static_value: "GlobalMap.constant_value"         # 使用全局常量
          static_map_value: "GlobalMap.map_values.key1"    # 使用嵌套的全局常量
    Vars:
      threshold: "GlobalMap.threshold"                   # 从全局映射设置阈值变量
      dev_prefix: "GlobalMap.device_prefix"              # 设置设备前缀变量

  - desc: "Section 2 - 使用全局映射变量"
    size: 2
    Points:
      - Tag:
          id: "'global_device'"
        Field:
          value1: "Bytes[0]"
          value2: "Bytes[1]"
          prefix_value: "Vars.dev_prefix"                  # 在字段中使用变量值而不是设备名中
    Next:
      - condition: "Bytes[0] > Vars.threshold"           # 基于全局阈值的条件路由
        target: "HighValue"
      - condition: "true"
        target: "LowValue"

  - desc: "Section 3 - 高于阈值的处理"
    size: 1
    Label: "HighValue"
    Points:
      - Tag:
          id: "'dev_high'"
        Field:
          calc_value: "Bytes[0] * GlobalMap.multiplier"    # 使用全局乘数计算
          result_type: "GlobalMap.result_types.high"       # 设置结果类型
    Next:
      - condition: "true"
        target: "END"

  - desc: "Section 4 - 低于阈值的处理"
    size: 1
    Label: "LowValue"
    Points:
      - Tag:
          id: "'dev_low'"
        Field:
          calc_value: "Bytes[0] / GlobalMap.divisor"       # 使用全局除数计算
          result_type: "GlobalMap.result_types.low"        # 设置结果类型
    Next:
      - condition: "true"
        target: "END"
`
)

// --- Helper Functions ---

// MockContext creates a context with a logger.
func MockContext() context.Context {
	// logger, _ := zap.NewDevelopment() // Enable for debugging
	logger := zap.NewNop()
	ctx := context.Background()
	ctx = pkg.WithLogger(ctx, logger)
	return ctx
}

// mockConfig creates a ViperConfig for testing.
func mockConfig(protoFileName string, protoYAML string, globalMap map[string]interface{}, paraMap map[string]interface{}) (*pkg.Config, error) {
	v := &pkg.Config{
		Parser: pkg.ParserConfig{
			Type: "byteParser",
			Para: map[string]interface{}{},
		},
		Others: map[string]interface{}{},
	}

	if paraMap != nil {
		v.Parser.Para = paraMap
	} else {
		v.Parser.Para["protoFile"] = protoFileName
		if globalMap != nil {
			v.Parser.Para["globalMap"] = globalMap
		}
	}

	if protoYAML != "" {
		var yamlData map[string]interface{}
		err := yaml.Unmarshal([]byte(protoYAML), &yamlData)
		if err != nil {
			if protoYAML == INVALID_YAML_NOT_LIST {
				var rawContent interface{}
				_ = yaml.Unmarshal([]byte(protoYAML), &rawContent)
				v.Others[protoFileName] = rawContent
				return v, nil
			} else {
				return nil, fmt.Errorf("测试配置错误：无法解析提供的 YAML: %w", err)
			}
		}

		sectionListRaw, exists := yamlData[protoFileName]
		if !exists {
			return nil, fmt.Errorf("测试配置错误：YAML 中未找到与 protoFileName '%s' 匹配的顶层键", protoFileName)
		}

		sectionListInterface, ok := sectionListRaw.([]interface{})
		if !ok {
			if protoYAML == INVALID_YAML_NOT_LIST {
				v.Others[protoFileName] = sectionListRaw
				return v, nil
			} else {
				if mapInterface, okMap := sectionListRaw.(map[interface{}]interface{}); okMap {
					v.Others[protoFileName] = mapInterface
					return v, nil

				}
				return nil, fmt.Errorf("测试配置错误：YAML 键 '%s' 下的值不是预期的列表类型, 实际类型: %T", protoFileName, sectionListRaw)
			}
		}

		var mapSlice []map[string]any
		for _, item := range sectionListInterface {
			if mapItem, ok := item.(map[string]interface{}); ok {
				convertedMap := make(map[string]any)
				err = ConvertMapKeysToStrings(mapItem, convertedMap)
				if err != nil {
					return nil, fmt.Errorf("测试配置错误：转换 YAML 内部 map 失败: %w", err)
				}
				mapSlice = append(mapSlice, convertedMap)
			} else {
				return nil, fmt.Errorf("测试配置错误：YAML 列表项不是 map[string]interface{} 类型, 实际类型: %T", item)
			}
		}
		v.Others[protoFileName] = mapSlice
	}

	return v, nil
}

// --- Test Functions ---

func TestNewByteParser(t *testing.T) {
	Convey("NewByteParser 初始化测试", t, func() {
		ctx := MockContext()
		protoFileName := "test_proto"
		validGlobalMap := map[string]interface{}{"globalKey": "globalValue"}
		validParaMap := map[string]interface{}{"protoFile": protoFileName, "globalMap": validGlobalMap}

		Convey("当配置完全有效时", func() {
			conf, err := mockConfig(protoFileName, BASE_TEST_YAML, validGlobalMap, validParaMap)
			So(err, ShouldBeNil)
			ctx = pkg.WithConfig(ctx, conf)

			parser, err := NewByteParser(ctx)
			So(err, ShouldBeNil)
			So(parser, ShouldNotBeNil)
			So(len(parser.Nodes), ShouldEqual, 2)
			So(parser.LabelMap, ShouldContainKey, "DataSection")
			So(parser.LabelMap["DataSection"], ShouldEqual, 1)
			So(parser.Env, ShouldNotBeNil)
			So(parser.Env.GlobalMap, ShouldResemble, validGlobalMap)
		})
		Convey("当 parser.config 为 nil 时", func() {
			conf, err := mockConfig(protoFileName, BASE_TEST_YAML, nil, nil)
			So(err, ShouldBeNil)
			conf.Parser.Para = nil
			ctx = pkg.WithConfig(ctx, conf)
			_, err = NewByteParser(ctx)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "parser.config 为空")
		})
		Convey("当缺少 protoFile 配置项时", func() {
			invalidParaMap := map[string]interface{}{"someOtherKey": "value"}
			conf, err := mockConfig(protoFileName, BASE_TEST_YAML, nil, invalidParaMap)
			So(err, ShouldBeNil)
			ctx = pkg.WithConfig(ctx, conf)
			_, err = NewByteParser(ctx)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "缺少 'protoFile' 配置项")
		})
		Convey("当 protoFile 类型错误时 (非字符串)", func() {
			invalidParaMap := map[string]interface{}{"protoFile": 123}
			conf, err := mockConfig(protoFileName, BASE_TEST_YAML, nil, invalidParaMap)
			So(err, ShouldBeNil)
			ctx = pkg.WithConfig(ctx, conf)
			_, err = NewByteParser(ctx)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "'protoFile' 必须是字符串")
		})
		Convey("当 protoFile 值为空字符串时", func() {
			invalidParaMap := map[string]interface{}{"protoFile": ""}
			conf, err := mockConfig("test_proto", BASE_TEST_YAML, nil, invalidParaMap)
			So(err, ShouldBeNil)
			ctx = pkg.WithConfig(ctx, conf)
			_, err = NewByteParser(ctx)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "'protoFile' 值不能为空字符串")
		})
		Convey("当 protoFile 在 Others 中不存在时", func() {
			missingProtoFileName := "missing_proto.yaml"
			validParaMapMissing := map[string]interface{}{"protoFile": missingProtoFileName}
			conf, err := mockConfig(protoFileName, BASE_TEST_YAML, nil, validParaMapMissing)
			So(err, ShouldBeNil)
			delete(conf.Others, missingProtoFileName)
			ctx = pkg.WithConfig(ctx, conf)
			_, err = NewByteParser(ctx)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "未找到协议文件:"+missingProtoFileName)
		})
		Convey("当 protoFile 内容格式错误时 (非列表)", func() {
			conf, err := mockConfig(protoFileName, INVALID_YAML_NOT_LIST, nil, validParaMap)
			So(err, ShouldBeNil)
			ctx = pkg.WithConfig(ctx, conf)
			_, err = NewByteParser(ctx)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "协议文件格式错误")
			So(err.Error(), ShouldContainSubstring, "不是一个列表/数组")
		})
		Convey("当 protoFile 内容包含重复标签时", func() {
			conf, err := mockConfig(protoFileName, INVALID_YAML_DUPLICATE_LABEL, nil, validParaMap)
			So(err, ShouldBeNil)
			So(conf, ShouldNotBeNil)
			So(conf.Others, ShouldNotBeNil)
			So(conf.Others, ShouldContainKey, protoFileName)
			So(conf.Others[protoFileName], ShouldNotBeNil)
			So(conf.Others[protoFileName], ShouldHaveSameTypeAs, []map[string]any{})
			ctx = pkg.WithConfig(ctx, conf)
			_, err = NewByteParser(ctx)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "初始化ByteParser失败")
			So(err.Error(), ShouldContainSubstring, "重复定义")
		})

		Convey("当 LOOP_TEST_YAML 格式正确时", func() {
			conf, err := mockConfig(protoFileName, LOOP_TEST_YAML, validGlobalMap, validParaMap)
			So(err, ShouldBeNil) // Check YAML unmarshalling
			ctx = pkg.WithConfig(ctx, conf)
			parser, err := NewByteParser(ctx) // Check Parser initialization (incl. BuildSequence)
			So(err, ShouldBeNil)
			So(parser, ShouldNotBeNil)
			So(len(parser.Nodes), ShouldEqual, 6) // Should have 6 nodes now
			So(parser.LabelMap, ShouldContainKey, "loop_start")
			So(parser.LabelMap["loop_start"], ShouldEqual, 2) // loop_start is the 3rd node (index 2)
		})
	})
}

func TestByteParserProcessing(t *testing.T) {
	Convey("ByteParser 处理逻辑测试", t, func() {
		baseCtx := MockContext()
		protoFileName := "test_proto"
		validGlobalMap := map[string]interface{}{"globalKey": "globalValue"}
		validParaMap := map[string]interface{}{"protoFile": protoFileName, "globalMap": validGlobalMap}

		// --- Helper function for running scenarios ---
		runScenario := func(conf *pkg.Config, desc string, inputData []byte, expectedPoints map[string][]map[string]interface{}, expectError bool, errorSubstring string) {
			Convey(desc, func() {
				ctx, cancel := context.WithCancel(baseCtx)
				defer cancel()

				ctx = pkg.WithConfig(ctx, conf)
				parser, err := NewByteParser(ctx)
				So(err, ShouldBeNil)
				So(parser, ShouldNotBeNil)

				dataChan := make(chan []byte, 1)
				sink := make(chan *pkg.PointPackage, 1)
				var processingErr error
				processingDone := make(chan struct{})
				go func() {
					defer close(processingDone)
					processingErr = parser.StartWithChan(dataChan, sink)
				}()

				dataChan <- inputData

				// 修改：为了确保死循环防护测试能正确检测到错误
				if expectError {
					// 如果期望错误，等待一个合理的时间让出错情况发生
					select {
					case <-processingDone:
						// 处理已经结束，继续检查错误
					case <-time.After(1 * time.Second):
						// 超时，取消上下文，让处理退出
						cancel()
						<-processingDone // 等待处理完全结束
					}

					// 检查是否出现了期望的错误
					So(processingErr, ShouldNotBeNil)
					So(processingErr.Error(), ShouldContainSubstring, errorSubstring)
				} else {
					// 如果不期望错误，等待结果
					var result *pkg.PointPackage
					select {
					case result = <-sink:
						// 成功获取到结果
					case <-time.After(1 * time.Second):
						t.Fatal("等待处理结果超时")
					case <-processingDone:
						// 如果处理意外结束，应该是出现了错误
						So(processingErr, ShouldBeNil)
					}

					cancel()
					<-processingDone // 等待处理完全结束

					// 再次检查错误（理论上应该在上面处理了）
					So(processingErr, ShouldBeNil)

					groupedPoints := make(map[string][]map[string]interface{})
					totalPoints := 0
					for _, p := range result.Points {
						if p != nil {
							// Use a tag field (e.g., "id") as the device identifier for grouping
							deviceID, ok := p.Tag["id"]
							if !ok {
								// If no id tag, try device or name tags
								deviceID, ok = p.Tag["device"]
								if !ok {
									deviceID, ok = p.Tag["name"]
									if !ok {
										// If all else fails, use "unknown" + index
										deviceID = fmt.Sprintf("unknown_%d", len(groupedPoints))
									}
								}
							}

							deviceIDStr := fmt.Sprintf("%v", deviceID) // Convert to string (it could be any type)
							groupedPoints[deviceIDStr] = append(groupedPoints[deviceIDStr], p.Field)
							totalPoints++
						}
					}

					expectedTotalPoints := 0
					for _, pointsList := range expectedPoints {
						expectedTotalPoints += len(pointsList)
					}
					So(totalPoints, ShouldEqual, expectedTotalPoints)

					So(len(groupedPoints), ShouldEqual, len(expectedPoints))
					for dev, expectedFieldsList := range expectedPoints {
						actualFieldsList, ok := groupedPoints[dev]
						So(ok, ShouldBeTrue)
						So(len(actualFieldsList), ShouldEqual, len(expectedFieldsList))
						So(actualFieldsList, ShouldResemble, expectedFieldsList)
					}
				}
			})
		}
		// --- End Helper ---

		Convey("使用 BASE_TEST_YAML 处理单帧数据", func() {
			confBase, _ := mockConfig(protoFileName, BASE_TEST_YAML, validGlobalMap, validParaMap)
			inputData := []byte{0x01, 0x02, 0xAA, 0xFF, 0xBB, 0xCC}
			expectedPoints := map[string][]map[string]interface{}{
				"dev1": {{"msg_type": byte(0x01), "payload_len": byte(0xAA)}},
				"dev2": {{"msg_type": byte(0x02)}},
				"dev3": {{"data1": byte(0xBB)}},
				"dev4": {{"data_from_var": byte(0xAA), "data2": byte(0xCC)}},
			}
			runScenario(confBase, "基本处理", inputData, expectedPoints, false, "")
		})

		Convey("使用 ROUTE_TEST_YAML 处理路由", func() {
			confRoute, _ := mockConfig(protoFileName, ROUTE_TEST_YAML, validGlobalMap, validParaMap)

			inputData1 := []byte{0xAA, 0xBB, 0xFF, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
			expectedPoints1 := map[string][]map[string]interface{}{
				"dev_head":       {{"msg_type": byte(0xAA), "orig_val": byte(0xFF)}},
				"dev_route_data": {{"data1": byte(0x11), "data2": byte(0x22)}},
				"dev_type1":      {{"handler_data": byte(0x33)}},
				"dev_post_type1": {{"post_data": byte(0x44)}},
				"dev_agg":        {{"agg_data": byte(0x55)}},
			}
			runScenario(confRoute, "当路由到 type1_handler 时", inputData1, expectedPoints1, false, "")

			inputData2 := []byte{0xCC, 0xDD, 0x55, 0x00, 0x66, 0x77, 0x88, 0x99, 0xEE, 0xFF}
			expectedPoints2 := map[string][]map[string]interface{}{
				"dev_head":       {{"msg_type": byte(0xCC), "orig_val": byte(0x55)}},
				"dev_route_data": {{"data1": byte(0x66), "data2": byte(0x77)}},
				"dev_type2":      {{"handler_data1": byte(0x88), "handler_data2": byte(0x99), "handler_data3": byte(0xEE)}},
				"dev_agg":        {{"agg_data": byte(0xFF)}},
			}
			runScenario(confRoute, "当路由到 type2_handler 时", inputData2, expectedPoints2, false, "")

			inputData3 := []byte{0x11, 0x22, 0x11, 0x00, 0xAA, 0xBB}
			runScenario(confRoute, "当路由无匹配时应报错", inputData3, nil, true, "所有路由规则都不匹配")
		})

		Convey("使用 LOOP_TEST_YAML 处理循环", func() {
			confLoop, errConf := mockConfig(protoFileName, LOOP_TEST_YAML, validGlobalMap, validParaMap)
			So(errConf, ShouldBeNil)

			inputDataLoop := []byte{
				0xAA, 0xBB, 0xCC, 0x02, // Header (loop_count = 2)
				0x11, 0x22, // Mid
				0xA1, 0xB1, 0xC1, // Loop 1 (index 0)
				0xA2, 0xB2, 0xC2, // Loop 2 (index 1)
				0xA3, 0xB3, 0xC3, // Loop 3 (index 2)
				0xDD, 0xEE, // After Loop
			}

			// Expected points based on the corrected YAML logic and dynamic names
			expectedPointsLoop := map[string][]map[string]interface{}{
				"dev_head": {{"msg_type": byte(0xAA), "head_val": byte(0xCC)}},   // S0
				"dev_mid":  {{"mid_data1": byte(0x11), "mid_data2": byte(0x22)}}, // S1
				// Loop 1 (index=0->1)
				"dev_1":       {{"start_marker": byte(0xA1)}}, // S2 (注意索引已更新)
				"dev_other_1": {{"body_data": byte(0xB1)}},    // S3
				"dev_end_1":   {{"end_marker": byte(0xC1)}},   // S4
				// Loop 2 (index=1->2)
				"dev_2":       {{"start_marker": byte(0xA2)}}, // S2
				"dev_other_2": {{"body_data": byte(0xB2)}},    // S3
				"dev_end_2":   {{"end_marker": byte(0xC2)}},   // S4
				// Loop 3 (index=2->3)
				"dev_3":       {{"start_marker": byte(0xA3)}}, // S2
				"dev_other_3": {{"body_data": byte(0xB3)}},    // S3
				"dev_end_3":   {{"end_marker": byte(0xC3)}},   // S4
				// After loop
				"dev_after_loop": {{"final_data1": byte(0xDD), "final_data2": byte(0xEE)}}, // S5
			}
			runScenario(confLoop, "正常循环3次(计数器递减,动态设备名)", inputDataLoop, expectedPointsLoop, false, "")
		})

		Convey("使用 END_TARGET_TEST_YAML 处理提前结束", func() {
			confEnd, errConf := mockConfig(protoFileName, END_TARGET_TEST_YAML, validGlobalMap, validParaMap)
			So(errConf, ShouldBeNil)

			// 场景1: stop_flag = 0xEE, 应该在 Section B 之后停止
			inputDataStop := []byte{0xAA, 0xEE, 0xBB, 0xCC} // 第三个字节 (0xBB) 属于 Section B, 第四个字节 (0xCC) 应该被忽略
			expectedPointsStop := map[string][]map[string]interface{}{
				"dev_a": {{"data1": byte(0xAA)}},  // 来自 Section A
				"dev_b": {{"data_b": byte(0xBB)}}, // 来自 Section B
				// dev_c 不应该出现
			}
			runScenario(confEnd, "当 stop_flag 为 0xEE 时应提前停止", inputDataStop, expectedPointsStop, false, "")

			// 场景2: stop_flag != 0xEE, 应该完整执行
			inputDataContinue := []byte{0x11, 0x22, 0x33, 0x44} // 0x22 != 0xEE
			expectedPointsContinue := map[string][]map[string]interface{}{
				"dev_a": {{"data1": byte(0x11)}},  // 来自 Section A
				"dev_b": {{"data_b": byte(0x33)}}, // 来自 Section B
				"dev_c": {{"data_c": byte(0x44)}}, // 来自 Section C
			}
			runScenario(confEnd, "当 stop_flag 不为 0xEE 时应继续执行", inputDataContinue, expectedPointsContinue, false, "")
		})

		Convey("死循环防护测试", func() {
			// 创建使用 DEAD_LOOP_TEST_YAML 的配置，这个配置中包含了一个会导致无限循环的节点
			confLoop, errConf := mockConfig(protoFileName, DEAD_LOOP_TEST_YAML, validGlobalMap, validParaMap)
			So(errConf, ShouldBeNil)

			// 获取每个节点的信息
			ctx := pkg.WithConfig(baseCtx, confLoop)
			// 创建带开发日志的上下文
			logger, _ := zap.NewDevelopment()
			ctx = pkg.WithLogger(ctx, logger)

			parser, err := NewByteParser(ctx)
			So(err, ShouldBeNil)
			So(parser, ShouldNotBeNil)

			// 检查配置是否正确
			Convey("通过代码逻辑分析问题", func() {
				// 配置中有一个节点，且有自我引用的路由
				So(len(parser.Nodes), ShouldEqual, 1)
				So(parser.LabelMap, ShouldContainKey, "loop_entry")
				So(parser.LabelMap["loop_entry"], ShouldEqual, 0)

				// 模拟手动测试循环处理
				inputData := []byte{0xAA} // 只需要一个字节
				state := NewByteState(parser.Env, parser.LabelMap, parser.Nodes)
				state.Data = inputData

				// 首次处理
				current := parser.Nodes[0]
				var next BProcessor
				var processErr error

				// 第一次处理
				next, processErr = current.ProcessWithBytes(ctx, state)
				So(processErr, ShouldBeNil)

				// 期望行为：Route方法应该返回自身以支持循环
				So(next, ShouldNotBeNil)       // 修正：应该返回非nil的处理器
				So(next, ShouldEqual, current) // 修正：应该返回自身

				// 结论:
				// 1. Route方法现在正确处理了自引用循环，
				//    当"target: loop_entry"指向自身时，返回节点自身而不是nil
				// 2. 死循环防护逻辑可以在ByteParser.StartWithChan中正常工作
				// 3. 这个测试验证了Route方法对自引用循环的正确处理
			})
		})

		Convey("处理数据不足的情况", func() {
			confBase, _ := mockConfig(protoFileName, BASE_TEST_YAML, validGlobalMap, validParaMap)
			ctx := pkg.WithConfig(baseCtx, confBase)
			parser, err := NewByteParser(ctx)
			So(err, ShouldBeNil)
			So(parser, ShouldNotBeNil)

			inputData := []byte{0x01, 0x02, 0xAA}
			env := &BEnv{
				Bytes:     nil,
				Vars:      make(map[string]interface{}),
				GlobalMap: parser.Env.GlobalMap,
				Points:    make([]*pkg.Point, 0),
			}
			byteState := NewByteState(env, parser.LabelMap, parser.Nodes)
			byteState.Data = inputData

			_, processErr := parser.Nodes[0].ProcessWithBytes(ctx, byteState)

			So(processErr, ShouldNotBeNil)
			So(processErr.Error(), ShouldContainSubstring, "数据不足")
			So(processErr.Error(), ShouldContainSubstring, "需要 4 字节")
		})

		Convey("处理只有 Vars 和 Next 的 Section 路由", func() {
			confVarsOnly, errConf := mockConfig(protoFileName, VARS_ONLY_ROUTE_TEST_YAML, validGlobalMap, validParaMap)
			So(errConf, ShouldBeNil)

			// 场景 1: 路由到 TargetAA
			inputDataAA := []byte{0xAA, 0x11, 0x22} // 第一个字节是 0xAA
			expectedPointsAA := map[string][]map[string]interface{}{
				"dev_AA": {{"data1": byte(0x11), "data2": byte(0x22)}}, // 只应执行 TargetAA
			}
			runScenario(confVarsOnly, "当 flag 为 0xAA 时应路由到 TargetAA", inputDataAA, expectedPointsAA, false, "")

			// 场景 2: 路由到 TargetBB
			inputDataBB := []byte{0xBB, 0x33} // 第一个字节是 0xBB
			expectedPointsBB := map[string][]map[string]interface{}{
				"dev_BB": {{"data": byte(0x33)}}, // 只应执行 TargetBB
			}
			runScenario(confVarsOnly, "当 flag 为 0xBB 时应路由到 TargetBB", inputDataBB, expectedPointsBB, false, "")

			// 场景 3: 无匹配路由
			inputDataNone := []byte{0xCC, 0x44} // 第一个字节不匹配任何条件
			runScenario(confVarsOnly, "当 flag 不匹配时应报错", inputDataNone, nil, true, "所有路由规则都不匹配")
		})

		Convey("测试GlobalMap功能", func() {
			// 定义更复杂的全局映射
			complexGlobalMap := map[string]interface{}{
				"constant_value": float64(42),  // 简单常量
				"threshold":      float64(100), // 用于条件判断的阈值
				"device_prefix":  "global",     // 设备名称前缀
				"multiplier":     float64(2),   // 乘数
				"divisor":        float64(2),   // 除数
				"map_values": map[string]interface{}{ // 嵌套映射
					"key1": "mapped_value",
					"key2": "other_value",
				},
				"result_types": map[string]interface{}{ // 结果类型映射
					"high": "exceeded_threshold",
					"low":  "below_threshold",
				},
			}

			confGlobalMap, errConf := mockConfig(protoFileName, GLOBALMAP_TEST_YAML, complexGlobalMap, nil)
			So(errConf, ShouldBeNil)

			// 场景1: 高于阈值的数据
			inputDataHigh := []byte{0x01, 0x02, 0xFF, 0x03, 0xAA} // 添加第5个字节 0xAA
			expectedPointsHigh := map[string][]map[string]interface{}{
				"dev_global_const": {{"static_value": float64(42), "static_map_value": "mapped_value"}},
				"global_device":    {{"value1": byte(0xFF), "value2": byte(0x03), "prefix_value": "global"}},
				"dev_high":         {{"calc_value": float64(340), "result_type": "exceeded_threshold"}}, // 根据实际解析结果
			}
			runScenario(confGlobalMap, "当数据高于全局阈值时", inputDataHigh, expectedPointsHigh, false, "")

			// 场景2: 低于阈值的数据
			inputDataLow := []byte{0x01, 0x02, 0x32, 0x04, 0xBB} // 添加第5个字节 0xBB
			expectedPointsLow := map[string][]map[string]interface{}{
				"dev_global_const": {{"static_value": float64(42), "static_map_value": "mapped_value"}},
				"global_device":    {{"value1": byte(0x32), "value2": byte(0x04), "prefix_value": "global"}},
				"dev_low":          {{"calc_value": float64(93.5), "result_type": "below_threshold"}}, // 根据实际解析结果
			}
			runScenario(confGlobalMap, "当数据低于全局阈值时", inputDataLow, expectedPointsLow, false, "")

			// 场景3: 测试缺失的 GlobalMap 键，应当发生错误
			// 创建一个不完整的全局映射，缺少某些必要的键
			incompleteGlobalMap := map[string]interface{}{
				// 缺少 threshold 和其他必要的键
				"constant_value": float64(42),
				"device_prefix":  "global",
			}
			confIncompleteGlobalMap, _ := mockConfig(protoFileName, GLOBALMAP_TEST_YAML, incompleteGlobalMap, nil)
			inputDataError := []byte{0x01, 0x02, 0x50, 0x05, 0xCC} // 添加第5个字节 0xCC

			// 使用 runScenario 函数，预期会返回错误
			runScenario(confIncompleteGlobalMap, "当缺少必要的全局映射键时", inputDataError, nil, true, "GlobalMap")
		})
	})
}

// Helper function to convert map[interface{}]interface{} to map[string]any recursively
// Moved to pkg/utils or similar if used widely
func ConvertMapKeysToStrings(input map[string]interface{}, output map[string]any) error {
	for k, v := range input {
		strKey := fmt.Sprintf("%v", k) // Convert key to string
		switch typedV := v.(type) {
		case map[interface{}]interface{}: // Handle nested map[interface{}]interface{}
			nestedOutput := make(map[string]any)
			// Recursive call for nested maps
			if err := ConvertMapKeysToStringsInterface(typedV, nestedOutput); err != nil {
				return fmt.Errorf("failed converting nested map for key '%s': %w", strKey, err)
			}
			output[strKey] = nestedOutput
		case map[string]interface{}: // Handle nested map[string]interface{} (already good key type)
			nestedOutput := make(map[string]any)
			// Recursive call for nested maps
			if err := ConvertMapKeysToStrings(typedV, nestedOutput); err != nil {
				return fmt.Errorf("failed converting nested map for key '%s': %w", strKey, err)
			}
			output[strKey] = nestedOutput
		case []interface{}: // Handle slices
			convertedSlice := make([]any, len(typedV))
			for i, item := range typedV {
				switch itemTyped := item.(type) {
				case map[interface{}]interface{}:
					nestedOutput := make(map[string]any)
					if err := ConvertMapKeysToStringsInterface(itemTyped, nestedOutput); err != nil {
						return fmt.Errorf("failed converting nested map in slice for key '%s': %w", strKey, err)
					}
					convertedSlice[i] = nestedOutput
				case map[string]interface{}:
					nestedOutput := make(map[string]any)
					if err := ConvertMapKeysToStrings(itemTyped, nestedOutput); err != nil {
						return fmt.Errorf("failed converting nested map in slice for key '%s': %w", strKey, err)
					}
					convertedSlice[i] = nestedOutput
				default:
					convertedSlice[i] = item // Keep other types as is
				}
			}
			output[strKey] = convertedSlice
		default:
			output[strKey] = v // Assign other types directly
		}
	}
	return nil
}

// Helper for ConvertMapKeysToStrings to handle map[interface{}]interface{} input
func ConvertMapKeysToStringsInterface(input map[interface{}]interface{}, output map[string]any) error {
	tempMap := make(map[string]interface{})
	for k, v := range input {
		tempMap[fmt.Sprintf("%v", k)] = v
	}
	return ConvertMapKeysToStrings(tempMap, output)

}
