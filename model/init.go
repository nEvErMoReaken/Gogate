package model

import (
	"gw22-train-sam/config"
	"sync"
)

// 确保初始化只执行一次
var once sync.Once

// Init 因为需要等待配置文件加载完毕，所以选择手动初始化
func Init(common *config.Common, proto *config.Proto, stopChan chan struct{}) {
	once.Do(func() {
		// 1. 创建设备快照集
		InitSnapshotCollection(proto, common)
	})
}
