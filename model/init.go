package model

import (
	config2 "gw22-train-sam/connecter/tcpServer/config"
	"sync"
)

// 确保初始化只执行一次
var once sync.Once

// Init 因为需要等待配置文件加载完毕，所以选择手动初始化
func Init(common *config2.TcpServer, proto *config2.Proto) {
	once.Do(func() {
		// 1. 创建设备快照集
		InitSnapshotCollection(proto, common)
	})
}
