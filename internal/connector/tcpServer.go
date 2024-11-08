package connector

import (
	"context"
	"errors"
	"fmt"
	"gateway/internal/pkg"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"io"
	"net"
	"time"
)

// TcpServerConnector Connector的TcpServer版本实现
type TcpServerConnector struct {
	ctx          context.Context
	listener     net.Listener
	chReady      chan pkg.DataSource
	serverConfig ServerConfig
	Sink         pkg.StreamDataSource
}

type ServerConfig struct {
	WhiteList bool              `mapstructure:"whiteList"`
	IPAlias   map[string]string `mapstructure:"ipAlias"`
	Port      string            `mapstructure:"port"`
	Timeout   time.Duration     `mapstructure:"timeout"`
}

func (t *TcpServerConnector) SinkType() string {
	return "stream"
}

func (t *TcpServerConnector) SetSink(source *pkg.DataSource) {
	if sink, ok := (*source).(*pkg.StreamDataSource); ok {
		t.Sink = *sink
	} else {
		pkg.LoggerFromContext(t.ctx).Error("TcpServer数据源类型错误, 期望pkg.StreamDataSource")
	}
}

func (t *TcpServerConnector) Ready() chan pkg.DataSource {
	return t.chReady
}

func init() {
	Register("tcpserver", NewTcpServer)
}

func NewTcpServer(ctx context.Context) (Connector, error) {
	// 1. 获取配置上下文
	config := pkg.ConfigFromContext(ctx)

	// 2. 处理 timeout 字段（从字符串解析为 time.Duration）
	if timeoutStr, ok := config.Connector.Para["timeout"].(string); ok {
		duration, err := time.ParseDuration(timeoutStr)
		if err != nil {
			pkg.LoggerFromContext(ctx).Error("配置文件解析失败", zap.Error(err))
			return nil, fmt.Errorf("配置文件解析失败: %s", err)
		}
		config.Connector.Para["timeout"] = duration // 替换为 time.Duration
	}

	// 3. 初始化配置结构
	var serverConfig ServerConfig
	err := mapstructure.Decode(config.Connector.Para, &serverConfig)
	if err != nil {
		pkg.LoggerFromContext(ctx).Error("解析超时配置失败", zap.Error(err))
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}

	// 4. 初始化listener
	listener, err := net.Listen("tcp", ":"+serverConfig.Port)
	if err != nil {
		pkg.LoggerFromContext(ctx).Error("解析超时配置失败", zap.Error(err))
		return nil, fmt.Errorf("tcpServer监听程序启动失败: %s\n", err)
	}

	return &TcpServerConnector{
		ctx:          ctx,
		chReady:      make(chan pkg.DataSource, 1),
		listener:     listener,
		serverConfig: serverConfig,
	}, nil
}

func (t *TcpServerConnector) Start() {
	log := pkg.LoggerFromContext(t.ctx)
	// 1. 监听指定的端口
	log.Debug("TCPServer listening on: " + t.serverConfig.Port)

	// 2. 在新的协程中监听 ctx.Done()
	go func() {
		<-t.ctx.Done()
		// 关闭 listener 使 Accept() 退出阻塞状态
		if err := t.listener.Close(); err != nil {
			log.Error("关闭监听器失败", zap.Error(err))
		}
	}()

	for {
		// 该循环会一直阻塞，直到有新地连接到来
		// 只有两种情况会退出循环：1.监听器关闭 2.接受连接失败
		// 所以需要该循环理论上无法自我逃逸，必须依赖外部的 Close 方法来关闭监听器
		conn, err := t.listener.Accept()
		if err != nil {
			// 检查错误是否由于监听器已关闭
			if errors.Is(err, net.ErrClosed) {
				log.Info("监听器已关闭，停止接受连接")
				return
			} else if errors.Is(err, io.EOF) {
				log.Info("收到 EOF 信号，停止接受连接")
				continue
			} else {
				log.Error("接受连接失败", zap.Error(err))
				continue
			}
		}

		connID := conn.RemoteAddr().String()
		// 不在这里关闭连接，让下层代码（例如读取操作完毕后）来管理关闭
		pkg.LoggerFromContext(t.ctx).Info("建立连接", zap.String("remote", conn.RemoteAddr().String()))
		err = t.initConn(conn)
		if err != nil {
			pkg.LoggerFromContext(t.ctx).Error("初始化连接失败，关闭连接", zap.String("remote", conn.RemoteAddr().String()), zap.Error(err))
			err := conn.Close()
			if err != nil {
				pkg.LoggerFromContext(t.ctx).Warn("关闭连接失败", zap.String("remote", conn.RemoteAddr().String()), zap.Error(err))
			}
		}
		go t.handleConn(conn, connID)
	}
}

func (t *TcpServerConnector) handleConn(conn net.Conn, connID string) {
	log := pkg.LoggerFromContext(t.ctx)
	// 创建缓冲区用于读取数据
	buffer := make([]byte, 1024)
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Error("关闭连接失败", zap.String("remote", connID), zap.Error(err))
			return
		}
		log.Info("连接已关闭", zap.String("remote", connID))
	}()

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			// 从 TCP 连接读取数据
			n, err := conn.Read(buffer)
			if err != nil {
				if err == io.EOF {
					log.Info("连接已断开", zap.String("remote", connID))
					break
				}
				log.Error("读取数据失败", zap.Error(err))
				break
			}

			// 将读取的数据写入到 Sink 的 writer 中
			if _, err := t.Sink.WriteASAP(buffer[:n]); err != nil {
				log.Error("写入数据到 Sink 失败", zap.Error(err))
				break
			}
		}
	}
}

func (t *TcpServerConnector) initConn(conn net.Conn) error {
	log := pkg.LoggerFromContext(t.ctx)
	// 不要在这里关闭连接，让上层代码（例如读取操作完毕后）来管理关闭！！
	// 1. 获取远程地址
	remoteAddrWithPort := conn.RemoteAddr().String()
	remoteAddr, _, err := net.SplitHostPort(remoteAddrWithPort)
	if err != nil {
		return fmt.Errorf("无法解析远程地址: %v", remoteAddrWithPort)
	}

	// 2. 初始化设备ID
	var deviceId string

	// 2.1 处理 IP 别名，无论白名单是否开启，都会影响 deviceId
	if alias, exists := t.serverConfig.IPAlias[remoteAddr]; exists {
		deviceId = alias
		log.Info("已找到 IP 别名", zap.String("remote", remoteAddr), zap.String("deviceId", deviceId))
	} else {
		// 如果没有匹配到别名，使用默认 IP 地址作为 deviceId
		deviceId = remoteAddr
		log.Info("IP 别名未找到，使用默认 deviceId", zap.String("remote", remoteAddr), zap.String("deviceId", deviceId))
	}

	// 2.2 检查白名单逻辑，如果白名单启用且没有在 ipAlias 中找到匹配，则拒绝连接
	if t.serverConfig.WhiteList {
		if _, exists := t.serverConfig.IPAlias[remoteAddr]; !exists {
			log.Warn("白名单启用，拒绝未在白名单中的连接", zap.String("remote", remoteAddr))
			return fmt.Errorf("白名单启用，拒绝连接: %s", remoteAddr)
		}
	}
	// 3. 设置超时时间
	if err = conn.SetReadDeadline(time.Now().Add(t.serverConfig.Timeout)); err != nil {
		return fmt.Errorf("设置超时时间失败 %s", conn.RemoteAddr().String())
	}

	return nil
}
