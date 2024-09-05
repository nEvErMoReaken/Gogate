package tcpServer

import (
	"fmt"
	"gw22-train-sam/connecter/tcpServer/config"
	"gw22-train-sam/logger"
	"gw22-train-sam/model"
	"log"
	"net"
)

type TcpServer struct {
	listener        net.Listener      // 监听器
	TcpServerConfig *config.TcpServer // 配置
	Proto           *config.Proto     // 协议
}

func init() {
	model.Register("tcpServer", NewTcpServer)
}

func NewTcpServer() model.Connector {
	// 0. 创建一个新的 TcpServer
	tcpServer := &TcpServer{}

	// 1. 读取配置文件
	TcpServerConfig, Proto, err := config.NewConfig("config")
	if TcpServerConfig == nil || Proto == nil || err != nil {
		log.Fatalf("[tcpServer]加载配置失败: %s\n", err)
	}
	tcpServer.TcpServerConfig = TcpServerConfig
	tcpServer.Proto = Proto

	// 2. 初始化listener
	listener, err := net.Listen("tcp", ":"+TcpServerConfig.TCPServer.Port)
	if err != nil {
		log.Fatalf("[tcpServer]监听程序启动失败: %s\n", err)
	}
	tcpServer.listener = listener
	return tcpServer
}

func (t *TcpServer) Listen() error {
	// 1. 监听指定的端口
	logger.Log.Infof("TCP dataflow listening on port %s", t.TcpServerConfig.TCPServer.Port)
	for {
		// 2. 等待客户端连接
		conn, err := t.listener.Accept()
		if err != nil {
			return fmt.Errorf("[tcpServer]与客户端建立连接时发生错误: %s\n", err)
		}
		// 3. 使用 goroutine 处理连接，一个连接对应一个协程
		go handleConnection(t, conn)
	}
}

func (t *TcpServer) Close() error {
	err := t.listener.Close()
	if err != nil {
		return fmt.Errorf("[tcpServer]关闭监听程序失败: %s\n", err)
	}
	return nil
}

func Listen() {

}
