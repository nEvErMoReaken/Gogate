package integration

import (
	"context"
	"fmt"
	"gateway/internal/connector"
	"gateway/internal/pkg"
	"net"
	"sync"
	"testing"
	"time"

	"gateway/test/integration/helpers"

	. "github.com/smartystreets/goconvey/convey"
	"go.uber.org/zap/zaptest"
)

// TestTCPServerIntegration 测试TCP服务器数据从接收到解析到分发到sink的完整流程
func TestTCPServerIntegration(t *testing.T) {
	// 跳过长时间运行测试，如果添加-short参数
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	Convey("测试TCP服务器数据处理", t, func() {
		// 确保 MemorySink 已注册
		helpers.RegisterMemorySink()

		Convey("使用内联协议配置", func() {
			// 1. 创建测试配置
			testPort := 18889 // 使用固定的高端口
			testTCPAddr := fmt.Sprintf("127.0.0.1:%d", testPort)

			config := &pkg.Config{
				Parser: pkg.ParserConfig{
					Type: "byte",
					Para: map[string]interface{}{
						"protoFile": "test_proto",
					},
				},
				Connector: pkg.ConnectorConfig{
					Type: "tcpserver",
					Para: map[string]interface{}{
						"url":        testTCPAddr,
						"whiteList":  false,
						"timeout":    "5s",
						"bufferSize": 16 * 1024,
					},
				},
				Others: map[string]interface{}{
					"test_proto": map[string]interface{}{
						"chunks": []interface{}{
							map[string]interface{}{
								"type": "DefaultChunk",
								"desc": "测试TCP数据包",
								"sections": []interface{}{
									map[string]interface{}{
										"desc":      "TCP测试段",
										"repeat":    1,
										"size":      4,
										"to_device": "tcp_test_device",
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
			logger := zaptest.NewLogger(t)
			ctx, cancel := context.WithCancel(context.Background())
			ctx = pkg.WithLogger(ctx, logger)
			ctx = pkg.WithConfig(ctx, config)
			ctx = InitErrorChannel(ctx)

			// 确保测试结束时取消上下文
			Reset(func() {
				cancel()
			})

			Convey("当创建并启动TCP服务器连接器", func() {
				// 创建输出通道
				outputChan := make(pkg.Parser2DispatcherChan, 10)

				// 创建 TCPServer 连接器
				tcpServer, err := connector.New(ctx)
				So(err, ShouldBeNil)

				// 启动服务器
				err = tcpServer.Start(&outputChan)
				So(err, ShouldBeNil)

				// 等待服务器启动
				time.Sleep(500 * time.Millisecond)

				Convey("当发送TCP测试数据", func() {
					// 4. 发送测试数据
					testData := []byte{0xAA, 0x01, 0x02, 0xFF}
					err = sendTCPPacket(testTCPAddr, testData)
					So(err, ShouldBeNil)

					// 5. 等待处理结果
					var receivedData *pkg.PointPackage
					select {
					case receivedData = <-outputChan:
						// 成功接收到数据
					case <-time.After(3 * time.Second):
						So(true, ShouldBeFalse) // 报告超时
					}

					// 6. 验证结果
					Convey("应该正确解析TCP数据", func() {
						So(receivedData, ShouldNotBeNil)
						So(receivedData.Points, ShouldNotBeEmpty)

						if len(receivedData.Points) > 0 {
							point := receivedData.Points[0]
							So(point.Device, ShouldEqual, "tcp_test_device")

							// 验证字段值
							So(point.Field["value1"], ShouldEqual, byte(0xAA))
							So(point.Field["value2"], ShouldEqual, 258) // 0x0102 = 258
							So(point.Field["status"], ShouldEqual, byte(0xFF))
						}
					})
				})
			})
		})
	})
}

// TestMultipleTCPClients 测试多个TCP客户端同时连接
func TestMultipleTCPClients(t *testing.T) {
	// 跳过长时间运行测试，如果添加-short参数
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	Convey("测试多个TCP客户端", t, func() {
		// 设置测试环境
		logger := zaptest.NewLogger(t)

		// 1. 创建测试配置
		testPort := 18890 // 使用固定的高端口
		testTCPAddr := fmt.Sprintf("127.0.0.1:%d", testPort)

		config := &pkg.Config{
			Parser: pkg.ParserConfig{
				Type: "byte",
				Para: map[string]interface{}{
					"protoFile": "test_proto",
				},
			},
			Connector: pkg.ConnectorConfig{
				Type: "tcpserver",
				Para: map[string]interface{}{
					"url":        testTCPAddr,
					"whiteList":  false,
					"timeout":    "5s",
					"bufferSize": 16 * 1024,
				},
			},
			Others: map[string]interface{}{
				"test_proto": map[string]interface{}{
					"chunks": []interface{}{
						map[string]interface{}{
							"type": "DefaultChunk",
							"desc": "测试TCP数据包",
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

		ctx, cancel := context.WithCancel(context.Background())
		ctx = pkg.WithLogger(ctx, logger)
		ctx = pkg.WithConfig(ctx, config)
		ctx = InitErrorChannel(ctx)

		Reset(func() {
			cancel() // 确保测试完成后取消上下文
		})

		Convey("创建并启动TCP服务器", func() {
			// 创建输出通道
			outputChan := make(pkg.Parser2DispatcherChan, 100)

			// 创建 TCPServer 连接器
			tcpServer, err := connector.New(ctx)
			So(err, ShouldBeNil)

			// 启动连接器
			err = tcpServer.Start(&outputChan)
			So(err, ShouldBeNil)

			// 等待服务器启动
			time.Sleep(500 * time.Millisecond)

			Convey("并发发送多个客户端的数据", func() {
				// 使用WaitGroup等待所有协程完成
				var wg sync.WaitGroup
				var errMutex sync.Mutex
				var sendErr error

				// 定义客户端数量
				const clientCount = 5

				// 并发发送数据
				for i := 0; i < clientCount; i++ {
					wg.Add(1)
					go func(idx int) {
						defer wg.Done()

						// 每个客户端发送不同的数据
						testData := []byte{byte(idx + 1), 0x02, 0x03, 0x04}

						err := sendTCPPacket(testTCPAddr, testData)
						if err != nil {
							errMutex.Lock()
							sendErr = err
							errMutex.Unlock()
						}
					}(i)
				}

				// 等待所有发送完成
				wg.Wait()

				// 检查是否有发送错误
				So(sendErr, ShouldBeNil)

				// 计数收到的数据包
				receivedCount := 0
				timeout := time.After(3 * time.Second)

				// 接收并计数处理后的数据
				for receivedCount < clientCount {
					select {
					case <-outputChan:
						receivedCount++
					case <-timeout:
						// 如果超时，退出循环
						t.Logf("收到 %d 个数据包，少于预期的 %d 个", receivedCount, clientCount)
						break
					}
				}

				// 验证结果
				So(receivedCount, ShouldEqual, clientCount)
			})
		})
	})
}

// TestTCPLongConnection 测试TCP长连接数据发送
func TestTCPLongConnection(t *testing.T) {
	// 跳过长时间运行测试，如果添加-short参数
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	Convey("测试TCP长连接数据传输", t, func() {
		// 设置测试环境
		logger := zaptest.NewLogger(t)

		// 1. 创建测试配置
		testPort := 18891 // 使用固定的高端口
		testTCPAddr := fmt.Sprintf("127.0.0.1:%d", testPort)

		config := &pkg.Config{
			Parser: pkg.ParserConfig{
				Type: "byte",
				Para: map[string]interface{}{
					"protoFile": "test_proto",
				},
			},
			Connector: pkg.ConnectorConfig{
				Type: "tcpserver",
				Para: map[string]interface{}{
					"url":        testTCPAddr,
					"whiteList":  false,
					"timeout":    "30s", // 长连接超时时间设置较长
					"bufferSize": 16 * 1024,
				},
			},
			Others: map[string]interface{}{
				"test_proto": map[string]interface{}{
					"chunks": []interface{}{
						map[string]interface{}{
							"type": "DefaultChunk",
							"desc": "测试TCP数据包",
							"sections": []interface{}{
								map[string]interface{}{
									"size": 4,
									"desc": "测试段",
									"fields": map[string]interface{}{
										"value":      "BytesToInt(Bytes, \"big\")",
										"first_byte": "Bytes[0]",
									},
								},
							},
						},
					},
				},
			},
		}

		ctx, cancel := context.WithCancel(context.Background())
		ctx = pkg.WithLogger(ctx, logger)
		ctx = pkg.WithConfig(ctx, config)
		ctx = InitErrorChannel(ctx)

		Reset(func() {
			cancel() // 确保测试完成后取消上下文
		})

		Convey("创建并启动TCP服务器", func() {
			// 创建输出通道
			outputChan := make(pkg.Parser2DispatcherChan, 100)

			// 创建 TCPServer 连接器
			tcpServer, err := connector.New(ctx)
			So(err, ShouldBeNil)

			// 启动连接器
			err = tcpServer.Start(&outputChan)
			So(err, ShouldBeNil)

			// 等待服务器启动
			time.Sleep(500 * time.Millisecond)

			Convey("使用长连接发送多次数据", func() {
				// 建立长连接
				conn, err := net.Dial("tcp", testTCPAddr)
				So(err, ShouldBeNil)
				defer conn.Close()

				// 发送数据次数
				const sendCount = 10

				// 发送多组数据
				for i := 0; i < sendCount; i++ {
					// 每次发送不同的数据
					testData := []byte{byte(i + 1), 0x02, 0x03, 0x04}

					_, err := conn.Write(testData)
					So(err, ShouldBeNil)

					// 小延迟，确保数据能够被处理
					time.Sleep(50 * time.Millisecond)
				}

				// 统计收到的数据包
				receivedCount := 0
				receivedValues := make(map[byte]bool)
				timeout := time.After(3 * time.Second)

				// 接收并验证处理后的数据
				for receivedCount < sendCount {
					select {
					case data := <-outputChan:
						receivedCount++

						// 验证数据包
						So(data, ShouldNotBeNil)
						So(data.Points, ShouldNotBeEmpty)

						if len(data.Points) > 0 {
							// 记录收到的第一个字节值
							firstByte, ok := data.Points[0].Field["first_byte"].(byte)
							So(ok, ShouldBeTrue)
							receivedValues[firstByte] = true
						}

					case <-timeout:
						// 如果超时，退出循环
						break
					}
				}

				// 验证结果 - 收到足够数量的数据包
				So(receivedCount, ShouldEqual, sendCount)

				// 验证接收到的值是不是预期的值
				for i := 0; i < sendCount; i++ {
					So(receivedValues[byte(i+1)], ShouldBeTrue)
				}
			})
		})
	})
}

// TestTCPFragmentedData 测试TCP服务器处理分片数据的能力
func TestTCPFragmentedData(t *testing.T) {
	// 跳过长时间运行测试，如果添加-short参数
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	Convey("测试TCP服务器处理分片数据", t, func() {
		// 设置测试环境
		logger := zaptest.NewLogger(t)

		// 1. 创建测试配置
		testPort := 18892 // 使用固定的高端口
		testTCPAddr := fmt.Sprintf("127.0.0.1:%d", testPort)

		config := &pkg.Config{
			Parser: pkg.ParserConfig{
				Type: "byte",
				Para: map[string]interface{}{
					"protoFile": "test_proto",
				},
			},
			Connector: pkg.ConnectorConfig{
				Type: "tcpserver",
				Para: map[string]interface{}{
					"url":        testTCPAddr,
					"whiteList":  false,
					"timeout":    "5s",
					"bufferSize": 16 * 1024,
				},
			},
			Others: map[string]interface{}{
				"test_proto": map[string]interface{}{
					"chunks": []interface{}{
						map[string]interface{}{
							"type": "DefaultChunk",
							"desc": "测试大数据包",
							"sections": []interface{}{
								map[string]interface{}{
									"size": 8,
									"desc": "大数据段",
									"fields": map[string]interface{}{
										"header":  "Bytes[0]",
										"command": "Bytes[1]",
										"value":   "BytesToInt(Bytes[2:6], \"big\")",
										"tail":    "Bytes[7]",
									},
								},
							},
						},
					},
				},
			},
		}

		ctx, cancel := context.WithCancel(context.Background())
		ctx = pkg.WithLogger(ctx, logger)
		ctx = pkg.WithConfig(ctx, config)
		ctx = InitErrorChannel(ctx)

		Reset(func() {
			cancel() // 确保测试完成后取消上下文
		})

		Convey("创建并启动TCP服务器", func() {
			// 创建输出通道
			outputChan := make(pkg.Parser2DispatcherChan, 10)

			// 创建 TCPServer 连接器
			tcpServer, err := connector.New(ctx)
			So(err, ShouldBeNil)

			// 启动连接器
			err = tcpServer.Start(&outputChan)
			So(err, ShouldBeNil)

			// 等待服务器启动
			time.Sleep(500 * time.Millisecond)

			Convey("分片发送较大数据包", func() {
				// 建立TCP连接
				conn, err := net.Dial("tcp", testTCPAddr)
				So(err, ShouldBeNil)
				defer conn.Close()

				// 准备测试数据（8字节完整数据包）
				testData := []byte{
					0xAA,                   // 头部标识
					0x01,                   // 命令字
					0x12, 0x34, 0x56, 0x78, // 数据值（大端序整数）
					0x00, // 保留字节
					0xFF, // 尾部标识
				}

				// 第一次发送部分数据（前4个字节）
				_, err = conn.Write(testData[:4])
				So(err, ShouldBeNil)

				// 短暂延迟，模拟网络延迟
				time.Sleep(100 * time.Millisecond)

				// 第二次发送剩余数据
				_, err = conn.Write(testData[4:])
				So(err, ShouldBeNil)

				// 等待处理结果
				var receivedData *pkg.PointPackage
				select {
				case receivedData = <-outputChan:
					// 成功接收到数据
				case <-time.After(3 * time.Second):
					So(true, ShouldBeFalse) // 报告超时
				}

				// 验证结果
				Convey("应该正确拼接并解析分片数据", func() {
					So(receivedData, ShouldNotBeNil)
					So(receivedData.Points, ShouldNotBeEmpty)

					if len(receivedData.Points) > 0 {
						point := receivedData.Points[0]

						// 验证字段值
						So(point.Field["header"], ShouldEqual, byte(0xAA))
						So(point.Field["command"], ShouldEqual, byte(0x01))
						So(point.Field["value"], ShouldEqual, int64(0x12345678))
						So(point.Field["tail"], ShouldEqual, byte(0xFF))
					}
				})
			})
		})
	})
}

// sendTCPPacket 发送TCP数据包到指定地址
func sendTCPPacket(addr string, data []byte) error {
	// 建立TCP连接
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("连接TCP地址失败: %v", err)
	}
	defer conn.Close()

	// 写入数据
	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("写入TCP数据失败: %v", err)
	}

	return nil
}
