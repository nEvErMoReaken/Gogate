package connector_test

import (
	"bufio"
	"context"
	"gateway/internal/pkg"
	"github.com/stretchr/testify/assert"
	"net"
	"sync"
	"testing"
	"time"
)

// 测试 TcpServerConnector 的初始化
func TestNewTcpServer(t *testing.T) {
	ctx := context.Background()

	// 模拟配置
	config := &pkg.Config{
		Connector: pkg.ConnectorConfig{
			Para: map[string]interface{}{
				"port":      "9992",
				"timeout":   "5s",
				"whiteList": false,
			},
		},
	}
	ctx = pkg.WithConfig(ctx, config)

	connector, err := NewTcpServer(ctx)

	assert.NoError(t, err, "初始化 TcpServerConnector 不应出错")
	assert.NotNil(t, connector, "TcpServerConnector 不应为 nil")
}

func TestTcpServerStart(t *testing.T) {
	ctx := context.Background()

	// 模拟配置
	config := &pkg.Config{
		Connector: pkg.ConnectorConfig{
			Para: map[string]interface{}{
				"port":      "12222",
				"timeout":   "5s",
				"whiteList": false,
			},
		},
	}
	ctx = pkg.WithConfig(ctx, config)
	ctx = pkg.WithErrChan(ctx, make(chan error))
	ctx = pkg.WithLogger(ctx, logger) // 你的 logger 定义需要在外部初始化
	// 启动 TcpServerConnector
	connector, err := NewTcpServer(ctx)
	assert.NoError(t, err, "初始化 TcpServerConnector 不应出错")
	tcpServer := connector.(*TcpServerConnector)

	// 启动服务器，使用 WaitGroup 来确保所有协程同步执行
	var wg sync.WaitGroup
	wg.Add(1)

	// 启动 TcpServerConnector
	go func() {
		defer wg.Done()
		tcpServer.Start()
	}()

	// 模拟客户端连接
	time.Sleep(1 * time.Second) // 确保服务器已启动
	conn, err := net.Dial("tcp", "localhost:12222")
	assert.NoError(t, err, "客户端应成功连接到服务器")
	time.Sleep(1 * time.Second) // 确保通道已经返回

	// 检查是否有 DataSource 准备好
	select {
	case dataSource := <-tcpServer.Ready():
		assert.NotNil(t, dataSource.Source, "DataSource 的 Source 不应为 nil")
	case err = <-pkg.ErrChanFromContext(ctx):
		t.Fatal(err) // 如果有错误，记录错误信息并终止测试
	case <-time.After(3 * time.Second): // 超时处理
		t.Fatal("服务器未能处理客户端连接")
	}

	// 关闭客户端连接和服务器
	err = conn.Close()
	if err != nil {
		t.Fatalf("关闭客户端连接失败: %v", err)
	}

	// 确保服务器正常关闭
	_ = tcpServer.Close()
	wg.Wait() // 等待服务器关闭
}

// 测试 handleConnection 处理逻辑
func TestInitConn(t *testing.T) {
	ctx := context.Background()

	// 模拟配置
	config := &pkg.Config{
		Connector: pkg.ConnectorConfig{
			Para: map[string]interface{}{
				"port":      "9923",
				"timeout":   "5s",
				"whiteList": false,
				"ipAlias": map[string]string{
					"127.0.0.1": "test-device",
					"::1":       "test-device", // IPv6 别名
				},
			},
		},
	}
	ctx = pkg.WithConfig(ctx, config)

	connector, err := NewTcpServer(ctx)
	assert.NoError(t, err, "初始化 TcpServerConnector 不应出错")
	tcpServer := connector.(*TcpServerConnector)

	// 模拟服务器监听和客户端连接
	listener, err := net.Listen("tcp", ":9999")
	assert.NoError(t, err, "监听端口不应出错")

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		conn, err := listener.Accept()
		assert.NoError(t, err, "服务器应成功接受连接")
		defer conn.Close()

		dataSource, err := tcpServer.handleConnection(conn)
		assert.NoError(t, err, "handleConnection 不应出错")
		assert.Equal(t, "test-device", dataSource.MetaData["deviceId"], "设备 ID 应为 'test-device'")
		assert.NotNil(t, dataSource.Source, "DataSource 的 Source 不应为 nil")

		reader := bufio.NewReader(conn)
		assert.Equal(t, reader, dataSource.Source, "Reader 应成功初始化")
	}()

	// 模拟客户端连接
	conn, err := net.Dial("tcp", "localhost:9999")
	assert.NoError(t, err, "客户端应成功连接到服务器")

	// 关闭客户端连接
	conn.Close()
	listener.Close()

	// 等待所有 goroutine 完成
	wg.Wait()
}

// 测试 TcpServerConnector 的 Close 行为
func TestTcpServerClose(t *testing.T) {
	ctx := context.Background()

	// 模拟配置
	config := &pkg.Config{
		Connector: pkg.ConnectorConfig{
			Para: map[string]interface{}{
				"port":      "9996",
				"timeout":   "5s",
				"whiteList": false,
			},
		},
	}
	ctx = pkg.WithConfig(ctx, config)

	// 启动 TcpServerConnector
	connector, err := NewTcpServer(ctx)
	assert.NoError(t, err, "初始化 TcpServerConnector 不应出错")
	tcpServer := connector.(*TcpServerConnector)

	// 启动服务器
	go tcpServer.Start()

	// 关闭服务器
	err = tcpServer.Close()
	assert.NoError(t, err, "关闭 TcpServerConnector 不应出错")
}
