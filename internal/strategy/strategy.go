package strategy

import (
	"gateway/internal/pkg"
)

// Strategy 定义了所有发送策略的通用接口
type Strategy interface {
	Start()
	GetChan() chan pkg.Point // 提供访问 chan 的方法
}

// StFactoryFunc 代表一个发送策略的工厂函数
type StFactoryFunc func(*pkg.StrategyConfig, chan struct{}) Strategy

// Factories 全局工厂映射，用于注册不同策略类型的构造函数
var Factories = make(map[string]StFactoryFunc)

// RegisterStrategy 注册一个发送策略
func RegisterStrategy(strategyType string, factory StFactoryFunc) {
	Factories[strategyType] = factory
}

// MapSendStrategy 代表发送策略集
type MapSendStrategy map[string]Strategy

var SendStrategyMap MapSendStrategy

// InitMapSendStrategy 初始化一个发送策略集
func InitMapSendStrategy(common *pkg.Config, stopChan chan struct{}) {
	SendStrategyMap = make(MapSendStrategy)
	for _, strategyConfig := range common.Strategy {
		if strategyConfig.Enable {
			if factory, exists := Factories[strategyConfig.Type]; exists {
				strategy := factory(&strategyConfig, stopChan)
				SendStrategyMap[strategyConfig.Type] = strategy
			}
		}
	}
}

// GetStrategy 获取一个发送策略
func GetStrategy(strategyType string) Strategy {
	return SendStrategyMap[strategyType]
}

// StartALL 启动所有发送策略
func (m MapSendStrategy) StartALL() {
	for name, strategy := range m {
		pkg.Log.Info("-----正在启动策略：", name)
		go strategy.Start()
	}
}
