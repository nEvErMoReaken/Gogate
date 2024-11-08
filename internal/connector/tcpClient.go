package connector

import (
	"bufio"
	"context"
	"fmt"
	"gateway/internal/pkg"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"net"
	"strings"
	"sync"
	"time"
)

// TcpClientConnector Connector的TcpClient版本实现
type TcpClientConnector struct {
	ctx          context.Context
	chReady      chan pkg.DataSource
	clientConfig ClientConfig
	activeConns  sync.Map // 使用 sync.Map 来存储活跃的连接
}

func (t *TcpClientConnector) GetDataSource() (pkg.DataSource, error) {
	// 懒连接器，不实现该部分
	return pkg.DataSource{}, fmt.Errorf("懒连接器无法立即返回数据源")
}

type ClientConfig struct {
	ServerAddrs []string      `mapstructure:"serverAddrs"` // 服务器地址列表
	Timeout     time.Duration `mapstructure:"timeout"`     // 超时时间
}

// Ready 方法返回 DataSource 准备好的通道
func (t *TcpClientConnector) Ready() chan pkg.DataSource {
	return t.chReady
}

// init 函数注册 TcpClientConnector
func init() {
	Register("tcpclient", NewTcpClient)
}

// NewTcpClient 函数创建并初始化 TcpClientConnector
func NewTcpClient(ctx context.Context) (Connector, error) {
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

	// 处理 serverAddrs 字段，支持字符串列表或逗号分隔的字符串
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
	var clientConfig ClientConfig
	err := mapstructure.Decode(config.Connector.Para, &clientConfig)
	if err != nil {
		pkg.LoggerFromContext(ctx).Error("配置文件解析失败", zap.Error(err))
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}

	// 初始化并返回 TcpClientConnector
	return &TcpClientConnector{
		ctx:          ctx,
		chReady:      make(chan pkg.DataSource, 1),
		clientConfig: clientConfig,
	}, nil
}

// Start 方法启动客户端连接到服务器
func (t *TcpClientConnector) Start() {
	for _, serverAddr := range t.clientConfig.ServerAddrs {
		// 对每个服务器地址启动一个协程
		go t.manageConnection(serverAddr)
	}
}

// manageConnection 管理对单个服务器的连接，包括重连逻辑
func (t *TcpClientConnector) manageConnection(serverAddr string) {
	log := pkg.LoggerFromContext(t.ctx)
	for {
		// 尝试连接到服务器
		conn, err := net.DialTimeout("tcp", serverAddr, t.clientConfig.Timeout)
		if err != nil {
			log.Warn("无法连接到服务器，5秒后重试", zap.String("serverAddr", serverAddr), zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		// 将连接存储到 activeConns
		t.activeConns.Store(serverAddr, conn)

		// 初始化连接
		dataSource, err := t.initConn(conn, serverAddr)
		if err != nil {
			log.Error("初始化连接失败，关闭连接", zap.String("serverAddr", serverAddr), zap.Error(err))
			err := conn.Close()
			if err != nil {
				log.Warn("关闭连接失败", zap.String("serverAddr", serverAddr), zap.Error(err))
			}
			t.activeConns.Delete(serverAddr)
			continue
		}

		// 将初始化的 DataSource 发送到通道
		select {
		case t.chReady <- dataSource:
			log.Info("DataSource 已准备好", zap.String("serverAddr", serverAddr))
		case <-time.After(3 * time.Second):
			log.Warn("发送 DataSource 到 chReady 超时", zap.String("serverAddr", serverAddr))
		}
		// 监控连接状态，如果连接断开则重连

	}
}

// initConn 初始化连接，并返回数据源
func (t *TcpClientConnector) initConn(conn net.Conn, serverAddr string) (pkg.DataSource, error) {
	// 设置超时时间
	if err := conn.SetReadDeadline(time.Now().Add(t.clientConfig.Timeout)); err != nil {
		return pkg.DataSource{}, fmt.Errorf("设置超时时间失败，关闭连接: %s", conn.RemoteAddr().String())
	}

	// 创建 reader
	reader := bufio.NewReader(conn)

	// 返回数据源
	return pkg.DataSource{
		Source: reader,
		MetaData: map[string]interface{}{
			"serverAddr": serverAddr,
		},
	}, nil
}

// Close 关闭客户端所有连接
func (t *TcpClientConnector) Close() error {
	log := pkg.LoggerFromContext(t.ctx)
	log.Info("关闭所有连接")
	t.activeConns.Range(func(key, value interface{}) bool {
		conn := value.(net.Conn)
		serverAddr := key.(string)
		log.Info("关闭连接", zap.String("serverAddr", serverAddr))
		err := conn.Close()
		if err != nil {
			log.Warn("关闭连接失败", zap.String("serverAddr", serverAddr), zap.Error(err))
		}
		return true
	})
	return nil
}
