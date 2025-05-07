package integration

import (
	"context"
	"fmt"
	"gateway/internal/connector"
	"gateway/internal/pkg"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"gateway/test/integration/helpers"

	. "github.com/smartystreets/goconvey/convey"
	"go.uber.org/zap/zaptest"
)

// TestTCPClientIntegration 测试TCP客户端连接到服务器并发送数据的完整流程
func TestTCPClientIntegration(t *testing.T) {
	// 跳过长时间运行测试，如果添加-short参数
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	Convey("测试TCP客户端数据处理", t, func() {
		// 确保 MemorySink 已注册
		helpers.RegisterMemorySink()

		// 设置测试环境
		logger := zaptest.NewLogger(t)
		testPort := 19001
		testTCPAddr := fmt.Sprintf("127.0.0.1:%d", testPort)

		// 1. 创建测试协议配置
		config := &pkg.Config{
			Parser: pkg.ParserConfig{
				Type: "byte",
				Para: map[string]interface{}{
					"protoFile": "test_proto",
				},
			},
			Connector: pkg.ConnectorConfig{
				Type: "tcpclient",
				Para: map[string]interface{}{
					"serveraddrs":    testTCPAddr,
					"timeout":        "5s",
					"reconnectdelay": "1s",
					"bufferSize":     16 * 1024,
				},
			},
			Others: map[string]interface{}{
				"test_proto": map[string]interface{}{
					"chunks": []interface{}{
						map[string]interface{}{
							"type": "DefaultChunk",
							"desc": "测试TCP客户端数据包",
							"sections": []interface{}{
								map[string]interface{}{
									"desc":      "TCP测试段",
									"repeat":    1,
									"size":      4,
									"to_device": "tcp_client_test_device",
									"fields": map[string]interface{}{
										"value1": "Bytes[0]",
										"value2": "int(Bytes[1]) * 256 + int(Bytes[2])",
										"status": "Bytes[3]",
									},
								},
							},
						},
					},
				},
			},
		}

		// 2. 设置上下文
		ctx, cancel := context.WithCancel(context.Background())
		ctx = pkg.WithLogger(ctx, logger)
		ctx = pkg.WithConfig(ctx, config)
		ctx = InitErrorChannel(ctx)

		// 声明在外部作用域
		var clientConnector connector.Template

		// 确保测试结束时取消上下文
		Reset(func() {
			// 关闭TCP客户端连接
			if tcpClient, ok := clientConnector.(*connector.TcpClientConnector); ok && tcpClient != nil {
				tcpClient.Done()
			}
			cancel()
		})

		Convey("当启动模拟TCP服务器", func() {
			// 创建TCP服务器监听
			listener, err := net.Listen("tcp", testTCPAddr)
			So(err, ShouldBeNil)
			defer listener.Close()

			// 创建一个通道用于通知服务器接收到了连接
			connReceived := make(chan struct{})
			// 创建一个通道用于通知测试服务器已启动
			serverReady := make(chan struct{})
			// 创建一个通道用于接收收到的数据
			receivedData := make(chan []byte)

			// 启动服务器协程
			go func() {
				defer close(receivedData)

				// 通知测试服务器已启动
				close(serverReady)

				// 等待客户端连接
				conn, err := listener.Accept()
				if err != nil {
					t.Logf("接受连接错误: %v", err)
					return
				}
				defer conn.Close()

				// 通知已收到连接
				close(connReceived)

				// 设置读取超时
				conn.SetReadDeadline(time.Now().Add(5 * time.Second))

				// 先向客户端发送测试数据
				testData := []byte{0xAA, 0x01, 0x02, 0xFF}
				_, err = conn.Write(testData)
				if err != nil {
					t.Logf("发送数据错误: %v", err)
					return
				}

				// 读取客户端可能的回复数据
				buffer := make([]byte, 1024)
				n, err := conn.Read(buffer)
				if err != nil {
					// 忽略读取超时错误，因为客户端可能不会回复数据
					if !strings.Contains(err.Error(), "timeout") {
						t.Logf("读取数据错误: %v", err)
					}
					return
				}

				// 如果收到数据，发送到通道
				receivedData <- buffer[:n]
			}()

			// 等待服务器准备就绪
			<-serverReady

			Convey("当创建并启动TCP客户端", func() {
				// 创建输出通道
				outputChan := make(pkg.Parser2DispatcherChan, 10)

				// 创建 TCP 客户端连接器
				var err error
				clientConnector, err = connector.New(ctx)
				So(err, ShouldBeNil)

				// 启动客户端
				err = clientConnector.Start(&outputChan)
				So(err, ShouldBeNil)

				// 等待客户端连接到服务器
				select {
				case <-connReceived:
					// 连接成功建立
				case <-time.After(3 * time.Second):
					So(false, ShouldBeTrue, "连接超时")
				}

				// 等待接收数据包
				var result *pkg.PointPackage
				select {
				case result = <-outputChan:
					// 成功接收到数据
				case <-time.After(5 * time.Second):
					So(false, ShouldBeTrue, "接收数据超时")
				}

				// 验证结果
				Convey("应该正确解析TCP数据", func() {
					So(result, ShouldNotBeNil)
					So(result.Points, ShouldNotBeEmpty)

					if len(result.Points) > 0 {
						point := result.Points[0]
						So(point.Device, ShouldEqual, "tcp_client_test_device")

						// 验证字段值
						So(point.Field["value1"], ShouldEqual, byte(0xAA))
						So(point.Field["value2"], ShouldEqual, 258) // 0x0102 = 258
						So(point.Field["status"], ShouldEqual, byte(0xFF))
					}
				})
			})
		})
	})
}

