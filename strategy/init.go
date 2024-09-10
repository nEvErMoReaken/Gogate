package strategy

import (
	"gw22-train-sam/common"
	"gw22-train-sam/model"
	"sync"
)

// 确保初始化只执行一次
var once sync.Once

// RunStrategy 因为需要等待配置文件加载完毕，所以选择手动初始化
func RunStrategy(common *common.CommonConfig, stopChan chan struct{}) {
	once.Do(func() {
		// 1. 创建数据源集：执行了这一步后，所有配置中启用了的数据源都已经初始化完成并放入了 mapSendStrategy 中
		model.InitMapSendStrategy(common, stopChan)
		// 2. 启动所有发送策略
		model.SendStrategyMap.StartALL()
	})
}
