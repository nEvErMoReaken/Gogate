package dataflow

import (
	"bufio"
	"encoding/hex"
	"errors"
	"github.com/spf13/viper"
	"gw22-train-sam/logger"
	"net"
	"time"
)

func handleConnection(conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			logger.Log.Infof("与" + conn.RemoteAddr().String() + "的连接已关闭")
		}
	}(conn)
	// 首先识别远端ip是哪个设备
	remoteAddr := conn.RemoteAddr().String()
	nameMap := viper.GetStringMap("nameMap")

	deviceId, exists := nameMap[remoteAddr]
	if !exists {
		logger.Log.Errorf("%s 地址不在配置清单中", remoteAddr)
		return
	}

	// 设置读写超时
	err := conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	if err != nil {
		logger.Log.Infof(conn.RemoteAddr().String() + "超时时间设置失败, 连接关闭")
		return
	}
	// 初始化reader开始读数据
	reader := bufio.NewReader(conn)
	for {
		reader.Peek()
		frame, err := reader.ReadByte()
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				logger.Log.Error("Read timeout:", err)
				return
			}
			logger.Log.Error("Error reading:", err)
			return
		}

		logger.Log.Infof("len:%d, efefefef%sfefefefe", len(frame), hex.EncodeToString(frame))

	}
}
