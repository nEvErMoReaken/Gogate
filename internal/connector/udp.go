package connector

import (
	"context"
	"errors"
	"fmt"
	"gateway/internal/pkg"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"net"
	"sync"
	"time"
)

// UdpConnector 是 UDP版本的 Template 实现
type UdpConnector struct {
	ctx        context.Context
	config     *UdpConfig
	bufferPool *sync.Pool // 缓冲区池
}

type UdpConfig struct {
	Url            string            `mapstructure:"url"`            // 监听地址
	WhiteList      bool              `mapstructure:"whiteList"`      // 是否启用白名单
	IPAlias        map[string]string `mapstructure:"ipAlias"`        // ip别名
	Timeout        time.Duration     `mapstructure:"timeout"`        // 超时时间
	ReconnectDelay time.Duration     `mapstructure:"reconnectDelay"` // 重连延迟
	BufferSize     int               `mapstructure:"bufferSize"`     // 缓冲区大小
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

	// 给bufferSize字段设置默认值
	if _, ok := config.Connector.Para["bufferSize"]; !ok {
		config.Connector.Para["bufferSize"] = 1024
	}
	// 初始化配置结构
	var udpConfig UdpConfig
	err := mapstructure.Decode(config.Connector.Para, &udpConfig)
	if err != nil {
		pkg.LoggerFromContext(ctx).Error("配置文件解析失败", zap.Error(err))
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}

	// 初始化 bufferPool
	bufferPool := &sync.Pool{
		New: func() interface{} {
			return make([]byte, udpConfig.BufferSize)
		},
	}
	// 初始化并返回 UdpConnector
	return &UdpConnector{
		ctx:        ctx,
		config:     &udpConfig,
		bufferPool: bufferPool,
	}, nil
}

func (u *UdpConnector) GetType() string {
	return "message"
}

// Start 方法启动udp监听服务器的数据
func (u *UdpConnector) Start(sourceChan chan pkg.DataSource) error {

	log := pkg.LoggerFromContext(u.ctx)
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

	go func() {
		for {
			select {
			case <-u.ctx.Done():
				log.Info("==收到停止信号，关闭 UDP监听 ==")
				if err = conn.Close(); err != nil {
					log.Error("关闭 UDP 连接失败", zap.Error(err))
				}
			default:
			}
		}
	}()
	// 数据来源到 DataSource 的映射表
	dataSourceMap := make(map[string]*pkg.MessageDataSource)
	go func() {
		for {
			buffer := u.bufferPool.Get().([]byte) // 缓冲区
			//bf := make([]byte, 200)
			// 从 UDP 服务器接收数据
			n, ddr, e := conn.ReadFromUDP(buffer)

			if e != nil {
				u.bufferPool.Put(buffer)
				// 如果连接已关闭，则退出循环
				if errors.Is(err, net.ErrClosed) {
					log.Info("UDP 连接已关闭")
					return
				}
				// 否则记录错误并继续
				log.Error("从 UDP 服务器接收数据失败", zap.Error(err))
				continue
			}
			//log.Debug(string(buffer[:n]))
			addrStr := ddr.String()
			if u.config.WhiteList {
				if _, exists := u.config.IPAlias[addrStr]; !exists {
					log.Warn("白名单启用，拒绝未在白名单中的连接", zap.String("remote", addrStr))
				}
			}

			dataSource, exists := dataSourceMap[addrStr]
			if !exists {
				log.Info("接收到新的数据源", zap.String("addr", addrStr))
				ds := pkg.NewMessageDataSource()
				ds.MetaData["remote"] = addrStr
				dataSourceMap[addrStr] = ds
				sourceChan <- ds
				dataSource = ds
			}
			// 将接收到的数据写入到 Sink 的 writer 中
			if err = dataSource.WriteOne(buffer[:n]); err != nil {
				log.Error("写入数据到 Sink 失败", zap.Error(err))
				return
			}
			//	将缓冲区放回池中
			u.bufferPool.Put(buffer)
		}
	}()

	return nil

}
