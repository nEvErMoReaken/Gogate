package tcpServer

import (
	"bufio"
	"gw22-train-sam/logger"
	"io"
	"net"
	"time"
)

// handleConnection 处理连接, 一个连接对应一个协程
func handleConnection(tcpServer *TcpServer, conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			logger.Log.Infof("与" + conn.RemoteAddr().String() + "的连接已关闭")
		}
	}(conn)
	frameContext := make(FrameContext)
	// 首先识别远端ip是哪个设备
	remoteAddr := conn.RemoteAddr().String()
	deviceId, exists := tcpServer.TCPServer.IPAlias[remoteAddr]
	// 作为变量
	result := new(interface{})
	*result = deviceId
	frameContext["deviceId"] = result
	if !exists {
		logger.Log.Errorf("%s 地址不在配置清单中", remoteAddr)
		return
	}
	err := conn.SetReadDeadline(time.Now().Add(tcpServer.TCPServer.Timeout))
	if err != nil {
		logger.Log.Infof(conn.RemoteAddr().String() + "超时时间设置失败, 连接关闭")
		return
	}
	// 初始化reader开始读数据
	reader := bufio.NewReader(conn)
	for {
		// 每一次循环都是一帧数据
		// TODO 解析逻辑
		// Step 1. 获取帧头长度
		frameHeadLen := 1
		// Step 2. 预解析
		data := make([]byte, frameHeadLen)
		// Read n 个字节
		_, err := io.ReadFull(reader, data)
		if err != nil {
			logger.Log.Errorf("[handleConnection]读取帧头失败: %s\n", err)
		}
		HeaderParse(data, &frameContext)
	}
}

// HeaderParse 预解析
func HeaderParse(frameSlice []byte, frameContext *FrameContext) {

}

//// BodyParse 解析帧, 包含了预解析和解析两个阶段
//func BodyParse(tcpServer *TcpServer, frameSlice []byte, frameContext *FrameContext) error {
//	// 预解析
//	for _, section := range tcpServer.Proto.Header.Sections {
//		section.Parse(reader, frameContext)
//	}
//	// 解析
//}
