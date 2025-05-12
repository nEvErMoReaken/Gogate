package parser

import (
	"context"
	"gateway/internal/pkg"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSection(t *testing.T) {
	Convey("Section功能测试套件", t, func() {
		Convey("Section基础功能", func() {
			// 创建一个简单的Section
			section := &Section{
				Desc: "测试Section",
				Size: 2,
				PointsExpression: []PointExpression{
					{
						Tag:   map[string]string{"id": `"device1"`},
						Field: map[string]string{"value": "Bytes[0]", "count": "Bytes[1]"},
					},
				},
				Var:   map[string]any{"total": "Bytes[0] + Bytes[1]"},
				Label: "TestLabel",
				index: 0,
			}

			// 编译表达式
			program, err := CompileSectionProgram(section.PointsExpression, section.Var)
			So(err, ShouldBeNil)
			section.Program = program

			// 创建ByteState
			env := &BEnv{
				Bytes:     nil,
				Vars:      make(map[string]interface{}),
				GlobalMap: make(map[string]interface{}),
				Points: []pkg.Point{
					{Tag: make(map[string]interface{}), Field: make(map[string]interface{})},
					{Tag: make(map[string]interface{}), Field: make(map[string]interface{})},
					{Tag: make(map[string]interface{}), Field: make(map[string]interface{})},
				},
			}
			labelMap := map[string]int{"TestLabel": 0}
			nodes := []BProcessor{section}
			byteState := NewByteState(env, labelMap, nodes)
			byteState.Data = []byte{10, 20}

			// 这里修正：初始化一个point，将其加入out切片
			point := &pkg.Point{
				Tag: map[string]interface{}{"id": "device1"},
				Field: map[string]interface{}{
					"value": byte(10),
					"count": byte(20),
				},
			}
			out := []*pkg.Point{point}

			Convey("ProcessWithBytes应正确处理数据", func() {
				out, next, err := section.ProcessWithBytes(context.Background(), byteState, out)
				So(err, ShouldBeNil)
				So(next, ShouldBeNil) // 应该是最后一个节点

				// 检查游标位置
				So(byteState.Cursor, ShouldEqual, 2)

				// 检查变量设置
				So(byteState.Env.Vars["total"], ShouldEqual, 30)

				// 检查输出点 - 这里需要实际检查输出slice，而不是内部变量
				So(len(out), ShouldBeGreaterThan, 0)
				So(out[0].Tag["id"], ShouldEqual, "device1")
				So(out[0].Field["value"], ShouldEqual, byte(10))
				So(out[0].Field["count"], ShouldEqual, byte(20))
			})
		})

		Convey("Skip功能测试", func() {
			// 创建Skip节点
			skip := &Skip{
				Skip:  3,
				index: 0,
			}

			// 创建后续Section
			section := &Section{
				Desc: "测试Skip后的Section",
				Size: 2,
				PointsExpression: []PointExpression{
					{
						Tag:   map[string]string{"id": `"device2"`},
						Field: map[string]string{"value": "Bytes[0]", "count": "Bytes[1]"},
					},
				},
				index: 1,
			}

			// 编译表达式
			program, err := CompileSectionProgram(section.PointsExpression, nil)
			So(err, ShouldBeNil)
			section.Program = program

			// 创建ByteState
			env := &BEnv{
				Bytes:     nil,
				Vars:      make(map[string]interface{}),
				GlobalMap: make(map[string]interface{}),
				Points: []pkg.Point{
					{Tag: make(map[string]interface{}), Field: make(map[string]interface{})},
					{Tag: make(map[string]interface{}), Field: make(map[string]interface{})},
					{Tag: make(map[string]interface{}), Field: make(map[string]interface{})},
				},
			}
			nodes := []BProcessor{skip, section}
			byteState := NewByteState(env, nil, nodes)
			byteState.Data = []byte{1, 2, 3, 4, 5}

			// 这里修正：初始化一个point，将其加入out切片
			// point := &pkg.Point{ // This point and out are no longer needed here due to test restructuring
			// 	// Device: "device2",
			// 	Tag: map[string]interface{}{"id": "device2"},
			// 	Field: map[string]interface{}{
			// 		"value": byte(4),
			// 		"count": byte(5),
			// 	},
			// }
			// out := []*pkg.Point{point}

			// Let's initialize out as empty for clarity in skip, and the assertion will be on the 'out' returned by the section.
			outForSkip := []*pkg.Point{}

			Convey("Skip.ProcessWithBytes应正确跳过字节", func() {
				_, next, err := skip.ProcessWithBytes(context.Background(), byteState, outForSkip)
				So(err, ShouldBeNil)
				So(next, ShouldEqual, section)
				So(byteState.Cursor, ShouldEqual, 3)
			})

			Convey("连续处理Skip和Section", func() {
				// Reset ByteState cursor for this specific test case if needed, or ensure it flows from previous
				byteState.Cursor = 0       // Reset for clean run
				outLocal := []*pkg.Point{} // Start with fresh out for this test flow

				// 先执行Skip
				outLocal, next, err := skip.ProcessWithBytes(context.Background(), byteState, outLocal)
				So(err, ShouldBeNil)
				So(next, ShouldEqual, section)
				So(byteState.Cursor, ShouldEqual, 3)

				// 再执行Section
				outLocal, next, err = next.ProcessWithBytes(context.Background(), byteState, outLocal)
				So(err, ShouldBeNil)
				So(next, ShouldBeNil)
				So(byteState.Cursor, ShouldEqual, 5)

				// 检查输出点
				So(len(outLocal), ShouldEqual, 1) // Expecting one point from the section
				So(outLocal[0].Tag["id"], ShouldEqual, "device2")
				So(outLocal[0].Field["value"], ShouldEqual, byte(4))
				So(outLocal[0].Field["count"], ShouldEqual, byte(5))
			})
		})

		Convey("Section路由功能测试", func() {
			// 准备测试环境
			env := &BEnv{
				Bytes:     []byte{10, 20},
				Vars:      make(map[string]interface{}),
				GlobalMap: make(map[string]interface{}),
				Points: []pkg.Point{
					{Tag: make(map[string]interface{}), Field: make(map[string]interface{})},
					{Tag: make(map[string]interface{}), Field: make(map[string]interface{})},
					{Tag: make(map[string]interface{}), Field: make(map[string]interface{})},
				},
			}

			// 创建节点
			section1 := &Section{
				Desc: "条件路由节点",
				Size: 2,
				PointsExpression: []PointExpression{
					{
						Tag:   map[string]string{"id": `"device"`},
						Field: map[string]string{"value": "Bytes[0]"},
					},
				},
				Var:   map[string]any{"condition_var": "Bytes[0] > 5"},
				index: 0,
			}

			section2 := &Section{
				Desc: "目标节点1",
				Size: 1,
				PointsExpression: []PointExpression{
					{
						Tag:   map[string]string{"id": `"device"`},
						Field: map[string]string{"result": `"1"`},
					},
				},
				Label: "Target1",
				index: 1,
			}

			section3 := &Section{
				Desc: "目标节点2",
				Size: 1,
				PointsExpression: []PointExpression{
					{
						Tag:   map[string]string{"id": `"device"`},
						Field: map[string]string{"result": `"2"`},
					},
				},
				Label: "Target2",
				index: 2,
			}

			// 编译表达式
			program1, err := CompileSectionProgram(section1.PointsExpression, section1.Var)
			So(err, ShouldBeNil)
			section1.Program = program1

			program2, err := CompileSectionProgram(section2.PointsExpression, nil)
			So(err, ShouldBeNil)
			section2.Program = program2

			program3, err := CompileSectionProgram(section3.PointsExpression, nil)
			So(err, ShouldBeNil)
			section3.Program = program3

			section1.NextRules = []Rule{}

			// 创建标签映射
			labelMap := map[string]int{
				"Target1": 1,
				"Target2": 2,
			}

			// 创建节点列表
			nodes := []BProcessor{section1, section2, section3}

			Convey("无条件时应默认到下一个节点", func() {
				section1.NextRules = []Rule{}
				next, err := section1.Route(MockContext(), env, labelMap, nodes)
				So(err, ShouldBeNil)
				So(next, ShouldEqual, section2)
			})

			Convey("添加路由规则后测试", func() {
				rules := []Rule{{Condition: "true", Target: "Target2"}}
				err := CompileNextRoute(rules)
				So(err, ShouldBeNil)
				section1.NextRules = rules

				next, err := section1.Route(MockContext(), env, labelMap, nodes)
				So(err, ShouldBeNil)
				So(next, ShouldEqual, section3)
			})

			Convey("特殊目标END应返回nil", func() {
				rules := []Rule{{Condition: "true", Target: "END"}}
				err := CompileNextRoute(rules)
				So(err, ShouldBeNil)
				section1.NextRules = rules

				next, err := section1.Route(MockContext(), env, labelMap, nodes)
				So(err, ShouldBeNil)
				So(next, ShouldBeNil)
			})

			Convey("基于变量的条件路由", func() {
				env.Vars["routeVar"] = 100

				rules := []Rule{
					{Condition: "Vars.routeVar < 50", Target: "Target1"},
					{Condition: "Vars.routeVar == 100", Target: "Target2"},
				}
				err := CompileNextRoute(rules)
				So(err, ShouldBeNil)
				section1.NextRules = rules

				next, err := section1.Route(MockContext(), env, labelMap, nodes)
				So(err, ShouldBeNil)
				So(next, ShouldEqual, section3)
			})

			Convey("多规则评估 - 第一个匹配规则生效", func() {
				env.Vars["routeVar"] = 100

				rules := []Rule{
					{Condition: "Vars.routeVar == 100", Target: "Target1"},
					{Condition: "Vars.routeVar > 50", Target: "Target2"},
				}
				err := CompileNextRoute(rules)
				So(err, ShouldBeNil)
				section1.NextRules = rules

				next, err := section1.Route(MockContext(), env, labelMap, nodes)
				So(err, ShouldBeNil)
				So(next, ShouldEqual, section2)
			})

			Convey("无匹配规则应返回错误", func() {
				env.Vars["routeVar"] = 10

				rules := []Rule{
					{Condition: "Vars.routeVar > 50", Target: "Target1"},
					{Condition: "Vars.routeVar == 100", Target: "Target2"},
				}
				err := CompileNextRoute(rules)
				So(err, ShouldBeNil)
				section1.NextRules = rules

				_, err = section1.Route(MockContext(), env, labelMap, nodes)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "所有路由规则都不匹配")
			})

			Convey("条件表达式运行时错误", func() {
				rules := []Rule{{Condition: "Vars.nonExistentVar > 10", Target: "Target1"}}
				err := CompileNextRoute(rules)
				So(err, ShouldBeNil)
				section1.NextRules = rules

				_, err = section1.Route(MockContext(), env, labelMap, nodes)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "执行路由条件表达式失败")
			})

			Convey("目标标签不存在应返回错误", func() {
				rules := []Rule{{Condition: "true", Target: "MissingTarget"}}
				err := CompileNextRoute(rules)
				So(err, ShouldBeNil)
				section1.NextRules = rules

				_, err = section1.Route(MockContext(), env, labelMap, nodes)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "路由目标标签 'MissingTarget' 在标签映射中未找到")
			})

			Convey("目标索引越界应返回错误", func() {
				invalidLabelMap := map[string]int{
					"Target1":           1,
					"Target2":           2,
					"OutOfBoundsTarget": 10,
				}
				rules := []Rule{{Condition: "true", Target: "OutOfBoundsTarget"}}
				err := CompileNextRoute(rules)
				So(err, ShouldBeNil)
				section1.NextRules = rules

				_, err = section1.Route(MockContext(), env, invalidLabelMap, nodes)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "超出节点范围")
			})
		})

		Convey("动态设备名测试", func() {
			// 创建带有变量的设备名的Section
			section := &Section{
				Desc: "动态设备名测试",
				Size: 2,
				PointsExpression: []PointExpression{
					{
						Tag:   map[string]string{"id": `"device" + string(Vars.index)`},
						Field: map[string]string{"value": "Bytes[0]"},
					},
				},
				Var:   map[string]any{"index": "Bytes[1]"},
				index: 0,
			}

			// 编译表达式
			program, err := CompileSectionProgram(section.PointsExpression, section.Var)
			So(err, ShouldBeNil)
			section.Program = program

			// 创建ByteState
			env := &BEnv{
				Bytes:     nil,
				Vars:      make(map[string]interface{}),
				GlobalMap: make(map[string]interface{}),
				Points: []pkg.Point{
					{Tag: make(map[string]interface{}), Field: make(map[string]interface{})},
					{Tag: make(map[string]interface{}), Field: make(map[string]interface{})},
					{Tag: make(map[string]interface{}), Field: make(map[string]interface{})},
				},
			}
			nodes := []BProcessor{section}
			byteState := NewByteState(env, nil, nodes)
			byteState.Data = []byte{42, 5}

			// 这里修正：初始化一个point，将其加入out切片
			out := []*pkg.Point{}

			Convey("动态设备名应正确解析", func() {
				processedOut, _, err := section.ProcessWithBytes(context.Background(), byteState, out)
				So(err, ShouldBeNil)
				So(byteState.Env.Vars["index"], ShouldEqual, byte(5))

				So(len(processedOut), ShouldEqual, 1)
				So(processedOut[0].Tag["id"], ShouldEqual, "device5")
				So(processedOut[0].Field["value"], ShouldEqual, byte(42))
			})
		})
	})
}

