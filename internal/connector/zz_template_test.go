package connector

//
//import (
//	"context"
//	"errors"
//	"gateway/internal/pkg"
//	"testing"
//
//	"github.com/smartystreets/goconvey/convey"
//)
//
//// 模拟 Template 实现
//type mockTemplate struct {
//	typ string
//}
//
//func (m *mockTemplate) Start(dataSourceChan chan pkg.DataSource) error {
//	return nil
//}
//
//func (m *mockTemplate) GetType() string {
//	return m.typ
//}
//
//// 模拟 FactoryFunc
//func mockFactory(ctx context.Context) (Template, error) {
//	return &mockTemplate{typ: "mockType"}, nil
//}
//
//func failingFactory(ctx context.Context) (Template, error) {
//	return nil, errors.New("factory error")
//}
//func mockConfigFromString(connType string) *pkg.Config {
//	return &pkg.Config{
//		Connector: pkg.ConnectorConfig{
//			Type: connType,
//			Para: map[string]interface{}{},
//		},
//	}
//}
//func TestConnectorPackage(t *testing.T) {
//	convey.Convey("测试 connector 包的功能", t, func() {
//		convey.Convey("测试 Register 函数", func() {
//			Register("mockType", mockFactory)
//			convey.So(Factories["mockType"], convey.ShouldNotBeNil)
//		})
//
//		convey.Convey("测试 New 函数", func() {
//			ctx := pkg.WithConfig(commonCtx, mockConfigFromString("mockType"))
//			// 注册 mockType 的工厂函数
//			Register("mockType", mockFactory)
//			// 调用 New 方法
//			conn, err := New(ctx)
//			convey.So(err, convey.ShouldBeNil)
//			convey.So(conn, convey.ShouldNotBeNil)
//			convey.So(conn.GetType(), convey.ShouldEqual, "mockType")
//		})
//
//		convey.Convey("测试 New 函数未注册类型", func() {
//			// 调用 New 方法
//			conn, err := New(commonCtx)
//			convey.So(err, convey.ShouldNotBeNil)
//			convey.So(conn, convey.ShouldBeNil)
//			convey.So(err.Error(), convey.ShouldContainSubstring, "未找到数据源类型")
//		})
//
//		convey.Convey("测试 New 函数失败的工厂函数", func() {
//
//			// 注册失败的工厂函数
//			Register("failingType", failingFactory)
//			// 调用 New 方法
//			conn, err := New(pkg.WithConfig(commonCtx, mockConfigFromString("failingType")))
//			convey.So(err, convey.ShouldNotBeNil)
//			convey.So(conn, convey.ShouldBeNil)
//			convey.So(err.Error(), convey.ShouldContainSubstring, "初始化数据源失败")
//		})
//	})
//}
