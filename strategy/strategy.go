package strategy

import (
	"gw22-train-sam/dataSource/byteType/tcpServer"
)

// StFactoryFunc 代表一个发送策略的工厂函数
type StFactoryFunc func(tcpServer.StrategyConfig, chan struct{}) SendStrategy

// 全局工厂映射，用于注册不同策略类型的构造函数
var strategyFactories = make(map[string]StFactoryFunc)

// Register 注册一个发送策略
func Register(strategyType string, factory StFactoryFunc) {
	strategyFactories[strategyType] = factory
}

// MapSendStrategy 代表发送策略集
type MapSendStrategy map[string]SendStrategy

var mapSendStrategy MapSendStrategy

// InitMapSendStrategy 初始化一个发送策略集
func InitMapSendStrategy(common *tcpServer.TcpServer, stopChan chan struct{}) {
	mapSendStrategy = make(MapSendStrategy)
	for _, strategyConfig := range common.Strategy {
		if strategyConfig.Enable {
			if factory, exists := strategyFactories[strategyConfig.Type]; exists {
				strategy := factory(strategyConfig, stopChan)
				mapSendStrategy[strategyConfig.Type] = strategy
			}
		}
	}
}

// GetStrategy 获取一个发送策略
func GetStrategy(strategyType string) SendStrategy {
	return mapSendStrategy[strategyType]
}

// StartALL 启动所有发送策略
func (m MapSendStrategy) StartALL() {
	for _, strategy := range m {
		go strategy.Start()
	}
}
