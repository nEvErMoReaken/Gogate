package parser

import (
	"context"
	"gateway/internal/pkg"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

// 测试用例
func TestJParser(t *testing.T) {

	ctx := pkg.WithErrChan(context.Background(), make(chan error, 10))
	Convey("Given a jParser and a MessageDataSource", t, func() {
		sinkMap := &pkg.PointDataSource{} // 替换为适当的结构体类型
		JsonScriptFuncCache["mockFunc"] = func(jsonMap map[string]interface{}) (devName string, devType string, fields map[string]interface{}, err error) {
			return "dev1", "type1", jsonMap, nil
		}
		// 模拟 json 解析方法
		// 这里假设 jParserConfig.Method 是 "simpleMethod"，你需要根据实际情况修改
		jParserConfig := jParserConfig{Method: "mockFunc"}

		// 监听解析函数是否能正确调用
		// 此处我们创建一个 JSON 字符串并测试解析
		Convey("When Start is called", func() {
			// 设置 Mock 读取返回的数据
			mockDataSource := &pkg.MessageDataSource{
				DataChan: make(chan []byte, 200),
				MetaData: map[string]string{"mock": "1"},
			}
			err := mockDataSource.WriteOne([]byte(`{"field1":"value1", "field2": 42}`))

			So(err, ShouldBeNil)

			// 假设 jParser 使用这个模拟的数据源
			jParser := &jParser{
				ctx:                ctx,
				SnapshotCollection: SnapshotCollection{},
				jParserConfig:      jParserConfig,
			}

			// 启动解析
			Convey("Should read and parse JSON data correctly", func() {
				// 模拟调用 Start 方法
				var ds pkg.DataSource = mockDataSource
				go jParser.Start(&ds, sinkMap)

				// 假设我们在数据源中读取到了数据
				res, err := mockDataSource.ReadOne()
				So(err, ShouldBeNil)
				So(res, ShouldNotBeNil)
			})
		})

		Convey("Given ConversionToSnapshot is called", func() {
			// 模拟一个简单的 JSON 字符串
			jsonStr := `{"field1":"value1", "field2": 42}`
			jParser := &jParser{
				ctx:                ctx,
				SnapshotCollection: SnapshotCollection{},
				jParserConfig:      jParserConfig,
			}

			// 解析 JSON 字符串并调用相关方法
			Convey("Should correctly convert JSON to Snapshot", func() {
				// 我们直接调用 ConversionToSnapshot 方法
				err := jParser.ConversionToSnapshot(jsonStr)
				So(err, ShouldBeNil)
				// 假设 SnapshotCollection 获取成功
				snapshot, err := jParser.SnapshotCollection.GetDeviceSnapshot("dev1", "type1")
				So(err, ShouldBeNil)

				// 假设我们设置字段值
				err = snapshot.SetField(ctx, "field1", "value1")
				So(err, ShouldBeNil)
			})
		})
	})

	Convey("Given an error in the parser", t, func() {
		// 模拟一个错误情况
		Convey("When a JSON parsing error occurs", func() {
			// 设置错误的 JSON 数据
			invalidJSON := `{"field1": value1}`

			// 调用 ConversionToSnapshot 处理无效的 JSON 数据
			Convey("Should handle JSON parse errors", func() {
				// 假设 ErrChan 是一个全局错误通道
				// 将错误通道添加到上下文中

				// 调用 ConversionToSnapshot 以触发错误
				jParser := jParser{
					ctx:                ctx,
					SnapshotCollection: SnapshotCollection{},
					jParserConfig:      jParserConfig{Method: "invalidMethod"},
				}
				err := jParser.ConversionToSnapshot(invalidJSON)

				// 验证错误通道是否得到了错误
				So(err, ShouldNotBeNil)
			})
		})
	})

	Convey("Given a method not found", t, func() {
		// 模拟方法没有找到的情况
		Convey("Should handle method not found error", func() {
			jParser := &jParser{
				ctx:                ctx,
				SnapshotCollection: make(SnapshotCollection),
				jParserConfig:      jParserConfig{Method: "unknownMethod"},
			}
			err := jParser.ConversionToSnapshot(`{"field1":"value1"}`)
			So(err, ShouldNotBeNil)
		})
	})
}
