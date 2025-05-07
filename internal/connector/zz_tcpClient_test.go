package connector

//
//import (
//	"context"
//	"fmt"
//	"gateway/internal/pkg"
//	"net"
//	"testing"
//	"time"
//
//	. "github.com/smartystreets/goconvey/convey"
//	"go.uber.org/zap"
//)
//
//func TestNewTcpClient_ConfigParsing(t *testing.T) {
//
//	Convey("测试配置解析", t, func() {
//		ctx := pkg.WithConfig(pkg.WithLogger(context.Background(), logger), &pkg.Config{
//			Connector: pkg.ConnectorConfig{
//				Type: "tcpclient",
//				Para: map[string]interface{}{
//					"serveraddrs":    "127.0.0.1:8080,127.0.0.2:8080",
//					"timeout":        "5s",
//					"reconnectdelay": "2s",
//				},
//			},
//		})
//
//		connector, err := NewTcpClient(ctx)
//		Convey("解析应无错误", func() {
//			So(err, ShouldBeNil)
//		})
//
//		tcpClient := connector.(*TcpClientConnector)
//		Convey("配置应正确解析", func() {
//			So(tcpClient.clientConfig.ServerAddrs, ShouldResemble, []string{"127.0.0.1:8080", "127.0.0.2:8080"})
//			So(tcpClient.clientConfig.Timeout, ShouldEqual, 5*time.Second)
//			So(tcpClient.clientConfig.ReconnectDelay, ShouldEqual, 2*time.Second)
//		})
//	})
//}
//
//func TestTcpClientConnector_Start(t *testing.T) {
//	Convey("测试 Start 方法", t, func() {
//		ctx := pkg.WithLogger(context.Background(), zap.NewExample())
//
//		client := &TcpClientConnector{
//			ctx: ctx,
//			clientConfig: &tcpClientConfig{
//				ServerAddrs: []string{"127.0.0.1:8080"},
//			},
//		}
//
//		sourceChan := make(chan pkg.DataSource, 1)
//
//		err := client.Start(sourceChan)
//		Convey("Start 应该成功返回", func() {
//			So(err, ShouldBeNil)
//		})
//
//		Convey("应向 channel 发送数据源", func() {
//			So(len(sourceChan), ShouldEqual, 1)
//		})
//	})
//}
//
//// Mock TCP Server
//func startMockTCPServer(t *testing.T, port string) net.Listener {
//	// 启动一个 mock TCP 服务器
//	ln, err := net.Listen("tcp", ":"+port)
//	if err != nil {
//		t.Fatalf("无法启动 mock TCP 服务器: %v", err)
//	}
//	go func() {
//		for {
//			conn, err := ln.Accept()
//			if err != nil {
//				return
//			}
//
//			// 模拟接收到客户端连接，向客户端发送数据
//			go func(conn net.Conn) {
//				defer func(conn net.Conn) {
//					err = conn.Close()
//					if err != nil {
//						fmt.Println("关闭连接失败:", err)
//					}
//				}(conn)
//				message := []byte("hello from server")
//				_, err := conn.Write(message)
//				if err != nil {
//					fmt.Println("发送数据失败:", err)
//				}
//			}(conn)
//		}
//	}()
//	return ln
//}
//
//func TestTcpClientConnector_manageConnection(t *testing.T) {
//	// 初始化测试用例
//	Convey("Given a TcpClientConnector", t, func() {
//
//		ln := startMockTCPServer(t, "8080")
//		defer ln.Close()
//
//		// 模拟创建 TcpClientConnector
//		Convey("When I create a new TcpClientConnector", func() {
//			// 创建一个模拟的 TcpClientConnector 实例
//			clientConfig := &tcpClientConfig{
//				ServerAddrs:    []string{"127.0.0.1:8080"},
//				Timeout:        5 * time.Second,
//				ReconnectDelay: 1 * time.Second,
//			}
//
//			// 传递 mock 的配置数据
//			client := &TcpClientConnector{
//				ctx:          commonCtx,
//				clientConfig: clientConfig,
//			}
//
//			Convey("Then the connector should be initialized", func() {
//				// 确保 connector 已被正确初始化
//				So(client, ShouldNotBeNil)
//				So(client.clientConfig.ServerAddrs, ShouldResemble, []string{"127.0.0.1:8080"})
//			})
//
//			// 启动客户端连接
//			ds := pkg.NewStreamDataSource()
//			go client.manageConnection("127.0.0.1:"+"8080", ds)
//
//			Convey("When the client connects to the mock server", func() {
//				buffer := make([]byte, 17)
//				length, err := ds.ReadFully(buffer)
//				if err != nil {
//					t.Fatalf("读取数据失败: %v", err)
//				}
//				Convey("Then the client should receive the message from the server", func() {
//					// 确保客户端从服务器接收到了消息
//					So(string(buffer[:length]), ShouldEqual, "hello from server")
//				})
//			})
//		})
//	})
//}
