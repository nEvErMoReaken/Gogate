package connector

import (
	"bufio"
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

// TcpServerConnector Connector的TcpServer版本实现
type TcpServerConnector struct {
	ctx          context.Context
	listener     net.Listener
	chReady      chan pkg.DataSource
	serverConfig ServerConfig
	activeConns  sync.Map // 使用 sync.Map 来存储活跃的连接
}

type ServerConfig struct {
	WhiteList bool              `mapstructure:"whiteList"`
	IPAlias   map[string]string `mapstructure:"ipAlias"`
	Port      string            `mapstructure:"port"`
	Timeout   time.Duration     `mapstructure:"timeout"`
}

func (t *TcpServerConnector) Ready() chan pkg.DataSource {
	return t.chReady
}

func init() {
	Register("tcpServer", NewTcpServer)
}

func NewTcpServer(ctx context.Context) (Connector, error) {
	// 1. 获取配置上下文
	config := pkg.ConfigFromContext(ctx)

	// 2. 处理 timeout 字段（从字符串解析为 time.Duration）
	if timeoutStr, ok := config.Connector.Para["timeout"].(string); ok {
		duration, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("解析超时配置失败: %s", err)
		}
		config.Connector.Para["timeout"] = duration // 替换为 time.Duration
	}

	// 3. 初始化配置结构
	var serverConfig ServerConfig
	err := mapstructure.Decode(config.Connector.Para, &serverConfig)
	if err != nil {
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}

	// 4. 初始化listener
	listener, err := net.Listen("tcp", ":"+serverConfig.Port)
	if err != nil {
		return nil, fmt.Errorf("tcpServer监听程序启动失败: %s\n", err)
	}

	return &TcpServerConnector{
		ctx:          ctx,
		chReady:      make(chan pkg.DataSource, 1),
		listener:     listener,
		serverConfig: serverConfig,
	}, nil
}

func (t *TcpServerConnector) GetDataSource() (pkg.DataSource, error) {
	// 懒连接器，不需要也无法返回数据源
	return pkg.DataSource{}, fmt.Errorf("懒连接器无法立即返回数据源")
}
func (t *TcpServerConnector) Start() {
	log := pkg.LoggerFromContext(t.ctx)
	// 1. 监听指定的端口
	log.Info("TCPServer listening on: " + t.serverConfig.Port)
	for {
		// 该循环会一直阻塞，直到有新的连接到来
		// 只有两种情况会退出循环：1.监听器关闭 2.接受连接失败
		// 所以需要该循环理论上无法自我逃逸，必须依赖外部的 Close 方法来关闭监听器
		conn, err := t.listener.Accept()
		if err != nil {
			// 检查错误是否由于监听器已关闭
			if errors.Is(err, net.ErrClosed) {
				log.Info("监听器已关闭，停止接受连接")
				return
			}
			log.Error("接受连接失败", zap.Error(err))
			continue
		}
		// 将连接存储到 activeConns
		connID := conn.RemoteAddr().String()
		t.activeConns.Store(connID, conn)
		go func(conn net.Conn) {
			// 不在这里关闭连接，让上层代码（例如读取操作完毕后）来管理关闭
			pkg.LoggerFromContext(t.ctx).Info("建立连接", zap.String("remote", conn.RemoteAddr().String()))
			dataSource, err := t.initConn(conn)
			if err == nil {
				// 将初始化的 DataSource 发送到通道
				select {
				case t.chReady <- dataSource:
					log.Info("DataSource 已准备好")
				case <-time.After(3 * time.Second): // 可以用配置的超时替代
					log.Warn("发送 DataSource 到 chReady 超时")
				}
			} else {
				log.Error("初始化连接失败", zap.String("remote", conn.RemoteAddr().String()), zap.Error(err))
				// 在初始化失败时关闭连接
				err := conn.Close()
				if err != nil {
					log.Warn("关闭连接失败", zap.String("remote", conn.RemoteAddr().String()), zap.Error(err))
				}
			}
		}(conn)
	}
}

func (t *TcpServerConnector) initConn(conn net.Conn) (pkg.DataSource, error) {
	log := pkg.LoggerFromContext(t.ctx)
	// 不要在这里关闭连接，让上层代码（例如读取操作完毕后）来管理关闭！！
	// 1. 获取远程地址
	remoteAddrWithPort := conn.RemoteAddr().String()
	remoteAddr, _, err := net.SplitHostPort(remoteAddrWithPort)
	if err != nil {
		// 出现错误时，立刻关闭连接
		err := conn.Close()
		if err != nil {
			return pkg.DataSource{}, err
		}
		return pkg.DataSource{}, fmt.Errorf("无法解析远程地址: %v", remoteAddrWithPort)
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
			return pkg.DataSource{}, fmt.Errorf("白名单启用，拒绝连接: %s", remoteAddr)
		}
	}
	// 3. 设置超时时间
	if err := conn.SetReadDeadline(time.Now().Add(t.serverConfig.Timeout)); err != nil {
		// 设置超时时间失败时关闭连接
		err := conn.Close()
		if err != nil {
			return pkg.DataSource{}, err
		}
		return pkg.DataSource{}, fmt.Errorf("设置超时时间失败，关闭连接: %s", conn.RemoteAddr().String())
	}

	// 4. 初始化reader开始读数据
	reader := bufio.NewReader(conn)
	return pkg.DataSource{
		Source: reader,
		MetaData: map[string]interface{}{
			"deviceId": deviceId,
		},
	}, nil
}
func (t *TcpServerConnector) Close() error {
	log := pkg.LoggerFromContext(t.ctx)
	log.Info("关闭监听器并停止接收新连接")

	// 关闭监听器，停止接收新连接
	err := t.listener.Close()
	if err != nil {
		return fmt.Errorf("[tcpServer]关闭监听程序失败: %s\n", err)
	}

	// 关闭所有活跃连接
	t.closeAllActiveConnections()

	return nil
}

func (t *TcpServerConnector) closeAllActiveConnections() {
	log := pkg.LoggerFromContext(t.ctx)
	t.activeConns.Range(func(key, value interface{}) bool {
		conn := value.(net.Conn)
		log.Info("关闭连接", zap.String("remote", conn.RemoteAddr().String()))
		err := conn.Close()
		if err != nil {
			return false
		} // 关闭连接
		return true // 继续遍历
	})
}
