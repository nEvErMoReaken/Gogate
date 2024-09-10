package tcpServer

import (
	"bufio"
	"gw22-train-sam/common"
	"net"
	"time"
)

// handleConnection 处理连接, 一个连接对应一个协程
func handleConnection(tcpServer *TcpServer, conn net.Conn, sequence *ChunkSequence) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			common.Log.Infof("与" + conn.RemoteAddr().String() + "的连接已关闭")
		}
	}(conn)
	frameContext := make(FrameContext)
	// 1. 首先识别远端ip是哪个设备
	remoteAddr := conn.RemoteAddr().String()
	// 2. 连接花名作为变量（如果有）
	if tcpServer.TCPServer.IPAlias == nil {
		// 2.1 如果IPAlias为空，则不需要进行识别
		common.Log.Infof("IPAlias为空")
	} else {
		// 2.2 如果IPAlias不为空，放入变量中
		deviceId, exists := tcpServer.TCPServer.IPAlias[remoteAddr]
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
	err := conn.SetReadDeadline(time.Now().Add(tcpServer.TCPServer.Timeout))
	if err != nil {
		common.Log.Infof(conn.RemoteAddr().String() + "超时时间设置失败, 连接关闭")
		return
	}
	// 4. 初始化reader开始读数据
	reader := bufio.NewReader(conn)
	for {
		// **** 读取与解析数据流程 ****
		for index, chunk := range sequence.Chunks {
			err := chunk.Process(reader)
			if err != nil {
				common.Log.Errorf("[handleConnection]解析第 %d 个 Chunk 失败: %s\n", index, err)
			}
		}
	}
}
