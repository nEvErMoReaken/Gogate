package tcpServer

import (
	"fmt"
	"github.com/spf13/viper"
	"gw22-train-sam/common"
	"gw22-train-sam/model"
	"log"
	"net"
)

type ServerModel struct {
	listener           net.Listener              // 监听器
	TcpServerConfig    *TcpServer                // 配置
	ChunkSequence      *ChunkSequence            // 解析序列
	snapShotCollection *model.SnapshotCollection // 快照集合
}

func init() {
	model.Register("tcpServer", NewTcpServer)
}

func NewTcpServer(common *common.CommonConfig, v *viper.Viper) model.Connector {
	// 0. 创建一个新的 ServerModel
	tcpServer := &ServerModel{}

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
	initSnapshotCollection(common, v)
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
		go handleConnection(t.TcpServerConfig, conn, t.ChunkSequence)
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
		//deviceSnapshot := model.GetDeviceSnapshot(chunk.(map[string]interface{}), step.To.Type)
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
