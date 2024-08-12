package dataflow

import (
	"bufio"
	"encoding/hex"
	"errors"
	"gw22-train-sam/logger"
	"net"
	"time"
)

func handleConnection(conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			logger.SugarLogger.Infof("与" + conn.RemoteAddr().String() + "的连接已关闭")
		}
	}(conn)

	// 设置读写超时
	err := conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	if err != nil {
		logger.SugarLogger.Infof(conn.RemoteAddr().String() + "超时时间设置失败, 连接关闭")
		return
	}

	reader := bufio.NewReader(conn)
	for {
		frame, err := reader.ReadByte()
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				logger.SugarLogger.Error("Read timeout:", err)
				return
			}
			logger.SugarLogger.Error("Error reading:", err)
			return
		}

		logger.SugarLogger.Infof("len:%d, efefefef%sfefefefe", len(frame), hex.EncodeToString(frame))

	}
}
