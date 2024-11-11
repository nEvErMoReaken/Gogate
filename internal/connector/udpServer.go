package connector

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"net"
	"sync"
	"time"
)

// UdpServerConfig 包含 UDP 配置信息
type UdpServerConfig struct {
	IPAlias    map[string]string `mapstructure:"ipAlias"`
	Port       string            `mapstructure:"port"`
	BufferSize int               `mapstructure:"bufferSize"`
	Timeout    time.Duration     `mapstructure:"timeout"`
}

// UdpConnector Connector的Udp版本实现
type UdpConnector struct {
	ctx        context.Context
	config     *UdpServerConfig
	conn       *net.UDPConn
	Sink       pkg.MessageDataSource // 数据通道
	bufferPool *sync.Pool            // 缓冲区池
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
			err = u.Close()
			if err != nil {
				log.Error("UDP 服务关闭失败", zap.Error(err))
				return
			}
			return
		default:
			// 在每次读取之前设置读取超时
			err = u.conn.SetReadDeadline(time.Now().Add(u.config.Timeout))
			if err != nil {
				log.Error("Error setting read deadline", zap.Error(err))
				continue
			}

			buffer := u.bufferPool.Get().([]byte) // 缓冲区
			var n int
			var addr *net.UDPAddr
			n, addr, err = u.conn.ReadFromUDP(buffer)
			if err != nil {
				log.Error("Error reading from UDP", zap.Error(err))

				continue
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

			// 3. 将接收到的数据和设备ID作为json字符串发送到数据通道
			err = u.Sink.WriteOne([]byte(fmt.Sprintf(`{"deviceId":"%s","data":"%s"}`, deviceId, string(buffer[:n]))))
			if err != nil {
				log.Error("Failed to write message to sink", zap.Error(err))
				return
			}
			// 4. 将缓冲区放回池中，供下次使用
			u.bufferPool.Put(buffer)
		}
	}
}

func (u *UdpConnector) SinkType() string {
	return "message"
}

func (u *UdpConnector) SetSink(source *pkg.DataSource) {
	// 确保接口断言类型是指针类型的 `MessageDataSource`
	if sink, ok := (*source).(*pkg.MessageDataSource); ok {
		u.Sink = *sink
	} else {
		pkg.LoggerFromContext(u.ctx).Error("Mqtt数据源类型错误, 期望pkg.MessageDataSource")
	}
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
	var udpConfig UdpServerConfig
	err = mapstructure.Decode(config.Connector.Para, &udpConfig)
	if err != nil {
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}
	// 初始化 bufferPool
	bufferPool := &sync.Pool{
		New: func() interface{} {
			return make([]byte, udpConfig.BufferSize)
		},
	}
	// 3. 创建 MQTT Connector 实例
	udpConn := &UdpConnector{
		ctx:        ctx,
		config:     &udpConfig,
		bufferPool: bufferPool,
	}

	return udpConn, nil
}
