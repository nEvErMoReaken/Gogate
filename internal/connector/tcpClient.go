package connector

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"io"
	"net"
	"strings"
	"time"
)

// TcpClientConnector Connector的TcpClient版本实现
type TcpClientConnector struct {
	ctx          context.Context
	clientConfig *tcpClientConfig
}

func (t *TcpClientConnector) SinkType() string {
	return "stream"
}

func (t *TcpClientConnector) SetSink(source *pkg.DataSource) {

}

type tcpClientConfig struct {
	ServerAddrs    []string      `mapstructure:"serverAddrs"`    // 服务器地址列表
	Timeout        time.Duration `mapstructure:"timeout"`        // 超时时间
	ReconnectDelay time.Duration `mapstructure:"reconnectDelay"` // 重连间隔
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
	// 处理 timeout 字段
	if timeoutStr, ok := config.Connector.Para["reconnectDelay"].(string); ok {
		duration, err := time.ParseDuration(timeoutStr)
		if err != nil {
			pkg.LoggerFromContext(ctx).Error("解析超时配置失败", zap.Error(err))
			return nil, fmt.Errorf("解析超时配置失败: %s", err)
		}
		config.Connector.Para["reconnectDelay"] = duration
	}
	// 处理 serverAddrs 字段，支持字符串列表或逗号分隔的字符串
	var serverAddrs []string
	switch addrs := config.Connector.Para["serverAddrs"].(type) {
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
	config.Connector.Para["serverAddrs"] = serverAddrs

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
	}, nil
}

func (t *TcpClientConnector) GetType() string {
	return "stream"
}

// Start 方法启动客户端连接到服务器
func (t *TcpClientConnector) Start(sourceChan chan pkg.DataSource) error {

	for _, serverAddr := range t.clientConfig.ServerAddrs {
		ds := pkg.NewStreamDataSource()
		sourceChan <- ds
		// 对每个服务器地址启动一个协程
		go t.manageConnection(serverAddr, ds)
	}
	return nil
}

// manageConnection 管理对单个服务器的连接，包括重连逻辑
func (t *TcpClientConnector) manageConnection(serverAddr string, ds *pkg.StreamDataSource) {
	log := pkg.LoggerFromContext(t.ctx)
	for {
		// 检查全局关闭信号，避免不必要的重连尝试
		select {
		case <-t.ctx.Done():
			log.Info("收到停止信号，终止重连循环", zap.String("serverAddr", serverAddr))
			return
		default:
		}
		// 尝试连接到服务器
		conn, err := net.DialTimeout("tcp", serverAddr, t.clientConfig.ReconnectDelay)

		if err != nil {
			log.Warn("无法连接到服务器，5秒后重试", zap.String("serverAddr", serverAddr), zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		log.Info("成功连接到服务器", zap.String("serverAddr", serverAddr))
		// 在退出循环前确保关闭连接
		func() {
			defer func(conn net.Conn) {
				err := conn.Close()
				if err != nil {
					log.Error("关闭连接失败", zap.String("serverAddr", serverAddr), zap.Error(err))
				}
			}(conn)

			// 设置连接超时时间
			if err = conn.SetReadDeadline(time.Now().Add(t.clientConfig.Timeout)); err != nil {
				log.Error("设置连接超时失败", zap.String("serverAddr", serverAddr), zap.Error(err))
				return
			}

			buffer := make([]byte, 1024)
			for {
				select {
				case <-t.ctx.Done():
					log.Info("收到停止信号，关闭连接", zap.String("serverAddr", serverAddr))
					return
				default:
					// 从 TCP 连接读取数据
					n, err := conn.Read(buffer)
					if err != nil {
						if err == io.EOF {
							log.Info("服务器关闭连接", zap.String("serverAddr", serverAddr))
						} else {
							log.Error("读取数据失败", zap.Error(err))
						}
						return // 读取失败，退出以重连
					}

					// 将读取的数据写入到 Sink 的 writer 中
					if _, err := ds.WriteASAP(buffer[:n]); err != nil {
						log.Error("写入数据到 Sink 失败", zap.Error(err))
						return // 写入失败，退出以重连
					}
				}
			}
		}()
		// 等待再重连
		log.Info("正在尝试重连", zap.String("serverAddr", serverAddr))
		time.Sleep(t.clientConfig.ReconnectDelay)
	}
}
