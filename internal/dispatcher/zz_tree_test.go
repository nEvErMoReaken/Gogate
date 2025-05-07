package dispatcher

import (
	"gateway/internal/pkg"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTree(t *testing.T) {
	Convey("设备树测试套件", t, func() {
		Convey("创建新的设备树", func() {
			// 创建一个测试用的分发器配置
			dispatcherConfig := &pkg.DispatcherConfig{
				RepeatDataFilters: []pkg.DataFilter{
					{
						DevFilter:  "dev1.*",
						TeleFilter: "temp.*",
					},
					{
						DevFilter:  "dev2.*",
						TeleFilter: "status.*",
					},
				},
			}

			tree := NewTree(dispatcherConfig)

			So(tree, ShouldNotBeNil)
			So(tree.Root, ShouldNotBeNil)
			So(tree.filterList, ShouldHaveLength, 2)
			So(tree.pathCache, ShouldNotBeNil)
		})

		Convey("获取路径部分", func() {
			// 创建一个测试用的分发器配置
			dispatcherConfig := &pkg.DispatcherConfig{
				RepeatDataFilters: []pkg.DataFilter{},
			}

			tree := NewTree(dispatcherConfig)

			Convey("解析简单路径", func() {
				parts := tree.getPathParts("device.sensor")
				So(parts, ShouldResemble, []string{"device", "sensor"})
			})

			Convey("解析多级路径", func() {
				parts := tree.getPathParts("area.device.sensor.metric")
				So(parts, ShouldResemble, []string{"area", "device", "sensor", "metric"})
			})

			Convey("使用缓存的路径", func() {
				// 第一次调用会缓存结果
				tree.getPathParts("device.sensor")

				// 检查缓存
				So(tree.pathCache, ShouldContainKey, "device.sensor")

				// 再次调用应该使用缓存
				parts := tree.getPathParts("device.sensor")
				So(parts, ShouldResemble, []string{"device", "sensor"})
			})
		})

		Convey("获取或创建节点", func() {
			// 创建一个测试用的分发器配置
			dispatcherConfig := &pkg.DispatcherConfig{
				RepeatDataFilters: []pkg.DataFilter{
					{
						DevFilter:  "dev1.*",
						TeleFilter: "temp.*",
					},
					{
						DevFilter:  "dev2.*",
						TeleFilter: "status.*",
					},
				},
			}

			tree := NewTree(dispatcherConfig)

			Convey("创建新节点", func() {
				node, err := tree.GetNode("area.dev1.sensor")

				So(err, ShouldBeNil)
				So(node, ShouldNotBeNil)
				So(node.devName, ShouldEqual, "area.dev1.sensor")

				// 验证节点层次结构
				parentNode := node.Parent
				So(parentNode.devName, ShouldEqual, "area.dev1")

				grandParentNode := parentNode.Parent
				So(grandParentNode.devName, ShouldEqual, "area")

				// 根节点的父节点应为nil
				So(grandParentNode.Parent, ShouldEqual, tree.Root)
			})

			Convey("获取现有节点", func() {
				// 首次创建节点
				node1, _ := tree.GetNode("area.dev1.sensor")

				// 再次获取同一节点
				node2, err := tree.GetNode("area.dev1.sensor")

				So(err, ShouldBeNil)
				So(node2, ShouldEqual, node1) // 应该是同一个节点实例
			})

			Convey("正确应用过滤规则", func() {
				// 获取匹配过滤规则的节点
				node, err := tree.GetNode("area.dev1.sensor")

				So(err, ShouldBeNil)
				So(node.Parent.repeatFilterRule, ShouldContain, "temp.*")

				// 获取另一个匹配不同过滤规则的节点
				node2, err := tree.GetNode("zone.dev2.unit")

				So(err, ShouldBeNil)
				So(node2.Parent.repeatFilterRule, ShouldContain, "status.*")
			})

			Convey("处理无效的设备名", func() {
				// 空设备名应该返回根节点，并且不报错
				node, err := tree.GetNode("")
				So(err, ShouldBeNil)
				So(node, ShouldEqual, tree.Root)

				// 包含连续点号的设备名
				node, err = tree.GetNode("area..dev1...sensor")
				So(err, ShouldBeNil)
				So(node.devName, ShouldEqual, "area.dev1.sensor") // 假设连续点号被视为单个点号处理
			})

		})

		Convey("添加点数据", func() {
			// 创建一个测试用的分发器配置
			dispatcherConfig := &pkg.DispatcherConfig{
				RepeatDataFilters: []pkg.DataFilter{
					{
						DevFilter:  "dev1.*",
						TeleFilter: "temp.*",
					},
					{
						DevFilter:  "dev2.*",
						TeleFilter: "status.*",
					},
				},
			}

			tree := NewTree(dispatcherConfig)

			// 设置策略过滤器
			tree.StrategyFilterList = map[string][]pkg.DataFilter{
				"strategy1": {
					{
						DevFilter:  "dev1",
						TeleFilter: ".*",
					},
				},
				"strategy2": {
					{
						DevFilter:  "dev2",
						TeleFilter: "pressure",
					},
				},
			}

			Convey("添加单个点", func() {
				err := tree.AddPoint("area.dev1.sensor", map[string]interface{}{
					"temp":     25.5,
					"humidity": 60,
				})

				So(err, ShouldBeNil)

				// 获取节点验证数据是否正确添加
				node, _ := tree.GetNode("area.dev1.sensor")
				So(node.HotValue["temp"], ShouldEqual, 25.5)
				So(node.HotValue["humidity"], ShouldEqual, 60)

			})

			Convey("批量添加点数据", func() {
				pointPackage := &pkg.PointPackage{
					FrameId: "frame1",
					Ts:      time.Now(),
					Points: []*pkg.Point{
						{
							Device: "area.dev1.sensor",
							Field: map[string]interface{}{
								"temp": 26.5,
							},
						},
						{
							Device: "zone.dev2.unit",
							Field: map[string]interface{}{
								"pressure": 101.3,
							},
						},
					},
				}

				err := tree.BatchAddPoint(pointPackage)

				So(err, ShouldBeNil)
				So(tree.LatestFrameId, ShouldEqual, "frame1")

				// 验证数据是否正确添加到各个节点
				node1, _ := tree.GetNode("area.dev1.sensor")
				So(node1.HotValue["temp"], ShouldEqual, 26.5)

				node2, _ := tree.GetNode("zone.dev2.unit")
				So(node2.HotValue["pressure"], ShouldEqual, 101.3)
			})

			Convey("添加空值", func() {
				err := tree.AddPoint("area.dev3.sensor", map[string]interface{}{})
				So(err, ShouldBeNil)
				node, _ := tree.GetNode("area.dev3.sensor")
				So(len(node.HotValue), ShouldEqual, 0) // HotValue 应为空
				// 确认空值添加不会将节点错误地标记为 dirty
				So(len(tree.dirtyNodes), ShouldEqual, 0)
			})

			Convey("无重复过滤器时的添加", func() {
				// 使用没有重复过滤器的树
				simpleTree := NewTree(&pkg.DispatcherConfig{})
				err := simpleTree.AddPoint("dev.sensor", map[string]interface{}{"value": 10})
				So(err, ShouldBeNil)
				node, _ := simpleTree.GetNode("dev.sensor")
				So(node.HotValue["value"], ShouldEqual, 10)
				// 再次添加相同值，不应被过滤
				err = simpleTree.AddPoint("dev.sensor", map[string]interface{}{"value": 10})
				So(err, ShouldBeNil)
				So(node.HotValue["value"], ShouldEqual, 10) // HotValue 仍然是 10
			})

			Convey("测试重复过滤与更新", func() {
				// 添加初始值
				err := tree.AddPoint("area.dev1.sensor", map[string]interface{}{"temp": 27.0, "voltage": 5.0})
				So(err, ShouldBeNil)
				node, _ := tree.GetNode("area.dev1.sensor")
				// temp 匹配 repeatFilterRule "temp.*"，所以 LastValue 和 HotValue 都应更新
				So(node.LastValue["temp"], ShouldEqual, 27.0)
				So(node.HotValue["temp"], ShouldEqual, 27.0)
				// voltage 不匹配 repeatFilterRule，所以只更新 HotValue，LastValue 不应包含 voltage
				So(node.HotValue["voltage"], ShouldEqual, 5.0)
				So(node.LastValue, ShouldNotContainKey, "voltage")
				So(tree.dirtyNodes, ShouldContainKey, node)

				_, _ = tree.Freeze() // 清空 HotValue 和 dirtyNodes
				So(len(node.HotValue), ShouldEqual, 0)
				So(len(tree.dirtyNodes), ShouldEqual, 0)

				// 再次添加，temp 值相同（应被过滤），voltage 值不同（不被过滤规则覆盖，应更新 HotValue）
				err = tree.AddPoint("area.dev1.sensor", map[string]interface{}{"temp": 27.0, "voltage": 5.1})
				So(err, ShouldBeNil)

				// temp 不应出现在 HotValue 中，因为值未变且匹配过滤规则。LastValue 保持不变。
				So(node.HotValue, ShouldNotContainKey, "temp")
				So(node.LastValue["temp"], ShouldEqual, 27.0) // LastValue 保持不变
				// voltage 应出现在 HotValue 中。LastValue 仍然不包含 voltage。
				So(node.HotValue["voltage"], ShouldEqual, 5.1)
				So(node.LastValue, ShouldNotContainKey, "voltage")
				// 节点应该因为 voltage 更新而被标记为 dirty
				So(tree.dirtyNodes, ShouldContainKey, node)

				_, _ = tree.Freeze() // 清空

				// 添加新值，temp 值不同（应更新 LastValue 和 HotValue），voltage 值也不同（应更新 HotValue）
				err = tree.AddPoint("area.dev1.sensor", map[string]interface{}{"temp": 28.0, "voltage": 5.2})
				So(err, ShouldBeNil)
				// 两者都应更新 HotValue
				So(node.HotValue["temp"], ShouldEqual, 28.0)
				So(node.HotValue["voltage"], ShouldEqual, 5.2)
				// 只有 temp 更新 LastValue
				So(node.LastValue["temp"], ShouldEqual, 28.0) // LastValue 也更新
				So(node.LastValue, ShouldNotContainKey, "voltage")
				So(tree.dirtyNodes, ShouldContainKey, node)
			})

			Convey("测试过滤逻辑", func() {
				// 添加两次相同数据，观察过滤效果
				err := tree.AddPoint("area.dev1.sensor", map[string]interface{}{
					"temp": 27.0,
				})
				So(err, ShouldBeNil)

				node, _ := tree.GetNode("area.dev1.sensor")
				So(node.LastValue["temp"], ShouldEqual, 27.0)

				_, _ = tree.Freeze()
				// 再次添加相同值
				err = tree.AddPoint("area.dev1.sensor", map[string]interface{}{
					"temp": 27.0,
				})
				So(err, ShouldBeNil)

				// 由于值相同且匹配过滤规则，节点不应标记为已更改
				So(len(node.HotValue), ShouldEqual, 0)
			})
		})

		Convey("冻结设备树并收集数据", func() {
			// 创建一个测试用的分发器配置
			dispatcherConfig := &pkg.DispatcherConfig{
				RepeatDataFilters: []pkg.DataFilter{},
			}

			tree := NewTree(dispatcherConfig)

			// 设置策略过滤器
			tree.StrategyFilterList = map[string][]pkg.DataFilter{
				"strategy1": {
					{
						DevFilter:  "area.*",
						TeleFilter: "temp",
					},
				},
				"strategy2": {
					{
						DevFilter:  "zone.*",
						TeleFilter: "pressure",
					},
				},
			}

			// 设置时间戳和帧ID
			tree.LatestTs = time.Now()
			tree.LatestFrameId = "testFrame"

			// 添加点数据
			err := tree.AddPoint("area.dev1.sensor", map[string]interface{}{
				"temp":     28.5,
				"humidity": 55,
			})
			So(err, ShouldBeNil)
			err = tree.AddPoint("zone.dev2.unit", map[string]interface{}{
				"pressure": 102.5,
				"flow":     15.7,
			})
			So(err, ShouldBeNil)
			Convey("收集更改的点数据", func() {
				results, err := tree.Freeze()
				So(err, ShouldBeNil)
				So(results, ShouldNotBeNil)

				// 验证策略1的数据 (应只包含temp字段)
				So(results, ShouldContainKey, "strategy1")
				strategy1Data := results["strategy1"]
				So(strategy1Data.FrameId, ShouldEqual, "testFrame")
				So(strategy1Data.Points, ShouldHaveLength, 1)
				So(strategy1Data.Points[0].Device, ShouldEqual, "area.dev1.sensor")
				So(strategy1Data.Points[0].Field, ShouldContainKey, "temp")
				So(strategy1Data.Points[0].Field, ShouldNotContainKey, "humidity")

				// 验证策略2的数据 (应只包含pressure字段)
				So(results, ShouldContainKey, "strategy2")
				strategy2Data := results["strategy2"]
				So(strategy2Data.Points, ShouldHaveLength, 1)
				So(strategy2Data.Points[0].Device, ShouldEqual, "zone.dev2.unit")
				So(strategy2Data.Points[0].Field, ShouldContainKey, "pressure")
				So(strategy2Data.Points[0].Field, ShouldNotContainKey, "flow")

				// 在 Freeze 后检查节点状态是否已重置
				node1, _ := tree.GetNode("area.dev1.sensor")
				node2, _ := tree.GetNode("zone.dev2.unit")
				So(len(node1.HotValue), ShouldEqual, 0)
				So(len(node2.HotValue), ShouldEqual, 0)

			})

			Convey("Freeze 空树或无变化的树", func() {
				emptyTree := NewTree(&pkg.DispatcherConfig{})
				results, err := emptyTree.Freeze()
				So(err, ShouldBeNil)
				So(len(results), ShouldEqual, 0) // 结果应为空

				// 添加数据后再 Freeze 一次
				_ = tree.AddPoint("some.device", map[string]interface{}{"value": 1})
				_, _ = tree.Freeze()
				// 再次 Freeze，此时无变化
				results, err = tree.Freeze()
				So(err, ShouldBeNil)
				So(len(results), ShouldEqual, 0) // 结果应为空
			})

			Convey("数据匹配多个策略", func() {
				// **为这个测试用例创建一个独立的 Tree 实例**
				multiStrategyConfig := &pkg.DispatcherConfig{
					RepeatDataFilters: []pkg.DataFilter{}, // 根据需要配置
				}
				multiStrategyTree := NewTree(multiStrategyConfig)
				multiStrategyTree.StrategyFilterList = map[string][]pkg.DataFilter{
					// 复制或定义必要的策略
					"strategy1": {
						{DevFilter: "area.*", TeleFilter: "temp"},
					},
					"strategy3": {
						{DevFilter: "area.dev1.*", TeleFilter: ".*"}, // 匹配 area.dev1 的所有遥测
					},
					"strategy_other": { // 添加一个不相关的策略以确保隔离性
						{DevFilter: "zone.*", TeleFilter: "pressure"},
					},
				}
				multiStrategyTree.LatestTs = time.Now()
				multiStrategyTree.LatestFrameId = "multiFrame"

				// 添加数据到新的 tree
				_ = multiStrategyTree.AddPoint("area.dev1.sensor", map[string]interface{}{"temp": 30.0, "pressure": 100.0})
				_ = multiStrategyTree.AddPoint("zone.unit", map[string]interface{}{"pressure": 101.0}) // 用于 strategy_other

				results, err := multiStrategyTree.Freeze()
				So(err, ShouldBeNil)
				So(results, ShouldNotBeNil)

				// 验证策略1 (只应有 temp)
				So(results, ShouldContainKey, "strategy1")
				So(results["strategy1"].Points, ShouldHaveLength, 1)
				So(results["strategy1"].Points[0].Device, ShouldEqual, "area.dev1.sensor")
				So(results["strategy1"].Points[0].Field, ShouldHaveLength, 1)
				So(results["strategy1"].Points[0].Field, ShouldContainKey, "temp")
				So(results["strategy1"].Points[0].Field["temp"], ShouldEqual, 30.0)

				// 验证策略3 (应有 temp 和 pressure)
				So(results, ShouldContainKey, "strategy3")
				So(results["strategy3"].Points, ShouldHaveLength, 1)
				So(results["strategy3"].Points[0].Device, ShouldEqual, "area.dev1.sensor")
				So(results["strategy3"].Points[0].Field, ShouldHaveLength, 2)
				So(results["strategy3"].Points[0].Field, ShouldContainKey, "temp")
				So(results["strategy3"].Points[0].Field, ShouldContainKey, "pressure")
				So(results["strategy3"].Points[0].Field["temp"], ShouldEqual, 30.0)
				So(results["strategy3"].Points[0].Field["pressure"], ShouldEqual, 100.0)

				// 验证其他策略
				So(results, ShouldContainKey, "strategy_other")
				So(results["strategy_other"].Points, ShouldHaveLength, 1)
				So(results["strategy_other"].Points[0].Device, ShouldEqual, "zone.unit")
				So(results["strategy_other"].Points[0].Field, ShouldContainKey, "pressure")

				// 确认 Freeze 清空了 dirtyNodes
				So(len(multiStrategyTree.dirtyNodes), ShouldEqual, 0)
			})

			Convey("数据不匹配任何策略", func() {
				// 添加的数据不匹配 strategy1 和 strategy2 的 DevFilter
				_ = tree.AddPoint("other.device", map[string]interface{}{"value": 123})
				node, _ := tree.GetNode("other.device")
				So(tree.dirtyNodes, ShouldContainKey, node) // 节点应标记为 dirty

				results, err := tree.Freeze()
				So(err, ShouldBeNil)
				// 结果中不应包含 other.device 的数据
				for _, pkg := range results {
					for _, p := range pkg.Points {
						So(p.Device, ShouldNotEqual, "other.device")
					}
				}
				So(len(node.HotValue), ShouldEqual, 0)   // Freeze 后 HotValue 应清空
				So(len(tree.dirtyNodes), ShouldEqual, 0) // Freeze 后 dirtyNodes 应清空
			})
		})

		Convey("更新时间戳", func() {
			dispatcherConfig := &pkg.DispatcherConfig{}
			tree := NewTree(dispatcherConfig)
			initialTime := tree.LatestTs
			newTime := time.Now().Add(time.Minute)
			tree.UpdateTime(newTime)
			So(tree.LatestTs, ShouldResemble, newTime)
			So(tree.LatestTs, ShouldNotResemble, initialTime)
		})

		Convey("过滤规则检查函数", func() {
			// 测试checkFilter函数
			cache := make(map[string]bool)

			Convey("匹配过滤规则", func() {
				result, err := checkFilter("temperature", []string{"temp.*"}, cache)
				So(err, ShouldBeNil)
				So(result, ShouldBeTrue)
				So(cache["temperature"], ShouldBeTrue)
			})

			Convey("不匹配过滤规则", func() {
				result, err := checkFilter("pressure", []string{"temp.*"}, cache)
				So(err, ShouldBeNil)
				So(result, ShouldBeFalse)
				So(cache["pressure"], ShouldBeFalse)
			})

			Convey("使用缓存结果", func() {
				// 预先在缓存中设置结果
				cache["humidity"] = true

				// 调用函数应该直接使用缓存结果
				result, err := checkFilter("humidity", []string{"not-matching"}, cache)
				So(err, ShouldBeNil)
				So(result, ShouldBeTrue) // 返回缓存的结果
			})

			Convey("无效的正则表达式", func() {
				result, err := checkFilter("value", []string{"["}, cache) // 无效的正则表达式
				So(err, ShouldNotBeNil)
				So(result, ShouldBeFalse)
			})
		})
	})
}
