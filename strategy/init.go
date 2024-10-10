package strategy

import (
	"gateway/internal/pkg"
	"gateway/logger"
	"gateway/model"
)

// RunStrategy 因为需要等待配置文件加载完毕，所以选择手动初始化
func RunStrategy(comm *pkg.Config, stopChan chan struct{}) {
	// 0. 区分注册和启用。方便测试配置时候开多个数据源又不用删除已有配置
	logger.Log.Infof("已注册的策略有：%+v", model.StrategyFactories)
	// 1. 创建数据源集：执行了这一步后，所有配置中启用了的数据源都已经初始化完成并放入了 mapSendStrategy 中
	model.InitMapSendStrategy(comm, stopChan)
	logger.Log.Infof("已启用的策略有：%+v", model.SendStrategyMap)
	// 2. 启动所有发送策略
	model.SendStrategyMap.StartALL()
}