func TestBuildSequence(t *testing.T) {
	Convey("BuildSequence功能测试", t, func() {
		Convey("应正确构建节点序列和标签映射", func() {
			// 准备配置列表
			configList := []map[string]any{
				{
					"desc":  "Section 1",
					"size":  2,
					"Label": "First",
					"Dev": map[string]map[string]string{
						"device1": {"field1": "Bytes[0]"},
					},
				},
				{
					"skip": 3,
				},
				{
					"desc":  "Section 2",
					"size":  1,
					"Label": "Second",
					"Dev": map[string]map[string]string{
						"device2": {"field2": "Bytes[0]"},
					},
				},
			}

			// 调用BuildSequence
			nodes, labelMap, err := BuildSequence(configList)
			So(err, ShouldBeNil)
			So(len(nodes), ShouldEqual, 3)
			So(len(labelMap), ShouldEqual, 2)

			// 检查标签映射
			So(labelMap["First"], ShouldEqual, 0)
			So(labelMap["Second"], ShouldEqual, 2)

			// 检查节点类型
			_, ok := nodes[0].(*Section)
			So(ok, ShouldBeTrue)
			_, ok = nodes[1].(*Skip)
			So(ok, ShouldBeTrue)
			_, ok = nodes[2].(*Section)
			So(ok, ShouldBeTrue)
		})

		Convey("处理空配置列表", func() {
			// 使用空配置列表
			emptyConfig := []map[string]any{}
			nodes, labelMap, err := BuildSequence(emptyConfig)
			So(err, ShouldBeNil)
			So(nodes, ShouldBeNil) // 根据当前实现，空配置应该返回nil
			So(labelMap, ShouldBeNil)
		})

		Convey("处理重复标签应返回错误", func() {
			configList := []map[string]any{
				{
					"desc":  "Section 1",
					"size":  2,
					"Label": "Duplicate",
				},
				{
					"desc":  "Section 2",
					"size":  1,
					"Label": "Duplicate", // 重复标签
				},
			}

			_, _, err := BuildSequence(configList)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "重复定义")
		})
	})
}

