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

/*
数据链路 ==>

TCP连接
   ↓
环形缓冲区（RingBuffer）
   ↓
帧解析器（拆包、粘包、半包处理）
   ↓
业务处理（聚合、发送、落库等）

*/

// TcpServerConnector Connector的TcpServer版本实现
type TcpServerConnector struct {
	ctx          context.Context
	serverConfig *TcpServerConfig
}

type TcpServerConfig struct {
	WhiteList  bool              `mapstructure:"whiteList"`
	IPAlias    map[string]string `mapstructure:"ipAlias"`
	Url        string            `mapstructure:"url"`
	Timeout    time.Duration     `mapstructure:"timeout"`
	BufferSize int               `mapstructure:"bufferSize"` // 添加bufferSize配置
}

func init() {
	Register("tcpserver", NewTcpServer)
}

func NewTcpServer(ctx context.Context) (Template, error) {
	// 1. 获取配置上下文
	config := pkg.ConfigFromContext(ctx)

	// 2. 处理 timeout 字段（从字符串解析为 time.Duration）
	if timeoutStr, ok := config.Connector.Para["timeout"].(string); ok {
		duration, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("配置文件解析失败: %s", err)
		}
		config.Connector.Para["timeout"] = duration // 替换为 time.Duration
	}
	//
	if timeoutStr, ok := config.Connector.Para["timeout"].(string); ok {
		duration, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("配置文件解析失败: %s", err)
		}
		config.Connector.Para["timeout"] = duration // 替换为 time.Duration
	}

	// 给bufferSize字段设置默认值
	if _, ok := config.Connector.Para["buffersize"]; !ok {
		config.Connector.Para["buffersize"] = 1024
	}

	// 3. 初始化配置结构
	var serverConfig TcpServerConfig
	err := mapstructure.Decode(config.Connector.Para, &serverConfig)
	if err != nil {
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}

	return &TcpServerConnector{
		ctx:          ctx,
		serverConfig: &serverConfig,
	}, nil
}

// ==== 接口实现 ====

// Step1: Start
func (t *TcpServerConnector) Start(sink *pkg.Parser2DispatcherChan) error {
	log := pkg.LoggerFromContext(t.ctx)
	log.Info("===正在启动Connector: TcpServer===")
	// 1. 监听指定的端口
	log.Debug("TCPServer listening on: " + t.serverConfig.Url)
	listener, err := net.Listen("tcp", t.serverConfig.Url)
	if err != nil {
		return fmt.Errorf("tcpServer监听程序启动失败: %s\n", err)
	}
	// 2. 在新的协程中监听 ctx.Done()
	go func() {
		<-t.ctx.Done()
		// 关闭 listener 使 Accept() 退出阻塞状态
		if err = listener.Close(); err != nil {
			log.Error("关闭监听器失败", zap.Error(err))
		}
	}()
	// 3. 接受连接
	go func() {
		for {
			var conn net.Conn
			conn, err = listener.Accept()
			if err != nil {
				// 只有在监听器被明确关闭时才退出循环
				if errors.Is(err, net.ErrClosed) {
					log.Info("TCPServer 监听器已关闭，停止接受连接")
					return // 或break，取决于您的循环结构
				}

				// 其他错误记录后继续接受连接
				log.Error("TCPServer 接受连接失败", zap.Error(err))
				// 可选：短暂延迟避免CPU高占用
				time.Sleep(100 * time.Millisecond)
				continue
			}
			connID := conn.RemoteAddr().String()
			// 不在这里关闭连接，让下层代码（例如读取操作完毕后）来管理关闭
			log.Info("建立连接", zap.String("remote", conn.RemoteAddr().String()))
			err = t.initConn(conn)
			if err != nil {
				log.Error("初始化连接失败，关闭连接", zap.String("remote", conn.RemoteAddr().String()), zap.Error(err))
				err = conn.Close()
				if err != nil {
					log.Warn("关闭连接失败", zap.String("remote", conn.RemoteAddr().String()), zap.Error(err))
				}
				continue
			}

			go func() {
				err := t.handleConn(conn, connID, *sink)
				if err != nil {
					log.Error("处理连接失败", zap.Error(err))
					conn.Close()
				}
			}()
		}
	}()
	return nil
}

func (t *TcpServerConnector) handleConn(conn net.Conn, connID string, sink pkg.Parser2DispatcherChan) error {
	log := pkg.LoggerFromContext(t.ctx)
	// 从 TCP 连接读取数据
	n, err := pkg.NewRingBuffer(conn, uint32(t.serverConfig.BufferSize))
	if err != nil {
		log.Error("创建环形缓冲区失败", zap.Error(err))
	}
	// 创建字节解析器
	byteParser, err := parser.NewByteParser(t.ctx)
	if err != nil {
		log.Error("创建字节解析器失败", zap.Error(err))
	}
	// 启动字节解析器
	err = byteParser.StartWithRingBuffer(n, sink)
	if err != nil {
		return err
	}
	return nil
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

	// 处理 IPv6 地址 "::1"，将其视为 "127.0.0.1"
	if remoteAddr == "::1" {
		remoteAddr = "127.0.0.1"
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
