package parser

import (
	"bytes"
	"context"
	"errors"
	"gateway/internal/pkg"
	. "github.com/smartystreets/goconvey/convey"
	"go.uber.org/zap"
	"io"
	"testing"
	"time"
)

// Mock Logger for capturing log outputs
var logger, _ = zap.NewDevelopment()

var commonCtx, cancel = context.WithCancel(pkg.WithErrChan(pkg.WithLogger(context.Background(), logger), make(chan error, 5)))

func TestExpandFieldTemplate(t *testing.T) {
	Convey("测试 expandFieldTemplate 函数", t, func() {
		Convey("正常展开模板", func() {
			template := "field{1..3}"
			result, err := expandFieldTemplate(template)
			So(err, ShouldBeNil)
			So(result, ShouldResemble, []string{"field1", "field2", "field3"})
		})

		Convey("错误模板：范围格式错误", func() {
			template := "field{1...3}" // 错误格式
			_, err := expandFieldTemplate(template)
			So(err, ShouldNotBeNil)
		})

		Convey("错误模板：范围无效", func() {
			template := "field{3..1}" // 起始值大于结束值
			_, err := expandFieldTemplate(template)
			So(err, ShouldNotBeNil)
		})
	})
}

func TestParseToDeviceName(t *testing.T) {
	Convey("测试 Section.parseToDeviceName 函数", t, func() {
		ctx := context.WithValue(context.Background(), "vobc_id", "123")

		Convey("正确解析模板变量", func() {
			section := Section{
				ToDeviceName: "ecc_${vobc_id}",
			}
			result, err := section.parseToDeviceName(ctx)
			So(err, ShouldBeNil)
			So(result, ShouldEqual, "ecc_123")
		})

		Convey("模板变量不存在", func() {
			section := Section{
				ToDeviceName: "ecc_${unknown_var}",
			}
			_, err := section.parseToDeviceName(ctx)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "未找到模板变量")
		})

		Convey("不包含模板变量的设备名称", func() {
			section := Section{
				ToDeviceName: "static_device",
			}
			result, err := section.parseToDeviceName(ctx)
			So(err, ShouldBeNil)
			So(result, ShouldEqual, "static_device")
		})
	})
}

func TestGetIntVar(t *testing.T) {
	Convey("测试 getIntVar 函数", t, func() {
		ctx := context.WithValue(context.Background(), "test_key", 42)

		Convey("直接返回整型值", func() {
			value, err := getIntVar(ctx, 100)
			So(err, ShouldBeNil)
			So(value, ShouldEqual, 100)
		})

		Convey("从 context 中获取变量", func() {
			value, err := getIntVar(ctx, "test_key")
			So(err, ShouldBeNil)
			So(value, ShouldEqual, 42)
		})

		Convey("变量不存在", func() {
			_, err := getIntVar(ctx, "missing_key")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "未找到变量")
		})

		Convey("变量类型错误", func() {
			ctx := context.WithValue(context.Background(), "string_key", "not_int")
			_, err := getIntVar(ctx, "string_key")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "类型错误")
		})
	})
}

func TestCreateChunk(t *testing.T) {
	Convey("测试 createChunk 函数", t, func() {
		Convey("正确解析 FixedLengthChunk 配置", func() {
			chunkMap := map[string]interface{}{
				"type":   "FixedLengthChunk",
				"length": 10,
				"sections": []map[string]interface{}{
					{
						"from": map[string]interface{}{
							"repeat": 1,
							"byte":   4,
						},
						"to": map[string]interface{}{
							"fields": []string{"field1", "field2"},
						},
					},
				},
			}
			chunk, err := createChunk(chunkMap)
			So(err, ShouldBeNil)
			So(chunk, ShouldNotBeNil)
		})

		Convey("错误的 chunk 类型", func() {
			chunkMap := map[string]interface{}{
				"type": "UnknownChunkType",
			}
			_, err := createChunk(chunkMap)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "unknown chunk type")
		})
	})
}

