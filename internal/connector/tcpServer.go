package connector

import (
	"bufio"
	"context"
	"fmt"
	"gateway/internal/parser"
	"gateway/internal/pkg"
	"gateway/util"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"log"
	"net"
	"time"
)

// TcpServerConnector Connector的TcpServer版本实现
type TcpServerConnector struct {
	ctx          context.Context
	listener     net.Listener
	chReady      chan interface{}
	serverConfig ServerConfig
}

type ServerConfig struct {
	WhiteList bool              `mapstructure:"whiteList"`
	IPAlias   map[string]string `mapstructure:"ipAlias"`
	Port      string            `mapstructure:"port"`
	Timeout   time.Duration     `mapstructure:"timeout"`
}

func (t *TcpServerConnector) Ready() <-chan interface{} {
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
		return nil, fmt.Errorf("TCPServer配置文件解析失败: %s", err)
	}
	// 2. 初始化listener
	listener, err := net.Listen("tcp", ":"+serverConfig.Port)
	if err != nil {
		log.Fatalf("[tcpServer]监听程序启动失败: %s\n", err)
	}

	return &TcpServerConnector{
		ctx:          ctx,
		chReady:      make(chan interface{}),
		listener:     listener,
		serverConfig: serverConfig,
	}, nil
}

func (t *TcpServerConnector) GetDataSource() (interface{}, error) {
	// 懒连接器，不需要也无法返回数据源
	return nil, nil
}
func (t *TcpServerConnector) Start() {
	// 1. 监听指定的端口
	pkg.LoggerFromContext(t.ctx).Info("TCPServer listening on: " + t.serverConfig.Port)
	for {
		// 2. 等待客户端连接
		conn, err := t.listener.Accept()
		if err != nil {
			util.ErrChanFromContext(t.ctx) <- fmt.Errorf("[tcpServer]接受连接失败: %s\n", err)
		}
		// 3. 处理连接
		pkg.LoggerFromContext(t.ctx).Info("与 %s 建立连接", zap.String("remote", conn.RemoteAddr().String()))
		chunks, err := parser.InitChunks(t.ctx, pkg.ConfigFromContext(t.ctx).Parser.Type)
		t.HandleConnection(conn, &chunks)
	}
}

func (t *TcpServerConnector) Close() error {
	err := t.listener.Close()
	if err != nil {
		return fmt.Errorf("[tcpServer]关闭监听程序失败: %s\n", err)
	}
	return nil
}

// HandleConnection 处理连接, 一个连接对应一个协程
func (t *TcpServerConnector) HandleConnection(conn net.Conn, chunkSequence *parser.ChunkSequence) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			pkg.Log.Infof("与 %s 的连接已关闭", conn.RemoteAddr().String())
		}
	}(conn)
	// 1. 首先识别远端ip是哪个设备
	remoteAddrWithPort := conn.RemoteAddr().String()
	var deviceId = remoteAddrWithPort
	// 2. 连接花名作为变量（如果有）
	if t.TcpServerConfig.TCPServer.IPAlias == nil {
		// 2.1 如果IPAlias为空，则不需要进行识别
		pkg.Log.Infof("IPAlias为空")
	} else {
		// 2.2 如果IPAlias不为空，放入变量中
		// remoteAddr 是 ip:port 的形式，需要去掉端口
		remoteAddr, _, _ := net.SplitHostPort(remoteAddrWithPort)
		var exists bool
		deviceId, exists = t.TcpServerConfig.TCPServer.IPAlias[remoteAddr]
		if !exists {
			pkg.Log.Errorf("未在配置清单中找到地址: %s", remoteAddr)
			return
		}
		// deviceId 是string 转 *interface{}
		// 将 string 转换为 interface{}，然后创建指针
		var deviceIdInterface interface{} = deviceId
		chunkSequence.VarPointer["device_id"] = &deviceIdInterface
	}
	// 3. 设置超时时间
	err := conn.SetReadDeadline(time.Now().Add(t.TcpServerConfig.TCPServer.Timeout))
	if err != nil {
		pkg.Log.Infof("超时时间设置失败，关闭连接: %s", conn.RemoteAddr().String())
		return
	}
	// 4. 初始化reader开始读数据
	reader := bufio.NewReader(conn)
	//for {
	//	select {
	//	case <-t.ChDone:
	//		return
	//	default:
	//		// 4.1 Frame 数组，用于存储一帧原始报文
	//		frame := make([]byte, 0)
	//		// 4.2 处理所有的 Chunk 并更新快照
	//		err = chunkSequence.ProcessAll(deviceId, reader, &frame, t.comm)
	//		if err != nil {
	//			if err == io.EOF {
	//				pkg.Log.Infof("[%s] 客户端断开连接: %s", deviceId, err)
	//				return // 客户端断开连接，优雅地结束
	//			}
	//			pkg.Log.Error(err)
	//			return
	//		}
	//
	//		// 4.3 发射所有的快照
	//		chunkSequence.SnapShotCollection.LaunchALL()
	//		// 4.4 打印原始报文
	//		hexString := ""
	//		for _, b := range frame {
	//			hexString += fmt.Sprintf("%02X", b)
	//		}
	//		pkg.Log.Infof("[%s] %s", deviceId, fmt.Sprintf("%s", hexString))
	//	}
	//}
}
