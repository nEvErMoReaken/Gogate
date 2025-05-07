package connector

import (
	"context"
	"errors"
	"fmt"
	"gateway/internal/parser"
	"gateway/internal/pkg"
	"net"
	"time"

	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

const (
	DefaultMaxFrameSize   = 65507 // IPv4允许的最大UDP数据包大小
	DefaultBufferSize     = 16 * 1024
	DefaultReconnectDelay = 3 * time.Second
	DefaultTimeout        = 3 * time.Second
)

// UdpConnector 是 UDP版本的 Template 实现
// 无连接的，所以需要手动的管理连接
// 为了和Tcp保持复用，也为了应用ringbuffer
// 使用一个伪流包装udp，使parser可以无感复用
type UdpConnector struct {
	ctx        context.Context
	config     *UdpConfig
	workersMap map[string]chan []byte
}

type UdpConfig struct {
	Url            string            `mapstructure:"url"`            // 监听地址
	WhiteList      bool              `mapstructure:"whiteList"`      // 是否启用白名单
	IPAlias        map[string]string `mapstructure:"ipAlias"`        // ip别名
	Timeout        time.Duration     `mapstructure:"timeout"`        // 超时时间
	ReconnectDelay time.Duration     `mapstructure:"reconnectDelay"` // 重连延迟
	BufferSize     int               `mapstructure:"bufferSize"`     // 缓冲区大小
	MaxFrameSize   int               `mapstructure:"maxFrameSize"`   // 最大帧大小
}

func init() {
	Register("udp", NewUdpConnector)
}

// NewUdpConnector 创建并初始化 UdpConnector
func NewUdpConnector(ctx context.Context) (Template, error) {
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
	} else {
		config.Connector.Para["timeout"] = DefaultTimeout
	}
	// 处理 reconnectdelay 字段
	if reconnectDelayStr, ok := config.Connector.Para["reconnectdelay"].(string); ok {
		duration, err := time.ParseDuration(reconnectDelayStr)
		if err != nil {
			pkg.LoggerFromContext(ctx).Error("解析超时配置失败", zap.Error(err))
			return nil, fmt.Errorf("解析超时配置失败: %s", err)
		}
		config.Connector.Para["reconnectdelay"] = duration
	} else {
		config.Connector.Para["reconnectdelay"] = DefaultReconnectDelay
	}

	// 给bufferSize字段设置默认值
	if _, ok := config.Connector.Para["buffersize"]; !ok {
		config.Connector.Para["buffersize"] = DefaultBufferSize
	}

	if _, ok := config.Connector.Para["maxFrameSize"]; !ok {
		config.Connector.Para["maxFrameSize"] = DefaultMaxFrameSize
	}

	// 初始化配置结构
	var udpConfig UdpConfig
	err := mapstructure.Decode(config.Connector.Para, &udpConfig)
	if err != nil {
		pkg.LoggerFromContext(ctx).Error("配置文件解析失败", zap.Error(err))
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}

	// 初始化并返回 UdpConnector
	return &UdpConnector{
		ctx:        ctx,
		config:     &udpConfig,
		workersMap: make(map[string]chan []byte),
	}, nil
}

// Start 方法启动udp监听服务器的数据
// Udp 是无连接的，所以我们需要手动的管理连接
// 最简单的差异点是，我们需要识别数据来源ip，然后给予别名
// 然后分给不同的协程处理，可以增加并发处理能力
func (u *UdpConnector) Start(sink *pkg.Parser2DispatcherChan) error {
	log := pkg.LoggerFromContext(u.ctx)
	log.Info("===正在启动Connector: Udp===")

	// 配置本地UDP监听地址
	addr, err := net.ResolveUDPAddr("udp", u.config.Url)
	if err != nil {
		log.Error("解析 UDP 地址失败", zap.Error(err))
		return fmt.Errorf("解析 UDP 地址失败: %s\n", err)
	}

	conn, err := net.ListenUDP("udp", addr)

	if err != nil {
		log.Error("UDP监听程序启动失败", zap.Error(err))
		return fmt.Errorf("UDP监听程序启动失败: %s\n", err)
	}

	go u.handleConnection(conn, *sink)

	return nil
}

func (u *UdpConnector) handleConnection(conn *net.UDPConn, sink pkg.Parser2DispatcherChan) error {
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例
	log := pkg.LoggerFromContext(u.ctx)

	// 1. 设置连接超时
	if err := conn.SetReadDeadline(time.Time{}); err != nil { // 无限超时
		log.Error("设置 UDP 连接超时失败", zap.Error(err))
		return fmt.Errorf("设置 UDP 连接超时失败: %s", err)
	}

	// 2. 处理UDP数据
	go func() {
		for {
			buffer := pkg.BytesPoolInstance.Get()
			// 2.1 从 UDP 服务器接收数据
			n, ddr, err := conn.ReadFromUDP(buffer)

			if err != nil {
				// 如果连接已关闭，则退出循环
				if errors.Is(err, net.ErrClosed) {
					log.Info("UDP 连接已关闭")
					return
				}
				// 否则记录错误并继续
				metrics.IncErrorCount()
				metrics.IncMsgErrors("udp")
				log.Error("从 UDP 服务器接收数据失败", zap.Error(err))
				pkg.BytesPoolInstance.Put(buffer) // 释放缓冲区
				continue
			}

			// 2.2 处理白名单
			addrStr := ddr.String()
			if u.config.WhiteList {
				if _, exists := u.config.IPAlias[addrStr]; !exists {
					metrics.IncMsgErrors("udp_whitelist")
					log.Warn("白名单启用，拒绝未在白名单中的连接", zap.String("remote", addrStr))
					pkg.BytesPoolInstance.Put(buffer) // 释放缓冲区
					continue
				}
			}

			// 2.3 处理数据源
			dataSource, exists := u.workersMap[addrStr]
			if !exists {
				log.Info("接收到新的数据源", zap.String("addr", addrStr))
				// 创建一个channel
				dataSource = make(chan []byte, 1024)
				u.workersMap[addrStr] = dataSource

				// 为每个数据源启动一个专用的worker
				go u.startWorker(addrStr, dataSource, sink)
			}

			// 2.4 将接收到的数据全部写入到channel中
			select {
			case <-u.ctx.Done():
				return
			case dataSource <- buffer[:n]:
				// 数据已发送
			default:
				// channel已满，记录错误
				log.Warn("数据源channel已满，丢弃数据包", zap.String("addr", addrStr))
				pkg.BytesPoolInstance.Put(buffer) // 释放缓冲区
			}

			// 2.5 记录成功处理的消息
			metrics.IncMsgProcessed("udp")
		}
	}()
	return nil
}

// startWorker 启动一个工作协程来处理特定数据源的数据
func (u *UdpConnector) startWorker(addrStr string, dataChan chan []byte, sink pkg.Parser2DispatcherChan) {
	log := pkg.LoggerFromContext(u.ctx)

	log.Info("启动UDP数据源工作协程", zap.String("addr", addrStr))
	parser, err := parser.NewByteParser(u.ctx)
	if err != nil {
		log.Error("创建字节解析器失败", zap.Error(err))
		return
	}

	parser.StartWithChan(dataChan, sink)

}