func TestNewIoReader(t *testing.T) {
	Convey("测试 NewIoReader 方法", t, func() {
		Convey("成功初始化 IoReader", func() {
			// 模拟有效的上下文配置
			ctx := pkg.WithConfig(context.Background(), &pkg.Config{
				Parser: pkg.ParserConfig{
					Para: map[string]interface{}{
						"ProtoFile": "test_proto",
					},
				},
				Others: map[string]interface{}{
					"test_proto": map[string]interface{}{
						"chunks": []interface{}{
							map[string]interface{}{
								"type":   "FixedLengthChunk",
								"length": 10,
							},
						},
					},
				},
			})

			ioReader, err := NewIoReader(ctx)
			So(err, ShouldBeNil)
			So(ioReader, ShouldNotBeNil)
		})

		Convey("协议文件不存在", func() {
			// 模拟上下文中没有指定的协议文件
			ctx := pkg.WithConfig(context.Background(), &pkg.Config{
				Parser: pkg.ParserConfig{
					Para: map[string]interface{}{
						"ProtoFile": "non_existent_proto",
					},
				},
				Others: map[string]interface{}{},
			})

			ioReader, err := NewIoReader(ctx)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "未找到协议文件")
			So(ioReader, ShouldBeNil)
		})

		Convey("协议文件格式错误", func() {
			// 模拟协议文件格式错误
			ctx := pkg.WithConfig(context.Background(), &pkg.Config{
				Parser: pkg.ParserConfig{
					Para: map[string]interface{}{
						"ProtoFile": "test_proto",
					},
				},
				Others: map[string]interface{}{
					"test_proto": "invalid_format",
				},
			})

			ioReader, err := NewIoReader(ctx)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "协议文件格式错误")
			So(ioReader, ShouldBeNil)
		})

		Convey("createIoParser 初始化失败", func() {
			// 模拟 createIoParser 返回错误
			ctx := pkg.WithConfig(context.Background(), &pkg.Config{
				Parser: pkg.ParserConfig{
					Para: map[string]interface{}{
						"ProtoFile": "test_proto",
					},
				},
				Others: map[string]interface{}{
					"test_proto": map[string]interface{}{
						"chunks": []interface{}{
							map[string]interface{}{
								"type": "UnknownChunkType", // 不支持的类型
							},
						},
					},
				},
			})

			ioReader, err := NewIoReader(ctx)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "初始化IoReader失败")
			So(ioReader, ShouldBeNil)
		})
	})
}

func TestFixedLengthChunk_Process(t *testing.T) {
	Convey("测试 FixedLengthChunk.Process 方法", t, func() {
		// 模拟日志上下文
		ctx := context.WithValue(context.Background(), "logger", zap.NewExample())

		Convey("处理成功 - 正常数据", func() {
			// 模拟一个简单的 FixedLengthChunk
			chunk := &FixedLengthChunk{
				length: 6, // 数据块长度
				Sections: []Section{
					{
						Repeat: 1,
						Length: 3,
						Decoding: func(data []byte) ([]interface{}, error) {
							return []interface{}{int(data[0]), int(data[1]), int(data[2])}, nil
						},
						ToVarNames:   []string{"var1", "var2", "var3"},
						ToDeviceType: "deviceType",
						ToDeviceName: "deviceName",
						ToFieldNames: []string{"field1", "field2", "field3"},
					},
				},
			}

			// 使用 bytes.Buffer 模拟 Reader
			var buf bytes.Buffer
			data := []byte{1, 2, 3, 4, 5, 6}
			// 将数据写入 buf
			buf.Write(data)

			// 创建 StreamDataSource，并指定 Reader 为 buf
			dataSource := &pkg.StreamDataSource{
				MetaData: map[string]string{"exampleMeta": "data"},
				Reader:   &buf, // 这里是Reader，数据从这里读取
				Writer:   nil,
			}

			// 模拟帧和快照集合
			frame := make([]byte, 0)
			handler := &SnapshotCollection{}

			// 执行 Process
			changedCtx, err := chunk.Process(ctx, dataSource, &frame, handler)

			So(err, ShouldBeNil)
			So(frame, ShouldResemble, []byte{1, 2, 3, 4, 5, 6})
			So(changedCtx.Value("var1"), ShouldEqual, 1)
			So(changedCtx.Value("var2"), ShouldEqual, 2)
			So(changedCtx.Value("var3"), ShouldEqual, 3)
		})

		Convey("解码器错误", func() {
			chunk := &FixedLengthChunk{
				length: 3,
				Sections: []Section{
					{
						Repeat: 1,
						Length: 3,
						Decoding: func(data []byte) ([]interface{}, error) {
							return nil, errors.New("解码失败")
						},
					},
				},
			}

			// 使用 bytes.Buffer 模拟 Reader
			var buf bytes.Buffer
			data := []byte{1, 2, 3}
			// 将数据写入 buf
			buf.Write(data)

			// 创建 StreamDataSource，并指定 Reader 为 buf
			dataSource := &pkg.StreamDataSource{
				MetaData: map[string]string{"exampleMeta": "data"},
				Reader:   &buf, // 这里是Reader，数据从这里读取
				Writer:   nil,
			}

			frame := make([]byte, 0)
			handler := &SnapshotCollection{}

			_, err := chunk.Process(ctx, dataSource, &frame, handler)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "解码失败")
		})

	})
}