// TestTCPClientReconnection 测试TCP客户端的重连功能
func TestTCPClientReconnection(t *testing.T) {
	// 跳过长时间运行测试，如果添加-short参数
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	Convey("测试TCP客户端重连功能", t, func() {
		// 设置测试环境
		logger := zaptest.NewLogger(t)
		testPort := 19002
		testTCPAddr := fmt.Sprintf("127.0.0.1:%d", testPort)

		// 1. 创建测试协议配置
		config := &pkg.Config{
			Parser: pkg.ParserConfig{
				Type: "byte",
				Para: map[string]interface{}{
					"protoFile": "test_proto",
				},
			},
			Connector: pkg.ConnectorConfig{
				Type: "tcpclient",
				Para: map[string]interface{}{
					"serveraddrs":    testTCPAddr,
					"timeout":        "5s",
					"reconnectdelay": "1s", // 设置较短的重连延迟以加快测试
					"bufferSize":     16 * 1024,
				},
			},
			Others: map[string]interface{}{
				"test_proto": map[string]interface{}{
					"chunks": []interface{}{
						map[string]interface{}{
							"type": "DefaultChunk",
							"desc": "测试TCP客户端数据包",
							"sections": []interface{}{
								map[string]interface{}{
									"size": 4,
									"desc": "测试段",
									"fields": map[string]interface{}{
										"value": "BytesToInt(Bytes, \"big\")",
									},
								},
							},
						},
					},
				},
			},
		}

		// 2. 设置上下文
		ctx, cancel := context.WithCancel(context.Background())
		ctx = pkg.WithLogger(ctx, logger)
		ctx = pkg.WithConfig(ctx, config)
		ctx = InitErrorChannel(ctx)

		// 声明在外部作用域
		var clientConnector connector.Template

		// 确保测试结束时取消上下文
		Reset(func() {
			// 关闭TCP客户端连接
			if tcpClient, ok := clientConnector.(*connector.TcpClientConnector); ok && tcpClient != nil {
				tcpClient.Done()
			}
			cancel()
		})

		Convey("当TCP客户端启动时服务器不可用", func() {
			// 创建输出通道
			outputChan := make(pkg.Parser2DispatcherChan, 10)

			// 创建 TCP 客户端连接器
			var err error
			clientConnector, err = connector.New(ctx)
			So(err, ShouldBeNil)

			// 启动客户端，此时服务器未启动
			err = clientConnector.Start(&outputChan)
			So(err, ShouldBeNil)

			// 等待一段时间，让客户端尝试连接
			time.Sleep(2 * time.Second)

			Convey("当服务器稍后启动", func() {
				// 创建TCP服务器监听
				listener, err := net.Listen("tcp", testTCPAddr)
				So(err, ShouldBeNil)
				defer listener.Close()

				// 创建一个通道用于通知服务器接收到了连接
				connReceived := make(chan struct{})
				// 创建一个互斥锁保护连接计数
				var connMutex sync.Mutex
				connCount := 0

				// 启动服务器协程
				go func() {
					for {
						// 等待客户端连接
						conn, err := listener.Accept()
						if err != nil {
							// 监听器已关闭，退出循环
							return
						}

						// 计数并通知已收到连接
						connMutex.Lock()
						connCount++
						if connCount == 1 {
							close(connReceived)
						}
						connMutex.Unlock()

						// 处理连接
						go func(c net.Conn) {
							defer c.Close()

							// 设置读取超时
							c.SetReadDeadline(time.Now().Add(2 * time.Second))

							// 发送测试数据给客户端
							testData := []byte{0x01, 0x02, 0x03, 0x04}
							c.Write(testData)

							// 模拟连接保持一段时间后关闭（强制客户端重连）
							time.Sleep(500 * time.Millisecond)
						}(conn)
					}
				}()

				// 等待客户端连接到服务器
				select {
				case <-connReceived:
					// 连接成功建立
				case <-time.After(5 * time.Second):
					So(false, ShouldBeTrue, "客户端未重连到服务器")
				}

				// 检查是否接收到数据
				select {
				case pointPackage := <-outputChan:
					So(pointPackage, ShouldNotBeNil)
					So(pointPackage.Points, ShouldNotBeEmpty)
				case <-time.After(3 * time.Second):
					So(false, ShouldBeTrue, "未收到数据包")
				}

				// 等待一段时间让客户端发现连接断开并重连
				time.Sleep(2 * time.Second)

				// 验证客户端是否重连
				connMutex.Lock()
				reconnected := connCount > 1
				connMutex.Unlock()

				So(reconnected, ShouldBeTrue)
			})
		})
	})
}

