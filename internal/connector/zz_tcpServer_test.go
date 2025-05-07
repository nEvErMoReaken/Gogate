package connector

//
//import (
//	"gateway/internal/pkg"
//	"net"
//	"testing"
//	"time"
//
//	"github.com/smartystreets/goconvey/convey"
//)
//
//// 模拟 pkg.ConfigFromContext 和 pkg.LoggerFromContext 以避免依赖真实的外部系统
//func mockConfigFromContext() *pkg.Config {
//	return &pkg.Config{
//		Connector: pkg.ConnectorConfig{
//			Para: map[string]interface{}{
//				"url":       ":8081",
//				"timeout":   "5s",
//				"whiteList": true,
//				"ipAlias": map[string]string{
//					"127.0.0.1": "localhost",
//				},
//			},
//		},
//	}
//}
//
//func TestNewTcpServer(t *testing.T) {
//	convey.Convey("测试 TcpServerConnector 的初始化", t, func() {
//		// 模拟上下文和配置
//		ctx := pkg.WithConfig(commonCtx, mockConfigFromContext())
//
//		convey.Convey("成功创建 TcpServerConnector", func() {
//			server, err := NewTcpServer(ctx)
//			convey.So(err, convey.ShouldBeNil)
//			convey.So(server, convey.ShouldNotBeNil)
//			convey.So(server.(*TcpServerConnector).serverConfig.Url, convey.ShouldEqual, ":8081")
//		})
//
//	})
//}
//
//func TestTcpServerConnector_Start(t *testing.T) {
//	convey.Convey("测试 TcpServerConnector 启动", t, func() {
//		// 模拟上下文和配置
//		ctx := pkg.WithConfig(commonCtx, mockConfigFromContext())
//
//		// 创建 server 实例
//		server, err := NewTcpServer(ctx)
//		convey.So(err, convey.ShouldBeNil)
//
//		// 创建一个模拟的 sourceChan
//		sourceChan := make(chan pkg.DataSource)
//
//		convey.Convey("成功启动服务器", func() {
//			// 启动服务器
//			err = server.(*TcpServerConnector).Start(sourceChan)
//			convey.So(err, convey.ShouldBeNil)
//
//			// 模拟一个连接
//			conn, err := net.Dial("tcp", "localhost:8081")
//			convey.So(err, convey.ShouldBeNil)
//			defer conn.Close()
//
//			// 检查是否能收到数据源
//			select {
//			case ds := <-sourceChan:
//				convey.So(ds, convey.ShouldNotBeNil)
//			case <-time.After(1 * time.Second):
//				convey.So(false, convey.ShouldBeTrue) // 如果没有收到数据源则失败
//			}
//		})
//	})
//}
//
//func TestTcpServerConnector_handleConn(t *testing.T) {
//	convey.Convey("测试 TcpServerConnector 连接处理", t, func() {
//		// 模拟上下文和配置
//		ctx := pkg.WithConfig(commonCtx, mockConfigFromContext())
//
//		// 创建 server 实例
//		server, err := NewTcpServer(ctx)
//		convey.So(err, convey.ShouldBeNil)
//
//		// 模拟连接
//		serverConn, clientConn := net.Pipe()
//
//		// 创建一个 source 用于写入数据
//		ds := pkg.NewStreamDataSource()
//
//		convey.Convey("正确处理连接数据", func() {
//			go server.(*TcpServerConnector).handleConn(serverConn, "testConn", ds)
//
//			// 向模拟连接写入数据
//			clientConn.Write([]byte("test data"))
//			buffer := make([]byte, 1024)
//			// 检查数据是否成功写入到 source
//			var n int
//			n, err = ds.Read(buffer)
//			convey.So(string(buffer[:n]), convey.ShouldEqual, "test data")
//		})
//
//		convey.Convey("连接关闭时正确清理资源", func() {
//			// 模拟关闭连接
//			serverConn.Close()
//
//			// 等待一段时间确保连接已关闭
//			time.Sleep(100 * time.Millisecond)
//		})
//	})
//}
