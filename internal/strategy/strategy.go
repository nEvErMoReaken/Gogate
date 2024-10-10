package strategy

import (
	"gateway/internal/pkg"
	"gateway/logger"
)

// StFactoryFunc 代表一个发送策略的工厂函数
type StFactoryFunc func(*pkg.StrategyConfig, chan struct{}) pkg.SendStrategy

// StrategyFactories 全局工厂映射，用于注册不同策略类型的构造函数
var StrategyFactories = make(map[string]StFactoryFunc)

// RegisterStrategy 注册一个发送策略
func RegisterStrategy(strategyType string, factory StFactoryFunc) {
	StrategyFactories[strategyType] = factory
}

// MapSendStrategy 代表发送策略集
type MapSendStrategy map[string]pkg.SendStrategy

var SendStrategyMap MapSendStrategy

// InitMapSendStrategy 初始化一个发送策略集
func InitMapSendStrategy(common *pkg.Config, stopChan chan struct{}) {
	SendStrategyMap = make(MapSendStrategy)
	for _, strategyConfig := range common.Strategy {
		if strategyConfig.Enable {
			if factory, exists := StrategyFactories[strategyConfig.Type]; exists {
				strategy := factory(&strategyConfig, stopChan)
				SendStrategyMap[strategyConfig.Type] = strategy
			}
		}
	}
}

// GetStrategy 获取一个发送策略
func GetStrategy(strategyType string) pkg.SendStrategy {
	return SendStrategyMap[strategyType]
}

// StartALL 启动所有发送策略
func (m MapSendStrategy) StartALL() {
	for name, strategy := range m {
		logger.Log.Info("-----正在启动策略：", name)
		go strategy.Start()
	}
}
