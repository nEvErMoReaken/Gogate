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

// UdpConfig 包含 UDP 配置信息
type UdpConfig struct {
	IPAlias    map[string]string `mapstructure:"ipAlias"`
	Port       string            `mapstructure:"port"`
	BufferSize int               `mapstructure:"bufferSize"`
	Timeout    time.Duration     `mapstructure:"timeout"`
}

// UdpConnector Connector的Udp版本实现
type UdpConnector struct {
	ctx      context.Context
	config   *UdpConfig
	dataChan chan string // 数据通道
	conn     *net.UDPConn
}

func init() {
	Register("udp", NewUdpConnector)
}

func (u *UdpConnector) Start() {
	log := pkg.LoggerFromContext(u.ctx)
	// 指定UDP地址
	addrToListen, err := net.ResolveUDPAddr("udp", u.config.Port)
	if err != nil {
		log.Error("Error resolving UDP address", zap.Error(err))
		return
	}

	// 监听UDP连接
	u.conn, err = net.ListenUDP("udp", addrToListen)
	if err != nil {
		log.Error("Error listening on UDP", zap.Error(err))
		return
	}

	log.Info("UDP服务已启动", zap.String("port", u.config.Port))

	// 不断接收消息
	for {
		select {
		case <-u.ctx.Done():
			log.Info("UDP 服务停止中...")
			err := u.Close()
			if err != nil {
				log.Error("UDP 服务关闭失败", zap.Error(err))
				return
			}
			return
		default:
			// 在每次读取之前设置读取超时
			err := u.conn.SetReadDeadline(time.Now().Add(u.config.Timeout))
			if err != nil {
				log.Error("Error setting read deadline", zap.Error(err))
				continue
			}

			buffer := make([]byte, u.config.BufferSize) // 缓冲区
			n, addr, err := u.conn.ReadFromUDP(buffer)
			if err != nil {
				// 检查是否为超时错误
				var opErr *net.OpError
				if errors.As(err, &opErr) && opErr.Timeout() {
					continue // 继续等待新的数据包
				}
				log.Error("Error reading from UDP", zap.Error(err))
				return
			}
			// 2. 初始化设备ID
			var deviceId string
			remoteAddr := addr.String()
			// 2.1 处理 IP 别名，无论白名单是否开启，都会影响 deviceId
			if alias, exists := u.config.IPAlias[remoteAddr]; exists {
				deviceId = alias
				log.Info("已找到 IP 别名", zap.String("remote", remoteAddr), zap.String("deviceId", deviceId))
			} else {
				// 如果没有匹配到别名，使用默认 IP 地址作为 deviceId
				deviceId = remoteAddr
				log.Info("IP 别名未找到，使用默认 deviceId", zap.String("remote", remoteAddr), zap.String("deviceId", deviceId))
			}

			// 将接收到的数据和设备ID作为json字符串发送到数据通道
			u.dataChan <- fmt.Sprintf(`{"deviceId":"%s","data":"%s"}`, deviceId, string(buffer[:n]))
		}
	}
}

func (u *UdpConnector) Ready() chan pkg.DataSource {
	// 饿连接器可以立即返回数据源无需通道
	return nil
}

func (u *UdpConnector) GetDataSource() (pkg.DataSource, error) {
	return pkg.DataSource{
		Source:   u.dataChan,
		MetaData: nil,
	}, nil
}

func (u *UdpConnector) Close() error {
	pkg.LoggerFromContext(u.ctx).Info("UDP连接正在关闭...")
	err := u.conn.Close()
	if err != nil {
		return fmt.Errorf("UDP连接关闭失败: %s", err)
	}
	return nil
}

func NewUdpConnector(ctx context.Context) (connector Connector, err error) {
	// 1. 初始化配置文件
	config := pkg.ConfigFromContext(ctx)
	// 2. 处理 timeout 字段（从字符串解析为 time.Duration）
	if timeoutStr, ok := config.Connector.Para["timeout"].(string); ok {
		duration, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("解析超时配置失败: %s", err)
		}
		config.Connector.Para["timeout"] = duration // 替换为 time.Duration
	}
	var udpConfig UdpConfig
	err = mapstructure.Decode(config.Connector.Para, &udpConfig)
	if err != nil {
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}

	// 3. 创建 MQTT Connector 实例
	udpConn := &UdpConnector{
		ctx:      ctx,
		config:   &udpConfig,
		dataChan: make(chan string, 200),
	}

	return udpConn, nil
}
