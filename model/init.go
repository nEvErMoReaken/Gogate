package model

import (
	"github.com/spf13/viper"
	"gw22-train-sam/common"
	"gw22-train-sam/dataSource/byteType/tcpServer"
	"sync"
)

// 确保初始化只执行一次
var once sync.Once

// Init 因为需要等待配置文件加载完毕，所以选择手动初始化
func Init(common *tcpServer.TcpServer, v *viper.Viper, config common.CommonConfig) {
	once.Do(func() {
		// 1. 初始化指定连接器
		factories, err := InitConnFactories(config.Connector.Type, v)
		if err != nil {
			return
		}
		// 1. 创建设备快照集
		//InitSnapshotCollection(common)
	})
}