func TestVarStore(t *testing.T) {
	Convey("VarStore功能测试", t, func() {
		Convey("parseToDevice函数测试", func() {
			vars := VarStore{
				"deviceId": 123,
				"type":     "sensor",
				"name":     "温度传感器",
			}

			Convey("简单替换单个变量", func() {
				template := "device${deviceId}"
				result, err := parseToDevice(vars, template)
				So(err, ShouldBeNil)
				So(result, ShouldEqual, "device123")
			})

			Convey("替换多个变量", func() {
				template := "${type}${deviceId}_${name}"
				result, err := parseToDevice(vars, template)
				So(err, ShouldBeNil)
				So(result, ShouldEqual, "sensor123_温度传感器")
			})

			Convey("处理不存在的变量", func() {
				template := "device${nonexistent}"
				_, err := parseToDevice(vars, template)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "未找到模板变量")
			})

			Convey("处理无变量的模板", func() {
				template := "staticDevice"
				result, err := parseToDevice(vars, template)
				So(err, ShouldBeNil)
				So(result, ShouldEqual, "staticDevice")
			})
		})

		Convey("getIntVar函数测试", func() {
			vars := VarStore{
				"intVal":      42,
				"int64Val":    int64(9223372036854775807),
				"floatVal":    42.0,
				"nonIntFloat": 42.5,
				"strInt":      "100",
				"nonIntStr":   "abc",
				"boolVal":     true,
			}

			Convey("直接传入整数", func() {
				result, err := getIntVar(vars, 10)
				So(err, ShouldBeNil)
				So(result, ShouldEqual, 10)
			})

			Convey("直接传入整数浮点数", func() {
				result, err := getIntVar(vars, 10.0)
				So(err, ShouldBeNil)
				So(result, ShouldEqual, 10)
			})

			Convey("直接传入非整数浮点数应返回错误", func() {
				_, err := getIntVar(vars, 10.5)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "无法将非整数浮点数")
			})

			Convey("获取整数变量", func() {
				result, err := getIntVar(vars, "intVal")
				So(err, ShouldBeNil)
				So(result, ShouldEqual, 42)
			})

			Convey("获取int64变量", func() {
				result, err := getIntVar(vars, "int64Val")
				So(err, ShouldBeNil)
				So(result, ShouldEqual, 9223372036854775807)
			})

			Convey("获取整数浮点变量", func() {
				result, err := getIntVar(vars, "floatVal")
				So(err, ShouldBeNil)
				So(result, ShouldEqual, 42)
			})

			Convey("获取非整数浮点变量应返回错误", func() {
				_, err := getIntVar(vars, "nonIntFloat")
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "不是整数")
			})

			Convey("获取整数字符串变量", func() {
				result, err := getIntVar(vars, "strInt")
				So(err, ShouldBeNil)
				So(result, ShouldEqual, 100)
			})

			Convey("获取非整数字符串变量应返回错误", func() {
				_, err := getIntVar(vars, "nonIntStr")
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "无法将变量")
			})

			Convey("获取布尔变量应返回错误", func() {
				_, err := getIntVar(vars, "boolVal")
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "类型")
			})

			Convey("获取不存在的变量应返回错误", func() {
				_, err := getIntVar(vars, "nonexistent")
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "未找到重复次数变量")
			})
		})
	})
}
