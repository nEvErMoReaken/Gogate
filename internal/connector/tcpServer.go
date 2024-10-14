package connector

import (
	"bufio"
	"context"
	"fmt"
	"gateway/internal/pkg"
	"gateway/util"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"net"
	"time"
)

// TcpServerConnector Connector的TcpServer版本实现
type TcpServerConnector struct {
	ctx          context.Context
	listener     net.Listener
	chReady      chan pkg.DataSource
	serverConfig ServerConfig
}

type ServerConfig struct {
	WhiteList bool              `mapstructure:"whiteList"`
	IPAlias   map[string]string `mapstructure:"ipAlias"`
	Port      string            `mapstructure:"port"`
	Timeout   time.Duration     `mapstructure:"timeout"`
}

func (t *TcpServerConnector) Ready() <-chan pkg.DataSource {
	return t.chReady
}

func init() {
	Register("tcpServer", NewTcpServer)
}

func NewTcpServer(ctx context.Context) (Connector, error) {
	// 1. 初始化配置文件
	v := pkg.ConfigFromContext(ctx)
	var serverConfig ServerConfig
	err := mapstructure.Decode(v.Connector.Para, &serverConfig)
	if err != nil {
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}
	// 2. 初始化listener
	listener, err := net.Listen("tcp", ":"+serverConfig.Port)
	if err != nil {
		return nil, fmt.Errorf("tcpServer监听程序启动失败: %s\n", err)
	}

	return &TcpServerConnector{
		ctx:          ctx,
		chReady:      make(chan pkg.DataSource),
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
		// 2. 等待客户端连接，阻塞
		conn, err := t.listener.Accept()
		if err != nil {
			util.ErrChanFromContext(t.ctx) <- fmt.Errorf("[tcpServer]接受连接失败: %s\n", err)
		}
		// 3. 处理连接
		pkg.LoggerFromContext(t.ctx).Info("与 %s 建立连接", zap.String("remote", conn.RemoteAddr().String()))
		reader, err := t.initConn(conn)
		if err == nil {
			t.chReady <- reader
		} else {
			log.Error("初始化连接失败", zap.String("remote", conn.RemoteAddr().String()), zap.Error(err))
		}
	}
}

func (t *TcpServerConnector) initConn(conn net.Conn) (pkg.DataSource, error) {
	log := pkg.LoggerFromContext(t.ctx)
	defer func(conn net.Conn) {
		time.Sleep(1 * time.Second) // 等待 1 秒，确保所有数据源都已经关闭
		err := conn.Close()
		if err != nil {
			pkg.LoggerFromContext(t.ctx).Info("与 %s 的连接已关闭", zap.String("remote", conn.RemoteAddr().String()))
		}
	}(conn)
	// 1. 首先识别远端ip是哪个设备
	remoteAddrWithPort := conn.RemoteAddr().String()
	var deviceId = ""
	// 2. 连接花名作为变量（如果有）
	if t.serverConfig.IPAlias == nil {
		// 2.1 如果IPAlias为空，则不需要进行识别
		log.Info("IPAlias为空, 使用默认地址")
	} else {
		// 2.2 如果IPAlias不为空，放入变量中
		// remoteAddr 是 ip:port 的形式，需要去掉端口
		remoteAddr, _, _ := net.SplitHostPort(remoteAddrWithPort)
		var exists bool
		deviceId, exists = t.serverConfig.IPAlias[remoteAddr]
		if !exists && t.serverConfig.WhiteList { // 如果白名单开启，但是没有找到对应的设备id
			return pkg.DataSource{
				Source:   nil,
				MetaData: nil,
			}, fmt.Errorf("未在配置清单中找到地址: %s", remoteAddr)
		}
	}
	// 3. 设置超时时间
	err := conn.SetReadDeadline(time.Now().Add(t.serverConfig.Timeout))
	if err != nil {
		//log.Error("超时时间设置失败，关闭连接", zap.String("remote", conn.RemoteAddr().String()))
		return pkg.DataSource{
			Source:   nil,
			MetaData: nil,
		}, fmt.Errorf("超时时间设置失败，关闭连接: %s", conn.RemoteAddr().String())
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
	err := t.listener.Close()
	if err != nil {
		return fmt.Errorf("[tcpServer]关闭监听程序失败: %s\n", err)
	}
	return nil
}
