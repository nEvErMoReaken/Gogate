package connector

import (
	"bufio"
	"context"
	"fmt"
	"gateway/internal/parser"
	"gateway/internal/pkg"
	"gateway/util"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"io"
	"log"
	"net"
	"time"
)

// TcpServerConnector Connector的TcpServer版本实现
type TcpServerConnector struct {
	ctx context.Context
	listener net.Listener
}

func init() {
	Register("tcpServer", NewTcpServer)
}

func NewTcpServer(ctx context.Context) ([]Connector,error) {
	// 0. 创建一个新的 TcpServerConnector
	tcpServer := &TcpServerConnector{
		ctx: ctx,
	}

	// 1. 读取配置文件
	TcpServerConfig, err := pkg.UnmarshalTCPConfig(v)
	//if TcpServerConfig == nil || err != nil {
	//	log.Fatalf("[tcpServer]加载配置失败: %s\n", err)
	//}
	//tcpServer.TcpServerConfig = TcpServerConfig
	// 2. 初始化listener
	listener, err := net.Listen("tcp", ":"+TcpServerConfig.TCPServer.Port)
	if err != nil {
		log.Fatalf("[tcpServer]监听程序启动失败: %s\n", err)
	}


	return tcpServer
}
func (t *TcpServerConnector) GetDataSource() (interface{}, error) {
	return t.
}
func (t *TcpServerConnector) Start()  {
	// 1. 监听指定的端口
	pkg.LoggerFromContext(t.ctx).Info("TCPServer listening on port: " + zap.String("port", t.TcpServerConfig.TCPServer.Port))
	for {
		// 2. 等待客户端连接
		conn, err := t.listener.Accept()
		if err != nil {
			util.ErrChanFromContext(t.ctx) <- fmt.Errorf("[tcpServer]接受连接失败: %s\n", err)
		}
		// 3. 使用 goroutine 处理连接，一个连接对应一个协程
		pkg.LoggerFromContext(t.ctx).Info("与 %s 建立连接", zap.String("remote", conn.RemoteAddr().String()))
		chunks, err := parser.InitChunks(t.v, t.TcpServerConfig.ProtoFile)
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

// @Deprecated initSnapshotCollection 初始化设备快照的数据点映射
func initSnapshotCollection(comm *pkg.Config, v *viper.Viper, protoFile string) *pkg.SnapshotCollection {
	snapshotCollection := make(pkg.SnapshotCollection)
	// 遍历所有的 PreParsing 和 Parsing 步骤，初始化设备快照
	chunks := v.Sub(protoFile).Get("chunks").([]interface{})
	for _, chunk := range chunks {
		chunkMap := chunk.(map[string]interface{})
		// 遍历Sections
		sections := chunkMap["sections"].([]interface{})
		for _, section := range sections {
			sectionMap := section.(map[string]interface{})
			pkg.Log.Infof("sectionMap: %+v", sectionMap) // sectionMap: map[decoding:map[method:Decode8BToInt] desc:帧长度 长度由字节69开始计算 from:map[byte:1] to:map[device:vobc fields:[RIOM_sta_1 RIOM_sta_2 RIOM_sta_3 RIOM_sta_4 RIOM_sta_5 RIOM_sta_6 RIOM_sta_7 RIOM_sta_8] type:vobc.info]]
			// 如果没有设备或者类型或者字段，直接跳过
			if sectionMap["to"] == nil {
				continue
			}
			toMap := sectionMap["to"].(map[string]interface{})
			if toMap["device"] == nil || toMap["type"] == nil || toMap["fields"] == nil {
				continue
			}
			deviceSnapshot := snapshotCollection.GetDeviceSnapshot(toMap["device"].(string), toMap["type"].(string))
			pkg.Log.Debugf("snapshotCollection: %+v", snapshotCollection)
			//for _, field := range toMap["fields"].([]interface{}) {
			//	//deviceSnapshot.SetField(field.(string), nil)
			//}
			pkg.Log.Debugf("deviceSnapshot: %+v", deviceSnapshot)
		}
	}
	// 初始化发送策略
	//for _, deviceSnapshot := range snapshotCollection {
	//	//deviceSnapshot.InitDataSink(comm)
	//	//common.Log.Debugf("初始化PointMap成功: %+v", deviceSnapshot.PointMap["influxdb"])
	//}
	pkg.Log.Debugf("初始化设备快照成功: %+v", snapshotCollection)
	return &snapshotCollection
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
	for {
		select {
		case <-t.ChDone:
			return
		default:
			// 4.1 Frame 数组，用于存储一帧原始报文
			frame := make([]byte, 0)
			// 4.2 处理所有的 Chunk 并更新快照
			err = chunkSequence.ProcessAll(deviceId, reader, &frame, t.comm)
			if err != nil {
				if err == io.EOF {
					pkg.Log.Infof("[%s] 客户端断开连接: %s", deviceId, err)
					return // 客户端断开连接，优雅地结束
				}
				pkg.Log.Error(err)
				return
			}

			// 4.3 发射所有的快照
			chunkSequence.SnapShotCollection.LaunchALL()
			// 4.4 打印原始报文
			hexString := ""
			for _, b := range frame {
				hexString += fmt.Sprintf("%02X", b)
			}
			pkg.Log.Infof("[%s] %s", deviceId, fmt.Sprintf("%s", hexString))
		}
	}
}
