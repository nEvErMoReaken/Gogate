package parser

import (
	"context"
	"gateway/internal/pkg"
	"testing"

	"github.com/expr-lang/expr"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSection(t *testing.T) {
	Convey("测试Section", t, func() {
		Convey("Section基本功能测试", func() {
			// 创建一个简单的Section
			section := &Section{
				Desc: "Section 1",
				Size: 2,
				PointsExpression: []PointExpression{
					{
						Tag: map[string]string{
							"id": `"device1"`,
						},
						Field: map[string]string{
							"value": "Bytes[0]",
							"count": "Bytes[1]",
						},
					},
				},
				Var: map[string]any{
					"total": "Bytes[0] + Bytes[1]",
				},
				Label: "TestLabel",
				index: 0,
			}

			// 编译表达式
			program, err := expr.Compile(`V("total", Bytes[0] + Bytes[1]); S({"id": "device1"}, {"value": Bytes[0], "count": Bytes[1]}); nil;`, BuildSectionExprOptions()...)
			So(err, ShouldBeNil)
			section.Program = program

			// 创建ByteState
			env := &BEnv{
				Bytes:       nil,
				Vars:        make(map[string]interface{}),
				GlobalMap:   make(map[string]interface{}),
				Points:      make([]*pkg.Point, 0),
				PointsIndex: make(map[uint64]int),
			}
			labelMap := map[string]int{"TestLabel": 0}
			nodes := []BProcessor{section}
			state := &ByteState{
				Data:     []byte{10, 20},
				Cursor:   0,
				Env:      env,
				LabelMap: labelMap,
				Nodes:    nodes,
			}

			Convey("ProcessWithBytes应正确处理数据", func() {
				next, err := section.ProcessWithBytes(context.Background(), state)
				So(err, ShouldBeNil)
				So(next, ShouldBeNil) // 应该是最后一个节点

				// 检查游标位置
				So(state.Cursor, ShouldEqual, 2)

				// 检查变量设置
				So(state.Env.Vars["total"], ShouldEqual, 30)

				// 检查输出点
				So(len(state.Env.Points), ShouldEqual, 1)
				So(state.Env.Points[0].Tag["id"], ShouldEqual, "device1")
				So(state.Env.Points[0].Field["value"], ShouldEqual, byte(10))
				So(state.Env.Points[0].Field["count"], ShouldEqual, byte(20))
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
						Tag: map[string]string{
							"id": `"device2"`,
						},
						Field: map[string]string{
							"value": "Bytes[0]",
							"count": "Bytes[1]",
						},
					},
				},
				index: 1,
			}

			// 编译表达式
			program, err := expr.Compile(`S({"id": "device2"}, {"value": Bytes[0], "count": Bytes[1]}); nil;`, BuildSectionExprOptions()...)
			So(err, ShouldBeNil)
			section.Program = program

			// 创建ByteState
			env := &BEnv{
				Bytes:       nil,
				Vars:        make(map[string]interface{}),
				GlobalMap:   make(map[string]interface{}),
				Points:      make([]*pkg.Point, 0),
				PointsIndex: make(map[uint64]int),
			}
			nodes := []BProcessor{skip, section}
			state := &ByteState{
				Data:   []byte{1, 2, 3, 4, 5},
				Cursor: 0,
				Env:    env,
				Nodes:  nodes,
			}

			Convey("Skip.ProcessWithBytes应正确跳过字节", func() {
				next, err := skip.ProcessWithBytes(context.Background(), state)
				So(err, ShouldBeNil)
				So(next, ShouldEqual, section)
				So(state.Cursor, ShouldEqual, 3)
			})

			Convey("连续处理Skip和Section", func() {
				// Reset ByteState cursor for this specific test case
				state.Cursor = 0

				// 先执行Skip
				next, err := skip.ProcessWithBytes(context.Background(), state)
				So(err, ShouldBeNil)
				So(next, ShouldEqual, section)
				So(state.Cursor, ShouldEqual, 3)

				// 再执行Section
				next, err = next.ProcessWithBytes(context.Background(), state)
				So(err, ShouldBeNil)
				So(next, ShouldBeNil)
				So(state.Cursor, ShouldEqual, 5)

				// 检查输出点
				So(len(state.Env.Points), ShouldEqual, 1) // Expecting one point from the section
				So(state.Env.Points[0].Tag["id"], ShouldEqual, "device2")
				So(state.Env.Points[0].Field["value"], ShouldEqual, byte(4))
				So(state.Env.Points[0].Field["count"], ShouldEqual, byte(5))
			})
		})

		Convey("Section路由功能测试", func() {
			// 准备测试环境
			env := &BEnv{
				Bytes:       []byte{10, 20},
				Vars:        make(map[string]interface{}),
				GlobalMap:   make(map[string]interface{}),
				Points:      make([]*pkg.Point, 0),
				PointsIndex: make(map[uint64]int),
			}

			// 创建节点
			section1 := &Section{
				Desc: "条件路由节点",
				Size: 2,
				PointsExpression: []PointExpression{
					{
						Tag: map[string]string{
							"id": `"device"`,
						},
						Field: map[string]string{
							"value": "Bytes[0]",
						},
					},
				},
				Var: map[string]any{
					"condition_var": "Bytes[0] > 5",
				},
				index: 0,
			}

			section2 := &Section{
				Desc: "目标节点1",
				Size: 1,
				PointsExpression: []PointExpression{
					{
						Tag: map[string]string{
							"id": `"device"`,
						},
						Field: map[string]string{
							"result": `"1"`,
						},
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
						Tag: map[string]string{
							"id": `"device"`,
						},
						Field: map[string]string{
							"result": `"2"`,
						},
					},
				},
				Label: "Target2",
				index: 2,
			}

			// 编译表达式
			program1, err := expr.Compile(`V("condition_var", Bytes[0] > 5); S({"id": "device"}, {"value": Bytes[0]}); nil;`, BuildSectionExprOptions()...)
			So(err, ShouldBeNil)
			section1.Program = program1

			program2, err := expr.Compile(`S({"id": "device"}, {"result": "1"}); nil;`, BuildSectionExprOptions()...)
			So(err, ShouldBeNil)
			section2.Program = program2

			program3, err := expr.Compile(`S({"id": "device"}, {"result": "2"}); nil;`, BuildSectionExprOptions()...)
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
				next, err := section1.Route(context.Background(), env, labelMap, nodes)
				So(err, ShouldBeNil)
				So(next, ShouldEqual, section2)
			})

			Convey("添加路由规则后测试", func() {
				rules := []Rule{{Condition: "true", Target: "Target2"}}
				err := CompileNextRoute(rules)
				So(err, ShouldBeNil)
				section1.NextRules = rules

				next, err := section1.Route(context.Background(), env, labelMap, nodes)
				So(err, ShouldBeNil)
				So(next, ShouldEqual, section3)
			})

			Convey("特殊目标END应返回nil", func() {
				rules := []Rule{{Condition: "true", Target: "END"}}
				err := CompileNextRoute(rules)
				So(err, ShouldBeNil)
				section1.NextRules = rules

				next, err := section1.Route(context.Background(), env, labelMap, nodes)
				So(err, ShouldBeNil)
				So(next, ShouldBeNil)
			})

			Convey("特殊目标DEFAULT应使用默认行为", func() {
				rules := []Rule{{Condition: "false", Target: "Target2"}, {Condition: "true", Target: "DEFAULT"}}
				err := CompileNextRoute(rules)
				So(err, ShouldBeNil)
				section1.NextRules = rules

				next, err := section1.Route(context.Background(), env, labelMap, nodes)
				So(err, ShouldBeNil)
				So(next, ShouldEqual, section2) // 默认为下一个节点，即index+1
			})

			Convey("条件判断", func() {
				env.Vars["condition_var"] = true
				rules := []Rule{
					{Condition: "Vars.condition_var", Target: "Target2"},
					{Condition: "true", Target: "Target1"},
				}
				err := CompileNextRoute(rules)
				So(err, ShouldBeNil)
				section1.NextRules = rules

				next, err := section1.Route(context.Background(), env, labelMap, nodes)
				So(err, ShouldBeNil)
				So(next, ShouldEqual, section3) // 第一个条件满足，应该路由到Target2
			})

			Convey("条件判断（第一个条件不满足）", func() {
				env.Vars["condition_var"] = false
				rules := []Rule{
					{Condition: "Vars.condition_var", Target: "Target2"},
					{Condition: "true", Target: "Target1"},
				}
				err := CompileNextRoute(rules)
				So(err, ShouldBeNil)
				section1.NextRules = rules

				next, err := section1.Route(context.Background(), env, labelMap, nodes)
				So(err, ShouldBeNil)
				So(next, ShouldEqual, section2) // 第一个条件不满足，第二个条件满足，应该路由到Target1
			})

			Convey("未找到标签时应报错", func() {
				rules := []Rule{{Condition: "true", Target: "NonExistentTarget"}}
				err := CompileNextRoute(rules)
				So(err, ShouldBeNil)
				section1.NextRules = rules

				result, routeErr := section1.Route(context.Background(), env, labelMap, nodes)
				So(routeErr, ShouldNotBeNil)
				So(result, ShouldBeNil)
				So(routeErr.Error(), ShouldContainSubstring, "路由目标标签")
			})

			Convey("条件表达式求值错误时应报错", func() {
				// 使用一个不存在的变量名作为条件表达式
				// 这种情况下编译就会失败
				compileErr := CompileNextRoute([]Rule{{Condition: "nonExistentVar", Target: "Target1"}})
				So(compileErr, ShouldNotBeNil)
				So(compileErr.Error(), ShouldContainSubstring, "unknown name nonExistentVar")
			})
		})

		Convey("动态设备名测试", func() {
			// 准备测试环境
			env := &BEnv{
				Bytes:       []byte{10, 20},
				Vars:        make(map[string]interface{}),
				GlobalMap:   make(map[string]interface{}),
				Points:      make([]*pkg.Point, 0),
				PointsIndex: make(map[uint64]int),
			}
			env.Vars["index"] = 123

			section := &Section{
				Desc: "动态设备名测试",
				Size: 2,
				PointsExpression: []PointExpression{
					{
						Tag: map[string]string{
							"id": `"device" + string(Vars.index)`,
						},
						Field: map[string]string{
							"value": "Bytes[0]",
						},
					},
				},
				index: 0,
			}

			// 编译表达式
			program, err := expr.Compile(`S({"id": "device" + string(Vars.index)}, {"value": Bytes[0]}); nil;`, BuildSectionExprOptions()...)
			So(err, ShouldBeNil)
			section.Program = program

			state := &ByteState{
				Data:   []byte{10, 20},
				Cursor: 0,
				Env:    env,
				Nodes:  []BProcessor{section},
			}

			Convey("应正确处理动态设备名", func() {
				next, err := section.ProcessWithBytes(context.Background(), state)
				So(err, ShouldBeNil)
				So(next, ShouldBeNil)

				// 检查输出点
				So(len(state.Env.Points), ShouldEqual, 1)
				So(state.Env.Points[0].Tag["id"], ShouldEqual, "device123")
				So(state.Env.Points[0].Field["value"], ShouldEqual, byte(10))
			})
		})
	})
}

