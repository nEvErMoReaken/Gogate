package connector

import (
	"context"
	"errors"
	"fmt"
	"gateway/internal/pkg"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"net"
	"time"
)

// UdpClientConfig 包含 UDP 客户端的配置信息
type UdpClientConfig struct {
	ServerAddr string        `mapstructure:"serverAddr"` // 服务器地址（例如 "localhost:8080"）
	Timeout    time.Duration `mapstructure:"timeout"`    // 接收数据的超时时间
	BufferSize int           `mapstructure:"bufferSize"` // 缓冲区大小
}

// UdpClient 实现与服务器接口一致的 UDP 客户端
type UdpClient struct {
	ctx      context.Context
	config   *UdpClientConfig
	dataChan chan string // 数据通道，用于接收到的数据
	conn     *net.UDPConn
}

// init 注册 UDP 客户端
func init() {
	Register("udp", NewUdpClient)
}

// NewUdpClient 创建一个新的 UdpClient 实例
func NewUdpClient(ctx context.Context) (Connector, error) {
	config := pkg.ConfigFromContext(ctx) // 从上下文中获取配置

	// 解析配置
	var udpClientConfig UdpClientConfig
	if err := mapstructure.Decode(config.Connector.Para, &udpClientConfig); err != nil {
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}

	serverAddr, err := net.ResolveUDPAddr("udp", udpClientConfig.ServerAddr)
	if err != nil {
		return nil, fmt.Errorf("无法解析服务器地址: %v", err)
	}

	// 创建 UDP 连接
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return nil, fmt.Errorf("无法连接到服务器: %v", err)
	}

	return &UdpClient{
		ctx:      ctx,
		config:   &udpClientConfig,
		dataChan: make(chan string, 200),
		conn:     conn,
	}, nil
}

// Start 启动客户端，不断监听服务器的数据
func (c *UdpClient) Start() {
	buffer := make([]byte, c.config.BufferSize)
	log := pkg.LoggerFromContext(c.ctx)
	for {
		select {
		case <-c.ctx.Done():
			log.Info("UDP 客户端正在关闭...")
			err := c.Close()
			if err != nil {
				log.Error("UDP 客户端关闭失败", zap.Error(err))
			}
			return
		default:
			// 设置读取超时
			err := c.conn.SetReadDeadline(time.Now().Add(c.config.Timeout))
			if err != nil {
				log.Error("设置读取超时失败", zap.Error(err))
				continue
			}

			// 接收服务器发送的数据
			n, _, err := c.conn.ReadFromUDP(buffer)
			if err != nil {
				var opErr *net.OpError
				if errors.As(err, &opErr) && opErr.Timeout() {
					continue // 超时则继续等待
				}
				log.Error("接收数据失败", zap.Error(err))
				return
			}

			// 将接收到的数据发送到 dataChan
			receivedData := string(buffer[:n])
			log.Info("接收到数据", zap.String("data", receivedData))
			c.dataChan <- receivedData
		}
	}
}

// Ready 返回数据通道（与服务器接口一致）
func (c *UdpClient) Ready() chan pkg.DataSource {
	return nil
}

// GetDataSource 提供数据源
func (c *UdpClient) GetDataSource() (pkg.DataSource, error) {
	return pkg.DataSource{
		Source:   c.dataChan,
		MetaData: nil,
	}, nil
}

// Close 关闭客户端连接
func (c *UdpClient) Close() error {
	fmt.Println("UDP 客户端连接正在关闭...")
	err := c.conn.Close()
	if err != nil {
		return fmt.Errorf("关闭 UDP 连接失败: %v", err)
	}
	close(c.dataChan)
	fmt.Println("UDP 客户端连接已关闭")
	return nil
}
