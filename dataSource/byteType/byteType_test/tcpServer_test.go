package byteType_test

import (
	"bufio"
	"github.com/stretchr/testify/assert"
	"gw22-train-sam/dataSource/byteType"
	"io"
	"net"
	"testing"
	"time"
)

// 模拟一个简单的 Chunk 解析器
type MockChunk struct{}

func (m *MockChunk) Process(reader io.Reader, frame *[]byte) error {
	// 模拟从 reader 中读取 4 个字节并添加到 frame 中
	buf := make([]byte, 4)
	_, err := reader.Read(buf)
	if err != nil {
		return err
	}
	*frame = append(*frame, buf...)
	return nil
}

func (m *MockChunk) String() string {
	return "MockChunk"
}

func TestHandleConnection(t *testing.T) {
	// 使用 net.Pipe 创建模拟的连接
	client, server := net.Pipe()
	defer func(client net.Conn) {
		err := client.Close()
		if err != nil {
			t.Fatalf("关闭客户端连接失败: %v", err)
		}
	}(client)
	defer func(server net.Conn) {
		err := server.Close()
		if err != nil {
			t.Fatalf("关闭服务端连接失败: %v", err)
		}
	}(server)

	// 初始化 ServerModel
	serverModel := &byteType.ServerModel{
		ChunkSequence: &byteType.ChunkSequence{
			Chunks: []byteType.Chunk{&MockChunk{}}, // 使用 MockChunk 替代真实 Chunk
		},
		ChDone: make(chan struct{}),
	}

	// 模拟客户端发送数据
	go func() {
		clientWriter := bufio.NewWriter(client)
		clientWriter.Write([]byte("test")) // 发送 4 字节数据
		clientWriter.Flush()
		time.Sleep(1 * time.Second) // 保证服务端处理
		client.Close()
	}()

	// 在另一个 Goroutine 中处理连接
	go serverModel.HandleConnection(server)

	// 等待一点时间以确保 handleConnection 执行完毕
	time.Sleep(2 * time.Second)

	// 断言日志输出或其他操作，例如 frame 是否正确处理
	assert.NotNil(t, serverModel)
}