// 模拟的 MockReader 实现
type MockReader struct {
	data []byte
}

func (r *MockReader) Read(p []byte) (n int, err error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	copy(p, r.data)
	n = len(r.data)
	r.data = nil // 只读一次数据，模拟 EOF
	return
}

// 模拟 IoReader.Start 方法
func TestIoReader_Start(t *testing.T) {
	Convey("测试 IoReader.Start 方法", t, func() {
		// 模拟日志上下文
		ctx := pkg.WithErrChan(context.Background(), make(chan error, 5))

		// 模拟一个 IoReader
		ioReader := &IoReader{
			ctx:                ctx,
			Chunks:             nil, // 使用 chunk
			SnapshotCollection: SnapshotCollection{},
		}

		// 模拟 DataSource 和 SinkMap
		dataSource := &pkg.StreamDataSource{
			MetaData: map[string]string{"exampleMeta": "data"},
			Reader:   nil, // 可以用模拟的 Reader
			Writer:   nil, // 这里不使用 Writer
		}

		sinkMap := &pkg.PointDataSource{}

		Convey("正常处理数据", func() {
			// 模拟数据源的 Reader
			dataSource.Reader = &MockReader{
				data: []byte{1, 2, 3, 4, 5, 6},
			}

			// 模拟 Chunk 的 Process 方法
			ioReader.Chunks = []Chunk{ // 使用 Chunk 接口类型的切片
				&FixedLengthChunk{ // 使用指针类型，因为我们在接口中通常使用指针接收者
					length: 6,
					Sections: []Section{
						{
							Repeat: 1,
							Length: 3,
							Decoding: func(data []byte) ([]interface{}, error) {
								// 假设解码返回这些数据
								return []interface{}{int(data[0]), int(data[1]), int(data[2])}, nil
							},
							ToDeviceName: "device1",
							ToDeviceType: "type1",
							ToFieldNames: []string{"field1", "field2", "field3"},
						},
					},
				},
			}
			var ds pkg.DataSource = dataSource
			// 让 Start 方法执行一次
			go ioReader.Start(&ds, sinkMap)
			deviceSnapshot, err := ioReader.SnapshotCollection.GetDeviceSnapshot("device1", "type1")
			So(err, ShouldBeNil)
			So(deviceSnapshot, ShouldNotBeNil)
			So(deviceSnapshot.DeviceName, ShouldEqual, "device1")
			So(deviceSnapshot.DeviceType, ShouldEqual, "type1")
			So(deviceSnapshot.Fields, ShouldNotBeNil)
		})

		Convey("读取到 EOF 后退出", func() {
			// 模拟数据源读取到 EOF
			dataSource.Reader = &MockReader{
				data: nil, // 模拟 EOF
			}
			var ds pkg.DataSource = dataSource

			// 让 Start 方法执行一次
			go ioReader.Start(&ds, sinkMap)

			// 等待一段时间，确保方法执行完毕
			time.Sleep(1 * time.Second)

		})

		Convey("解析 Chunk 失败", func() {
			// 模拟一个解码失败的 Chunk
			ioReader.Chunks = []Chunk{ // 使用 Chunk 接口类型的切片
				&FixedLengthChunk{ // 使用指针类型，因为我们在接口中通常使用指针接收者
					length: 6,
					Sections: []Section{
						{
							Repeat: 1,
							Length: 3,
							Decoding: func(data []byte) ([]interface{}, error) {
								return nil, errors.New("解码失败") // 返回解码错误
							},
						},
					},
				},
			}
			// 模拟数据源的 Reader
			dataSource.Reader = &MockReader{
				data: []byte{1, 2, 3, 4, 5, 6},
			}
			var ds pkg.DataSource = dataSource
			// 让 Start 方法执行一次
			go ioReader.Start(&ds, sinkMap)

			// 等待一段时间，确保方法执行完毕
			time.Sleep(1 * time.Second)

			// 断言通过错误通道可以接收到解析失败的消息
			select {
			case err := <-pkg.ErrChanFromContext(ioReader.ctx):
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "解析第 1 个 Chunk 失败: 解码失败")
			case <-time.After(2 * time.Second):
				So("timeout", ShouldEqual, "should not reach here")
			}
		})
	})
}
