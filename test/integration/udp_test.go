package integration

import (
	"context"
	"fmt"
	"gateway/internal/connector"
	"gateway/internal/pkg"
	"gateway/internal/sink"
	"net"
	"sync"
	"testing"
	"time"

	"gateway/test/integration/helpers"

	. "github.com/smartystreets/goconvey/convey"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// 全局变量用于跟踪 MemorySink 实例
var globalMemorySink *helpers.MemorySink

// TestUDPPipelineIntegration 测试UDP数据从接收到解析到分发到sink的完整流程
func TestUDPPipelineIntegration(t *testing.T) {
	// 跳过长时间运行测试，如果添加-short参数
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	Convey("测试UDP数据包解析", t, func() {
		// 确保 MemorySink 已注册
		helpers.RegisterMemorySink()

		Convey("使用内联协议配置", func() {
			// 1. 创建测试配置
			config := &pkg.Config{
				Parser: pkg.ParserConfig{
					Type: "byte",
					Para: map[string]interface{}{
						"protoFile": "test_proto",
					},
				},
				Others: map[string]interface{}{
					"test_proto": map[string]interface{}{
						"chunks": []interface{}{
							map[string]interface{}{
								"type": "DefaultChunk",
								"desc": "测试UDP数据包",
								"sections": []interface{}{
									map[string]interface{}{
										"desc":      "UDP测试段",
										"repeat":    1,
										"size":      4,
										"to_device": "udp_test_device",
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
			ctx := pkg.WithLogger(context.Background(), logger)
			ctx = pkg.WithConfig(ctx, config)
			ctx = InitErrorChannel(ctx)

			// 3. 注册并创建 MockUDPConnector
			helpers.RegisterMockConnector()

			Convey("当创建并启动MockUDPConnector", func() {
				// 创建输出通道
				outputChan := make(pkg.Parser2DispatcherChan, 10)

				// 设置配置中的连接器类型
				config.Connector = pkg.ConnectorConfig{
					Type: "mock_udp",
					Para: map[string]interface{}{},
				}

				// 创建 MockUDPConnector
				mockConn, err := connector.New(ctx)
				So(err, ShouldBeNil)

				// 启动连接器
				err = mockConn.Start(&outputChan)
				So(err, ShouldBeNil)

				// 转换为 mock 连接器
				mockUDP, ok := mockConn.(*helpers.MockUDPConnector)
				So(ok, ShouldBeTrue)

				Convey("当注入UDP测试数据", func() {
					// 4. 发送测试数据
					testData := []byte{0xAA, 0x01, 0x02, 0xFF}
					err = mockUDP.InjectData(testData)
					So(err, ShouldBeNil)

					// 5. 等待处理结果
					var receivedData *pkg.PointPackage
					select {
					case receivedData = <-outputChan:
						// 成功接收到数据
					case <-time.After(3 * time.Second):
						So(true, ShouldBeFalse)
					}

					// 6. 验证结果
					Convey("应该正确解析UDP数据", func() {
						So(receivedData, ShouldNotBeNil)
						So(receivedData.Points, ShouldNotBeEmpty)

						if len(receivedData.Points) > 0 {
							point := receivedData.Points[0]
							So(point.Device, ShouldEqual, "udp_test_device")

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

// createTestConfig 创建测试配置
func createTestConfig(udpAddr, protoFilePath string) *pkg.Config {
	return &pkg.Config{
		Connector: pkg.ConnectorConfig{
			Type: "udp",
			Para: map[string]interface{}{
				"url":       udpAddr,
				"whiteList": false,
				"timeout":   "5s",
			},
		},
		Parser: pkg.ParserConfig{
			Type: "byte",
			Para: map[string]interface{}{
				"protoFile": "test_proto", // 使用内存中的协议定义
			},
		},
		Strategy: []pkg.StrategyConfig{
			{
				Type:   "memory_sink",
				Enable: true,
				Para:   map[string]interface{}{},
			},
		},
		Dispatcher: pkg.DispatcherConfig{
			// 对于测试，我们不需要设置任何重复数据过滤规则
			RepeatDataFilters: []pkg.DataFilter{},
		},
		// 添加内联协议定义
		Others: map[string]interface{}{
			"test_proto": map[string]interface{}{
				"chunks": []interface{}{
					map[string]interface{}{
						"type": "DefaultChunk",
						"desc": "测试UDP数据包",
						"sections": []interface{}{
							map[string]interface{}{
								"desc":      "UDP测试段",
								"repeat":    1,
								"size":      4,
								"to_device": "udp_test_device",
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
}

// sendUDPPacket 发送UDP数据包到指定地址
func sendUDPPacket(addr string, data []byte) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("解析UDP地址失败: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return fmt.Errorf("连接UDP地址失败: %v", err)
	}
	defer conn.Close()

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("写入UDP数据失败: %v", err)
	}

	return nil
}

// getMemorySink 从策略集合中获取MemorySink实例
// 更新返回类型和内部类型断言
func getMemorySink(t *testing.T, config *pkg.Config) (*helpers.MemorySink, bool) {
	logger := zaptest.NewLogger(t)
	ctx := pkg.WithLogger(context.Background(), logger)
	ctx = pkg.WithConfig(ctx, config)

	// 确保 memory_sink 已注册 (可能需要在测试设置中调用 helpers.RegisterMemorySink() 或类似函数)
	// 这里假设 sink.Factories 已经被填充
	factoryFunc, exists := sink.Factories["memory_sink"]
	if !exists {
		// 尝试注册（如果尚未完成）
		helpers.RegisterMemorySink()
		factoryFunc, exists = sink.Factories["memory_sink"]
		if !exists {
			t.Logf("MemorySink 工厂函数未注册")
			return nil, false
		}
	}

	template, err := factoryFunc(ctx)
	if err != nil {
		t.Logf("创建MemorySink失败: %v", err)
		return nil, false
	}

	// 更新类型断言
	memorySink, ok := template.(*helpers.MemorySink)
	if !ok {
		t.Logf("无法将模板转换为 helpers.MemorySink, 实际类型: %T", template)
		return nil, false
	}

	return memorySink, true
}

// NewContextWithLogger 辅助函数，用于创建带有Logger的上下文
func NewContextWithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return pkg.WithLogger(ctx, logger)
}

// NewContextWithConfig 辅助函数，用于创建带有配置的上下文
func NewContextWithConfig(ctx context.Context, config *pkg.Config) context.Context {
	return pkg.WithConfig(ctx, config)
}

// 初始化错误通道，确保正确的测试环境
func InitErrorChannel(ctx context.Context) context.Context {
	errChan := make(chan error, 10)
	return pkg.WithErrChan(ctx, errChan)
}

func TestMockUDPConnector(t *testing.T) {
	Convey("测试模拟UDP连接器", t, func() {
		// 设置测试环境
		logger := zaptest.NewLogger(t)

		// 创建上下文
		ctx, cancel := context.WithCancel(context.Background())
		ctx = NewContextWithLogger(ctx, logger)
		ctx = InitErrorChannel(ctx)

		// 创建配置 - 使用内联定义的协议，避免文件路径问题
		config := &pkg.Config{
			Parser: pkg.ParserConfig{
				Type: "byte", // 修改为正确的解析器类型
				Para: map[string]interface{}{
					"protoFile": "test_proto", // 指定使用的协议文件名
				},
			},
			Connector: pkg.ConnectorConfig{
				Type: "mock_udp", // 设置使用模拟UDP连接器
				Para: map[string]interface{}{},
			},
			Others: map[string]interface{}{
				"test_proto": map[string]interface{}{
					"chunks": []interface{}{
						map[string]interface{}{
							"type": "DefaultChunk",
							"desc": "测试数据包",
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
		ctx = NewContextWithConfig(ctx, config)

		Reset(func() {
			cancel() // 确保测试完成后取消上下文
		})

		// 注册模拟UDP连接器
		helpers.RegisterMockConnector()

		Convey("创建并启动模拟UDP连接器", func() {
			// 创建数据接收通道
			outputChan := make(pkg.Parser2DispatcherChan, 10)

			// 创建模拟UDP连接器
			mockConn, err := connector.New(ctx)
			So(err, ShouldBeNil)

			// 启动连接器
			err = mockConn.Start(&outputChan)
			So(err, ShouldBeNil)

			// 转换为mock连接器以便使用特定方法
			mockUDP, ok := mockConn.(*helpers.MockUDPConnector)
			So(ok, ShouldBeTrue)

			Convey("注入测试数据", func() {
				// 测试数据 - 使用0x01, 0x02, 0x03, 0x04字节
				// 实际解析结果为16909060 (0x01020304在小端系统中的整数表示)
				testData := []byte{0x01, 0x02, 0x03, 0x04}

				// 注入测试数据
				err = mockUDP.InjectData(testData)
				So(err, ShouldBeNil)

				// 等待处理完成
				var receivedData *pkg.PointPackage
				select {
				case receivedData = <-outputChan:
					// 成功接收到数据
				case <-time.After(2 * time.Second):
					So(true, ShouldBeFalse) // 报告超时
				}

				// 验证处理结果
				So(receivedData, ShouldNotBeNil)
				So(receivedData.Points, ShouldNotBeEmpty)

				// 验证解析出的值
				if len(receivedData.Points) > 0 && len(receivedData.Points[0].Field) > 0 {
					value, ok := receivedData.Points[0].Field["value"]
					So(ok, ShouldBeTrue)
					So(value, ShouldEqual, 16909060)
				}
			})
		})
	})
}

func TestMultipleUDPClients(t *testing.T) {
	Convey("测试多个UDP客户端", t, func() {
		// 设置测试环境
		logger := zaptest.NewLogger(t)

		// 创建上下文
		ctx, cancel := context.WithCancel(context.Background())
		ctx = NewContextWithLogger(ctx, logger)
		ctx = InitErrorChannel(ctx)

		// 创建配置 - 使用内联定义的协议，避免文件路径问题
		config := &pkg.Config{
			Parser: pkg.ParserConfig{
				Type: "byte", // 修改为正确的解析器类型
				Para: map[string]interface{}{
					"protoFile": "test_proto", // 指定使用的协议文件名
				},
			},
			Connector: pkg.ConnectorConfig{
				Type: "mock_udp", // 设置使用模拟UDP连接器
				Para: map[string]interface{}{},
			},
			Others: map[string]interface{}{
				"test_proto": map[string]interface{}{
					"chunks": []interface{}{
						map[string]interface{}{
							"type": "DefaultChunk",
							"desc": "测试数据包",
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
		ctx = NewContextWithConfig(ctx, config)

		Reset(func() {
			cancel() // 确保测试完成后取消上下文
		})

		// 注册模拟UDP连接器
		helpers.RegisterMockConnector()

		Convey("创建并启动模拟UDP连接器处理多客户端数据", func() {
			// 创建数据接收通道
			outputChan := make(pkg.Parser2DispatcherChan, 100)

			// 创建并启动多个模拟UDP连接器 - 由于每个测试用例重复使用相同的连接器
			// 我们这里模拟多个客户端通过单个连接器发送数据
			mockConn, err := connector.New(ctx)
			So(err, ShouldBeNil)

			err = mockConn.Start(&outputChan)
			So(err, ShouldBeNil)

			// 转换为mock连接器
			mockUDP, ok := mockConn.(*helpers.MockUDPConnector)
			So(ok, ShouldBeTrue)

			Convey("并发注入多个客户端的数据", func() {
				// 使用WaitGroup等待所有协程完成
				var wg sync.WaitGroup
				var errMutex sync.Mutex
				var injectErr error

				// 定义客户端数量
				const clientCount = 5

				// 并发注入数据
				for i := 0; i < clientCount; i++ {
					wg.Add(1)
					go func(idx int) {
						defer wg.Done()

						// 每个客户端发送不同的数据
						testData := []byte{byte(idx + 1), 0x02, 0x03, 0x04}

						err := mockUDP.InjectData(testData)
						if err != nil {
							errMutex.Lock()
							injectErr = err
							errMutex.Unlock()
						}
					}(i)
				}

				// 等待所有注入完成
				wg.Wait()

				// 检查是否有注入错误
				So(injectErr, ShouldBeNil)

				// 计数收到的数据包
				receivedCount := 0
				timeout := time.After(2 * time.Second)

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

// 创建并注册 MemorySink 实例
func createAndRegisterMemorySink(ctx context.Context) (*helpers.MemorySink, error) {
	// 调用工厂函数直接创建一个 MemorySink 实例
	sinkTemplate, err := helpers.NewMemorySink(ctx)
	if err != nil {
		return nil, fmt.Errorf("创建 MemorySink 实例失败: %v", err)
	}

	// 类型转换
	memorySink, ok := sinkTemplate.(*helpers.MemorySink)
	if !ok {
		return nil, fmt.Errorf("类型转换失败")
	}

	return memorySink, nil
}
