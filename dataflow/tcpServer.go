package dataflow

import (
	"fmt"
	"gw22-train-sam/logger"
	"net"
)

func Start() {
	// 1. 监听指定的端口
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		return
	}
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			logger.SugarLogger.Infof("监听程序关闭")
		}
	}(listener)
	logger.SugarLogger.Infof("TCP dataflow listening on port 8080")

	for {
		// 2. 等待客户端连接
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting:", err.Error())
			return
		}

		// 3. 使用 goroutine 处理连接
		go handleConnection(conn)
	}
}
