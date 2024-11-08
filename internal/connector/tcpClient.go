package connector

import (
	"bufio"
	"context"
	"fmt"
	"gateway/internal/pkg"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"net"
	"time"
)

// TcpClientConnector Connector的TcpClient版本实现
type TcpClientConnector struct {
	ctx          context.Context
	conn         net.Conn
	chReady      chan pkg.DataSource
	clientConfig ClientConfig
}

func (t *TcpClientConnector) GetDataSource() (pkg.DataSource, error) {
	// 懒连接器 ，不实现该部分
	return pkg.DataSource{
		Source:   nil,
		MetaData: nil,
	}, nil
}

type ClientConfig struct {
	ServerAddr string        `mapstructure:"serverAddr"` // 服务器地址
	Timeout    time.Duration `mapstructure:"timeout"`    // 超时时间
}

// Ready 方法返回 DataSou	rce 准备好的通道
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
	log := pkg.LoggerFromContext(t.ctx)
	// 尝试连接到服务器
	err := t.connectToServer()
	if err != nil {
		log.Error("无法连接到服务器", zap.Error(err))
		t.reconnect()
	}
}

// connectToServer 尝试连接到服务器
func (t *TcpClientConnector) connectToServer() error {
	log := pkg.LoggerFromContext(t.ctx)
	// 1. 建立连接
	log.Debug("正在连接到服务器: " + t.clientConfig.ServerAddr)
	conn, err := net.DialTimeout("tcp", t.clientConfig.ServerAddr, t.clientConfig.Timeout)
	if err != nil {
		return fmt.Errorf("无法连接到服务器: %v", err)
	}
	t.conn = conn

	// 2. 设置超时时间
	err = conn.SetReadDeadline(time.Now().Add(t.clientConfig.Timeout))
	if err != nil {
		err = conn.Close()
		if err != nil {
			return fmt.Errorf("设置超时时间失败并且在关闭时发生错误: %v", err)
		}
		return fmt.Errorf("设置超时时间失败: %v", err)
	}

	// 3. 初始化连接
	dataSource, err := t.initConn(conn)
	if err != nil {
		err := conn.Close()
		if err != nil {
			return fmt.Errorf("初始化连接失败并且在关闭时发生错误: %v", err)
		}
		return fmt.Errorf("初始化连接失败: %v", err)
	}

	// 4. 发送数据源到准备好的通道
	select {
	case t.chReady <- dataSource:
		log.Info("DataSource 已准备好")
	case <-time.After(3 * time.Second):
		log.Warn("发送 DataSource 到 chReady 超时")
	}

	return nil
}

// initConn 初始化连接，并返回数据源
func (t *TcpClientConnector) initConn(conn net.Conn) (pkg.DataSource, error) {
	// 1. 通过 bufio.Reader 读取数据
	reader := bufio.NewReader(conn)

	// 2. 返回数据源
	return pkg.DataSource{
		Source: reader,
		MetaData: map[string]interface{}{
			"clientAddr": t.clientConfig.ServerAddr,
		},
	}, nil
}

// reconnect 处理断线重连逻辑
func (t *TcpClientConnector) reconnect() {
	log := pkg.LoggerFromContext(t.ctx)
	for {
		// 重新尝试连接
		time.Sleep(5 * time.Second) // 重连间隔
		err := t.connectToServer()
		if err == nil {
			log.Info("重连成功")
			break
		}
		log.Warn("重连失败，5秒后重试...", zap.Error(err))
	}
}

// Close 关闭客户端连接
func (t *TcpClientConnector) Close() error {
	log := pkg.LoggerFromContext(t.ctx)
	if t.conn != nil {
		err := t.conn.Close()
		if err != nil {
			log.Error("关闭连接失败", zap.Error(err))
			return err
		}
		log.Info("连接已关闭")
	}
	return nil
}
