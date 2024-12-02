package connector_test

import (
	"context"
	"gateway/internal/pkg"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"net"
	"sync"
	"testing"
	"time"
)

// Mock Logger for capturing log outputs
var logger, _ = zap.NewDevelopment()

// 测试成功连接到服务器
func TestTcpClientStart(t *testing.T) {
	ctx := context.Background()

	// 创建模拟配置
	config := &pkg.Config{
		Connector: pkg.ConnectorConfig{
			Para: map[string]interface{}{
				"serverAddr": "localhost:12223",
				"timeout":    "5s",
			},
		},
	}
	ctx = pkg.WithConfig(ctx, config)
	ctx = pkg.WithErrChan(ctx, make(chan error))
	ctx = pkg.WithLogger(ctx, logger) // 请确保在外部初始化你的 logger

	// 初始化 TcpClientConnector
	connector, err := NewTcpClient(ctx)
	assert.NoError(t, err, "初始化 TcpClientConnector 不应出错")
	tcpClient := connector.(*TcpClientConnector)

	// 启动一个本地 TCP 服务器，模拟远程服务
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		listener, err := net.Listen("tcp", "localhost:12223")
		if err != nil {
			t.Fatalf("无法启动模拟服务器: %v", err)
		}
		defer listener.Close()

		// 等待客户端连接
		conn, err := listener.Accept()
		assert.NoError(t, err, "服务器应成功接受客户端连接")
		defer conn.Close()
	}()

	// 后启动 TcpClientConnector， 确保一启动就有数据可以进行验证
	go tcpClient.Start()

	// 确认客户端连接成功，等待数据源准备就绪
	select {
	case dataSource := <-tcpClient.Ready():
		assert.NotNil(t, dataSource.Source, "DataSource 的 Source 不应为 nil")
	case err = <-pkg.ErrChanFromContext(ctx):
		t.Fatal(err) // 如果有错误，记录错误信息并终止测试
	case <-time.After(3 * time.Second): // 超时处理
		t.Fatal("客户端未能连接到服务器")
	}

	// 确保客户端关闭连接
	err = tcpClient.Close()
	assert.NoError(t, err, "关闭客户端连接不应出错")

	wg.Wait() // 等待服务器协程结束
}

func TestTcpClientDelayedServerStart(t *testing.T) {
	ctx := context.Background()

	// 创建模拟配置
	config := &pkg.Config{
		Connector: pkg.ConnectorConfig{
			Para: map[string]interface{}{
				"serverAddr": "localhost:12224", // 使用动态端口
				"timeout":    "2s",
			},
		},
	}
	ctx = pkg.WithConfig(ctx, config)
	ctx = pkg.WithErrChan(ctx, make(chan error))
	ctx = pkg.WithLogger(ctx, logger) // 请确保在外部初始化你的 logger

	// 初始化 TcpClientConnector
	connector, err := NewTcpClient(ctx)
	assert.NoError(t, err, "初始化 TcpClientConnector 不应出错")
	tcpClient := connector.(*TcpClientConnector)

	// 启动 TcpClientConnector，立即尝试连接
	go tcpClient.Start()

	// 延迟启动服务器
	time.Sleep(3 * time.Second) // 延迟 3 秒以触发客户端的重连逻辑

	// 启动本地 TCP 服务器，绑定动态端口
	listener, err := net.Listen("tcp", "localhost:12224")
	assert.NoError(t, err, "无法启动模拟服务器")
	defer listener.Close()listener.Close()

	// 获取动态分配的端口并更新到客户端配置
	addr := listener.Addr().String()
	config.Connector.Para["serverAddr"] = addr

	// 模拟服务器接受客户端连接
	reconnected := make(chan struct{}) // 用于确认重连成功
	go func() {
		conn, err := listener.Accept() // 等待客户端连接
		assert.NoError(t, err, "服务器应成功接受客户端重连请求")
		defer conn.Close()

		reconnected <- struct{}{}
	}()

	// 等待重连成功
	select {
	case <-reconnected:
		t.Log("客户端成功重连到服务器")
	case <-time.After(10 * time.Second): // 超时检查
		t.Fatal("客户端未能在预期时间内重连到服务器")
	}

	// 关闭客户端连接
	err = tcpClient.Close()
	assert.NoError(t, err, "关闭客户端连接不应出错")
}