func TestBuildSequence(t *testing.T) {
	Convey("测试BuildSequence函数", t, func() {
		Convey("构建单节点序列", func() {
			config := []map[string]any{
				{
					"desc": "Section 1",
					"size": 2,
					"Points": []map[string]any{
						{
							"Tag": map[string]any{
								"id": `"device1"`,
							},
							"Field": map[string]any{
								"field1": "Bytes[0]",
							},
						},
					},
					"Label": "Label1",
				},
			}

			nodes, labelMap, err := BuildSequence(config)
			So(err, ShouldBeNil)
			So(len(nodes), ShouldEqual, 1)
			So(labelMap["Label1"], ShouldEqual, 0)

			// 检查节点类型
			section, ok := nodes[0].(*Section)
			So(ok, ShouldBeTrue)
			So(section.Desc, ShouldEqual, "Section 1")
			So(section.Size, ShouldEqual, 2)
			So(section.Label, ShouldEqual, "Label1")
			So(section.index, ShouldEqual, 0)
		})

		Convey("构建混合节点序列", func() {
			config := []map[string]any{
				{
					"desc": "Section 1",
					"size": 2,
					"Points": []map[string]any{
						{
							"Tag": map[string]any{
								"id": `"device1"`,
							},
							"Field": map[string]any{
								"field1": "Bytes[0]",
							},
						},
					},
					"Label": "Label1",
				},
				{
					"skip": 3,
				},
				{
					"desc": "Section 2",
					"size": 1,
					"Points": []map[string]any{
						{
							"Tag": map[string]any{
								"id": `"device2"`,
							},
							"Field": map[string]any{
								"field2": "Bytes[0]",
							},
						},
					},
					"Label": "Label2",
				},
			}

			nodes, labelMap, err := BuildSequence(config)
			So(err, ShouldBeNil)
			So(len(nodes), ShouldEqual, 3)
			So(labelMap["Label1"], ShouldEqual, 0)
			So(labelMap["Label2"], ShouldEqual, 2)

			// 检查节点类型
			section1, ok := nodes[0].(*Section)
			So(ok, ShouldBeTrue)
			So(section1.Desc, ShouldEqual, "Section 1")
			So(section1.Size, ShouldEqual, 2)
			So(section1.Label, ShouldEqual, "Label1")
			So(section1.index, ShouldEqual, 0)

			skip, ok := nodes[1].(*Skip)
			So(ok, ShouldBeTrue)
			So(skip.Skip, ShouldEqual, 3)
			So(skip.index, ShouldEqual, 1)

			section2, ok := nodes[2].(*Section)
			So(ok, ShouldBeTrue)
			So(section2.Desc, ShouldEqual, "Section 2")
			So(section2.Size, ShouldEqual, 1)
			So(section2.Label, ShouldEqual, "Label2")
			So(section2.index, ShouldEqual, 2)
		})

		Convey("处理标签重复", func() {
			config := []map[string]any{
				{
					"desc":  "Section 1",
					"size":  2,
					"Label": "DuplicateLabel",
				},
				{
					"desc":  "Section 2",
					"size":  1,
					"Label": "DuplicateLabel",
				},
			}

			_, _, err := BuildSequence(config)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "标签 'DuplicateLabel' 在 Section 1 (Desc: Section 2) 处重复定义")
		})

		Convey("处理空配置", func() {
			config := []map[string]any{}
			nodes, labelMap, err := BuildSequence(config)
			So(err, ShouldBeNil)
			So(nodes, ShouldBeNil)
			So(labelMap, ShouldBeNil)
		})
	})
}

func TestVarStore(t *testing.T) {
	// 添加必要的VarStore测试
}
