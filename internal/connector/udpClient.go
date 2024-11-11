package connector

import (
	"context"
	"errors"
	"fmt"
	"gateway/internal/pkg"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"net"
	"strings"
	"time"
)

// UdpClientConnector 是 UDP 客户端版本的 Connector 实现
type UdpClientConnector struct {
	ctx          context.Context
	clientConfig *UdpClientConfig
	Sink         pkg.StreamDataSource
}

func (u *UdpClientConnector) SinkType() string {
	return "stream"
}

func (u *UdpClientConnector) SetSink(source *pkg.DataSource) {
	if sink, ok := (*source).(*pkg.StreamDataSource); ok {
		u.Sink = *sink
	} else {
		pkg.LoggerFromContext(u.ctx).Error("UDP 数据源类型错误, 期望 pkg.StreamDataSource")
	}
}

type UdpClientConfig struct {
	ServerAddrs []string      `mapstructure:"serverAddrs"` // 服务器地址列表
	Timeout     time.Duration `mapstructure:"timeout"`     // 超时时间
}

func init() {
	Register("udpclient", NewUdpClient)
}

// NewUdpClient 创建并初始化 UdpClientConnector
func NewUdpClient(ctx context.Context) (Connector, error) {
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

	// 处理 serverAddrs 字段
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
	var clientConfig UdpClientConfig
	err := mapstructure.Decode(config.Connector.Para, &clientConfig)
	if err != nil {
		pkg.LoggerFromContext(ctx).Error("配置文件解析失败", zap.Error(err))
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}

	// 初始化并返回 UdpClientConnector
	return &UdpClientConnector{
		ctx:          ctx,
		clientConfig: &clientConfig,
	}, nil
}

// Start 方法启动客户端监听服务器的数据
func (u *UdpClientConnector) Start() {
	for _, serverAddr := range u.clientConfig.ServerAddrs {
		// 为每个服务器地址启动一个协程来管理连接
		go u.manageConnection(serverAddr)
	}
}

// manageConnection 管理对单个服务器的连接，包括重连和数据接收逻辑
func (u *UdpClientConnector) manageConnection(serverAddr string) {
	log := pkg.LoggerFromContext(u.ctx)

	for {
		// 检查关闭信号，避免不必要的重连
		select {
		case <-u.ctx.Done():
			log.Info("收到停止信号，终止重连循环", zap.String("serverAddr", serverAddr))
			return
		default:
		}

		// 解析 UDP 地址
		_, err := net.ResolveUDPAddr("udp", serverAddr)
		if err != nil {
			log.Error("无法解析 UDP 地址", zap.String("serverAddr", serverAddr), zap.Error(err))
			pkg.ErrChanFromContext(u.ctx) <- fmt.Errorf("panic: 无法解析 UDP 地址: %v, addr: %v", err, serverAddr)
			return
		}

		// 创建 UDP 连接
		conn, err := net.ListenUDP("udp", nil)
		if err != nil {
			log.Warn("无法创建 UDP 连接，5秒后重试", zap.String("serverAddr", serverAddr), zap.Error(err))
			time.Sleep(u.clientConfig.Timeout)
			continue
		}

		log.Info("UDP 连接已建立", zap.String("serverAddr", serverAddr))

		// 管理连接的生命周期
		func() {
			defer func() {
				if err := conn.Close(); err != nil {
					log.Error("关闭 UDP 连接失败", zap.String("serverAddr", serverAddr), zap.Error(err))
				}
			}()

			buffer := make([]byte, 1024)

			for {
				select {
				case <-u.ctx.Done():
					log.Info("收到停止信号，关闭 UDP 连接", zap.String("serverAddr", serverAddr))
					return
				default:
					// 设置读取超时时间
					err := conn.SetReadDeadline(time.Now().Add(u.clientConfig.Timeout))
					if err != nil {
						log.Error("设置读取超时失败", zap.String("serverAddr", serverAddr), zap.Error(err))
						return
					}

					// 从 UDP 服务器接收数据
					n, _, err := conn.ReadFromUDP(buffer)
					if err != nil {
						var opErr *net.OpError
						if errors.As(err, &opErr) && opErr.Timeout() {
							log.Warn("读取超时，准备重新接收", zap.String("serverAddr", serverAddr))
						}
					}

					// 将接收到的数据写入到 Sink 的 writer 中
					if _, err := u.Sink.WriteASAP(buffer[:n]); err != nil {
						log.Error("写入数据到 Sink 失败", zap.Error(err))
						return // 写入失败，退出以重连
					}
				}
			}
		}()

		// 重连等待
		log.Info("5秒后重试 UDP 连接", zap.String("serverAddr", serverAddr))
		time.Sleep(u.clientConfig.Timeout)
	}
}