// TestTCPClientMultipleServers 测试TCP客户端连接多个服务器
func TestTCPClientMultipleServers(t *testing.T) {
	// 跳过长时间运行测试，如果添加-short参数
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	Convey("测试TCP客户端连接多个服务器", t, func() {
		// 设置测试环境
		logger := zaptest.NewLogger(t)
		testPort1 := 19003
		testPort2 := 19004
		testTCPAddr1 := fmt.Sprintf("127.0.0.1:%d", testPort1)
		testTCPAddr2 := fmt.Sprintf("127.0.0.1:%d", testPort2)

		// 1. 创建测试协议配置
		config := &pkg.Config{
			Parser: pkg.ParserConfig{
				Type: "byte",
				Para: map[string]interface{}{
					"protoFile": "test_proto",
				},
			},
			Connector: pkg.ConnectorConfig{
				Type: "tcpclient",
				Para: map[string]interface{}{
					"serveraddrs":    fmt.Sprintf("%s,%s", testTCPAddr1, testTCPAddr2),
					"timeout":        "5s",
					"reconnectdelay": "1s",
					"bufferSize":     16 * 1024,
				},
			},
			Others: map[string]interface{}{
				"test_proto": map[string]interface{}{
					"chunks": []interface{}{
						map[string]interface{}{
							"type": "DefaultChunk",
							"desc": "测试TCP客户端数据包",
							"sections": []interface{}{
								map[string]interface{}{
									"size": 4,
									"desc": "测试段",
									"fields": map[string]interface{}{
										"server_id": "Bytes[0]",
										"value":     "BytesToInt(Bytes, \"big\")",
									},
								},
							},
						},
					},
				},
			},
		}

		// 2. 设置上下文
		ctx, cancel := context.WithCancel(context.Background())
		ctx = pkg.WithLogger(ctx, logger)
		ctx = pkg.WithConfig(ctx, config)
		ctx = InitErrorChannel(ctx)

		// 声明在外部作用域
		var clientConnector connector.Template

		// 确保测试结束时取消上下文
		Reset(func() {
			// 关闭TCP客户端连接
			if tcpClient, ok := clientConnector.(*connector.TcpClientConnector); ok && tcpClient != nil {
				tcpClient.Done()
			}
			cancel()
		})

		Convey("当启动两个模拟TCP服务器", func() {
			var listeners [2]net.Listener

			// 启动第一个服务器
			listener1, err := net.Listen("tcp", testTCPAddr1)
			So(err, ShouldBeNil)
			listeners[0] = listener1
			defer listener1.Close()

			// 启动第二个服务器
			listener2, err := net.Listen("tcp", testTCPAddr2)
			So(err, ShouldBeNil)
			listeners[1] = listener2
			defer listener2.Close()

			// 准备接收的服务器连接数和服务器互斥锁
			connectionsReceived := make(chan int, 2)
			var serversMutex sync.Mutex
			connectedServers := make(map[int]bool)

			// 为每个服务器启动处理协程
			for serverIdx, listener := range listeners {
				serverIdx := serverIdx // 捕获变量
				go func(l net.Listener) {
					// 等待客户端连接
					conn, err := l.Accept()
					if err != nil {
						t.Logf("接受连接错误: %v", err)
						return
					}
					defer conn.Close()

					// 记录连接并通知
					serversMutex.Lock()
					connectedServers[serverIdx] = true
					serversMutex.Unlock()
					connectionsReceived <- serverIdx

					// 设置读取超时
					conn.SetReadDeadline(time.Now().Add(5 * time.Second))

					// 发送特定于服务器的测试数据
					testData := []byte{byte(serverIdx + 1), 0x11, 0x22, 0x33}
					conn.Write(testData)
				}(listener)
			}

			Convey("当创建并启动TCP客户端", func() {
				// 创建输出通道
				outputChan := make(pkg.Parser2DispatcherChan, 10)

				// 创建 TCP 客户端连接器
				var err error
				clientConnector, err = connector.New(ctx)
				So(err, ShouldBeNil)

				// 启动客户端
				err = clientConnector.Start(&outputChan)
				So(err, ShouldBeNil)

				// 等待两个服务器都收到连接
				serversConnected := 0
				timeout := time.After(5 * time.Second)
				for serversConnected < 2 {
					select {
					case <-connectionsReceived:
						serversConnected++
					case <-timeout:
						break
					}
				}

				// 验证两台服务器都已连接
				serversMutex.Lock()
				numConnected := len(connectedServers)
				serversMutex.Unlock()
				So(numConnected, ShouldEqual, 2)

				// 跟踪收到的服务器数据
				receivedFromServer := make(map[byte]bool)
				dataReceived := make(chan struct{})

				// 启动一个协程接收数据
				go func() {
					received := 0
					for received < 2 {
						select {
						case data := <-outputChan:
							if data != nil && len(data.Points) > 0 {
								serverId, ok := data.Points[0].Field["server_id"].(byte)
								if ok {
									receivedFromServer[serverId] = true
									received++
								}
							}
						case <-time.After(3 * time.Second):
							break
						}
					}
					close(dataReceived)
				}()

				// 等待接收完成
				select {
				case <-dataReceived:
					// 数据接收完成
				case <-time.After(5 * time.Second):
					So(false, ShouldBeTrue)
				}

				// 验证结果
				So(len(receivedFromServer), ShouldEqual, 2)
				So(receivedFromServer[1], ShouldBeTrue)
				So(receivedFromServer[2], ShouldBeTrue)
			})
		})
	})
}
