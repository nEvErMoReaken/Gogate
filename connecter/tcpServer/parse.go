package tcpServer

import (
	"bufio"
	"gw22-train-sam/logger"
	"net"
	"time"
)

// FrameContext 每一帧独立的上下文
type FrameContext struct {
	Parameters map[string]string // 单帧中共享的参数
}

// GetParameter 获取参数
func (f *FrameContext) GetParameter(key string) string {
	if value, exists := f.Parameters[key]; exists {
		return value
	}
	return ""
}

// SetParameter 设置参数
func (f *FrameContext) SetParameter(key, value string) {
	f.Parameters[key] = value
}

// handleConnection 处理连接, 一个连接对应一个协程
func handleConnection(tcpServer *TcpServer, conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			logger.Log.Infof("与" + conn.RemoteAddr().String() + "的连接已关闭")
		}
	}(conn)
	// 首先识别远端ip是哪个设备
	remoteAddr := conn.RemoteAddr().String()
	deviceId, exists := tcpServer.TcpServerConfig.TCPServer.IPAlias[remoteAddr]
	if !exists {
		logger.Log.Errorf("%s 地址不在配置清单中", remoteAddr)
		return
	}
	err := conn.SetReadDeadline(time.Now().Add(tcpServer.TcpServerConfig.TCPServer.Timeout))
	if err != nil {
		logger.Log.Infof(conn.RemoteAddr().String() + "超时时间设置失败, 连接关闭")
		return
	}
	// 初始化reader开始读数据
	reader := bufio.NewReader(conn)
	for {
		// 读取一帧数据
		err := parseFrame(reader, &FrameContext{
			Parameters: make(map[string]string),
		})
		if err != nil {
			logger.Log.Errorf("解析帧失败: %s", err)
			return
		}
	}
}

// parseFrame 解析帧, 包含了预解析和解析两个阶段
func parseFrame(reader *bufio.Reader, frameContext *FrameContext) error {
	// 预解析
	// 解析
}
