package tcpServer

import (
	"bufio"
	"fmt"
	"github.com/spf13/viper"
	"gw22-train-sam/common"
	"gw22-train-sam/model"
	"log"
	"net"
	"time"
)

type ServerModel struct {
	listener           net.Listener              // 监听器
	TcpServerConfig    *TcpServer                // 配置
	ChunkSequence      *ChunkSequence            // 解析序列
	snapShotCollection *model.SnapshotCollection // 快照集合
	chDone             chan struct{}             // 停止通道
}

func init() {
	model.RegisterConn("tcpServer", NewTcpServer)
}

func NewTcpServer(common *common.CommonConfig, v *viper.Viper, chDone chan struct{}) model.Connector {
	// 0. 创建一个新的 ServerModel
	tcpServer := &ServerModel{
		chDone: chDone,
	}

	// 1. 读取配置文件
	TcpServerConfig, err := UnmarshalTCPConfig(v)
	if TcpServerConfig == nil || err != nil {
		log.Fatalf("[tcpServer]加载配置失败: %s\n", err)
	}
	tcpServer.TcpServerConfig = TcpServerConfig
	// 2. 初始化listener
	listener, err := net.Listen("tcp", ":"+TcpServerConfig.TCPServer.Port)
	if err != nil {
		log.Fatalf("[tcpServer]监听程序启动失败: %s\n", err)
	}
	tcpServer.listener = listener

	// 3. 初始化解析序列
	chunks, err := InitChunks(v)
	if err != nil {
		log.Fatalf("[tcpServer]初始化解析序列失败: %s\n", err)
	}
	tcpServer.ChunkSequence = &chunks
	// 4. 初始化所有快照集合
	tcpServer.snapShotCollection = initSnapshotCollection(common, v)
	return tcpServer
}

func (t *ServerModel) Listen() error {
	// 1. 监听指定的端口
	common.Log.Infof("TCP dataflow listening on port %s", t.TcpServerConfig.TCPServer.Port)
	for {
		// 2. 等待客户端连接
		conn, err := t.listener.Accept()
		if err != nil {
			return fmt.Errorf("[tcpServer]与客户端建立连接时发生错误: %s\n", err)
		}
		// 3. 使用 goroutine 处理连接，一个连接对应一个协程
		go t.handleConnection(conn)
	}
}

func (t *ServerModel) Close() error {
	err := t.listener.Close()
	if err != nil {
		return fmt.Errorf("[tcpServer]关闭监听程序失败: %s\n", err)
	}
	return nil
}

// initSnapshotCollection 初始化设备快照的数据点映射
func initSnapshotCollection(common *common.CommonConfig, v *viper.Viper) *model.SnapshotCollection {
	snapshotCollection := make(model.SnapshotCollection)
	// 遍历所有的 PreParsing 和 Parsing 步骤，初始化设备快照
	chunks := v.Sub("TcpProto").Get("chunks").([]interface{})
	for _, chunk := range chunks {
		chunkMap := chunk.(map[string]interface{})
		// 如果没有设备或者类型或者字段，直接跳过
		if chunkMap["device"] == nil || chunkMap["type"] == nil || chunkMap["fields"] == nil {
			continue
		}
		deviceSnapshot := model.GetDeviceSnapshot(chunkMap["device"].(string), chunkMap["type"].(string))
		for _, field := range chunkMap["fields"].([]string) {
			deviceSnapshot.SetField(field, nil)
		}
	}
	// 初始化发送策略
	for _, deviceSnapshot := range snapshotCollection {
		deviceSnapshot.InitPointPackage(common)
	}
	return &snapshotCollection
}

// handleConnection 处理连接, 一个连接对应一个协程
func (t *ServerModel) handleConnection(conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			common.Log.Infof("与" + conn.RemoteAddr().String() + "的连接已关闭")
		}
	}(conn)
	frameContext := make(model.FrameContext)
	// 1. 首先识别远端ip是哪个设备
	remoteAddr := conn.RemoteAddr().String()
	// 2. 连接花名作为变量（如果有）
	if t.TcpServerConfig.TCPServer.IPAlias == nil {
		// 2.1 如果IPAlias为空，则不需要进行识别
		common.Log.Infof("IPAlias为空")
	} else {
		// 2.2 如果IPAlias不为空，放入变量中
		deviceId, exists := t.TcpServerConfig.TCPServer.IPAlias[remoteAddr]
		if !exists {
			common.Log.Errorf("%s 地址不在配置清单中", remoteAddr)
			return
		} else {
			result := new(interface{})
			*result = deviceId
			frameContext["deviceId"] = result
		}
	}
	// 3. 设置超时时间
	err := conn.SetReadDeadline(time.Now().Add(t.TcpServerConfig.TCPServer.Timeout))
	if err != nil {
		common.Log.Infof(conn.RemoteAddr().String() + "超时时间设置失败, 连接关闭")
		return
	}
	// 4. 初始化reader开始读数据
	reader := bufio.NewReader(conn)
	for {
		select {
		case <-t.chDone:
			return
		default:
			// 4.0 Frame 数组，用于存储一帧原始报文
			frame := make([]byte, 0)
			// 4.1 处理所有的 Chunk
			for index, chunk := range t.ChunkSequence.Chunks {
				err := chunk.Process(reader, &frame)
				if err != nil {
					common.Log.Errorf("[handleConnection]解析第 %d 个 Chunk 失败: %s\n", index, err)
				}
			}
			// 4.2 发射所有的快照
			t.snapShotCollection.LaunchALL()
			// 4.3 打印原始报文
			common.Log.Infof("[frame]: %s", frame)
		}
	}
}
