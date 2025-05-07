package connector

import (
	"context"
	"fmt"
	"gateway/internal/parser"
	"gateway/internal/pkg"
	"net"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

// TcpClientConnector Connector的TcpClient版本实现
type TcpClientConnector struct {
	ctx          context.Context
	clientConfig *tcpClientConfig
	done         chan struct{}
}

type tcpClientConfig struct {
	ServerAddrs    []string      `mapstructure:"serverAddrs"`    // 服务器地址列表
	Timeout        time.Duration `mapstructure:"timeout"`        // 超时时间
	ReconnectDelay time.Duration `mapstructure:"reconnectDelay"` // 重连间隔
	BufferSize     int           `mapstructure:"bufferSize"`     // 环形缓冲区大小
}

// init 函数注册 TcpClientConnector
func init() {
	Register("tcpclient", NewTcpClient)
}

// NewTcpClient 函数创建并初始化 TcpClientConnector
func NewTcpClient(ctx context.Context) (Template, error) {
	// 获取配置
	config := pkg.ConfigFromContext(ctx)

	// 处理 timeout 字段
	if timeoutStr, ok := config.Connector.Para["timeout"].(string); ok {
		duration, err := time.ParseDuration(timeoutStr)
		if err != nil {
			pkg.LoggerFromContext(ctx).Error("解析超时配置失败", zap.Error(err))
			return nil, fmt.Errorf("解析超时配置失败: %s", err)
		}
		config.Connector.Para["timeout"] = duration
	}
	// 处理 reconnectdelay 字段
	if reconnectDelayStr, ok := config.Connector.Para["reconnectdelay"].(string); ok {
		duration, err := time.ParseDuration(reconnectDelayStr)
		if err != nil {
			pkg.LoggerFromContext(ctx).Error("解析重连间隔配置失败", zap.Error(err))
			return nil, fmt.Errorf("解析重连间隔配置失败: %s", err)
		}
		config.Connector.Para["reconnectdelay"] = duration
	}
	// 处理 serverAddrs 字段，支持字符串列表或逗号分隔的字符串
	var serverAddrs []string
	switch addrs := config.Connector.Para["serveraddrs"].(type) {
	case []interface{}:
		for _, addr := range addrs {
			if addrStr, ok := addr.(string); ok {
				serverAddrs = append(serverAddrs, addrStr)
			}
		}
	case string:
		serverAddrs = strings.Split(addrs, ",")
	default:
		pkg.LoggerFromContext(ctx).Error("解析服务器地址列表失败")
		return nil, fmt.Errorf("解析服务器地址列表失败")
	}
	config.Connector.Para["serveraddrs"] = serverAddrs

	// 初始化配置结构
	var clientConfig tcpClientConfig
	err := mapstructure.Decode(config.Connector.Para, &clientConfig)
	if err != nil {
		pkg.LoggerFromContext(ctx).Error("配置文件解析失败", zap.Error(err))
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}

	// 初始化并返回 TcpClientConnector
	return &TcpClientConnector{
		ctx:          ctx,
		clientConfig: &clientConfig,
		done:         make(chan struct{}),
	}, nil
}

// Start 方法启动客户端连接到服务器
func (t *TcpClientConnector) Start(sink *pkg.Parser2DispatcherChan) error {
	log := pkg.LoggerFromContext(t.ctx)
	log.Info("===正在启动Connector: TcpClient===")
	for _, serverAddr := range t.clientConfig.ServerAddrs {
		// 对每个服务器地址启动一个协程
		go func(addr string) {
			select {
			case <-t.done:
				return
			default:
				err := t.handleConnection(addr, *sink)
				if err != nil {
					pkg.LoggerFromContext(t.ctx).Error("处理连接失败", zap.Error(err))
				}
			}
		}(serverAddr)
	}
	return nil
}

// handleConnection 处理对单个服务器的连接，包括重连逻辑
func (t *TcpClientConnector) handleConnection(serverAddr string, sink pkg.Parser2DispatcherChan) error {
	log := pkg.LoggerFromContext(t.ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	for {
		// 1. 尝试连接到服务器
		conn, err := net.DialTimeout("tcp", serverAddr, t.clientConfig.Timeout)
		if err != nil {
			metrics.IncErrorCount()
			metrics.IncMsgErrors("tcpclient_connect")
			log.Warn(fmt.Sprintf("无法连接到服务器，%s 后重试", t.clientConfig.ReconnectDelay), zap.String("serverAddr", serverAddr), zap.Error(err))
			time.Sleep(t.clientConfig.ReconnectDelay)
			continue
		}

		// 2. 设置连接超时时间
		if err = conn.SetReadDeadline(time.Now().Add(t.clientConfig.Timeout)); err != nil {
			metrics.IncErrorCount()
			metrics.IncMsgErrors("tcpclient_timeout")
			log.Error("设置连接超时失败", zap.String("serverAddr", serverAddr), zap.Error(err))
			conn.Close()
			time.Sleep(t.clientConfig.ReconnectDelay)
			continue
		}

		log.Info("成功连接到服务器", zap.String("serverAddr", serverAddr))

		// 3. 创建环形缓冲区
		ringBuffer, err := pkg.NewRingBuffer(conn, uint32(t.clientConfig.BufferSize))
		if err != nil {
			log.Error("创建环形缓冲区失败", zap.Error(err))
			conn.Close()
			time.Sleep(t.clientConfig.ReconnectDelay)
			continue
		}

		// 4. 创建字节解析器
		byteParser, err := parser.NewByteParser(t.ctx)
		if err != nil {
			log.Error("创建字节解析器失败", zap.Error(err))
			conn.Close()
			time.Sleep(t.clientConfig.ReconnectDelay)
			continue
		}

		// 5. 启动解析器处理数据, 该方法会阻塞，直到连接断开
		err = byteParser.StartWithRingBuffer(ringBuffer, sink)
		if err != nil {
			log.Error("启动字节解析器失败", zap.Error(err))
			conn.Close()
			time.Sleep(t.clientConfig.ReconnectDelay)
			continue
		}
		// 6. 只有.done() 方法被调用时，才会退出循环
		return nil
	}
}

// Done 手动关闭连接器
func (t *TcpClientConnector) Done() {
	close(t.done)
}
