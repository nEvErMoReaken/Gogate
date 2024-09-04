package model

import (
	"gw22-train-sam/config"
	"gw22-train-sam/model/strategyImpl"
)

// SendStrategy 定义了所有发送策略的通用接口

type SendStrategy interface {
	AddDevice(device DeviceSnapshot) // 这里不传指针，担心后续还没发送前面的设备就被修改了
	Start()
}

// MapSendStrategy 代表一个发送策略的映射
type MapSendStrategy map[string]SendStrategy

// mapSendStrategy 代表一个发送策略的映射
var mapSendStrategy MapSendStrategy

// InitMapSendStrategy 初始化一个发送策略映射
func InitMapSendStrategy(common *config.Common, stopChan chan struct{}) {
	mapSendStrategy = make(MapSendStrategy)
	for _, strategy := range common.Strategy {
		switch strategy.Type {
		case "influxdb":
			mapSendStrategy[strategy.Type] = strategyImpl.NewInfluxDbStrategy(strategy, stopChan)
			//case "mqtt":
			//	strategyMap[strategy.Name] = strategyImpl.NewMqttStrategy(strategy)
		}
	}
}

// GetStrategy 获取一个发送策略
func GetStrategy(strategyType string) SendStrategy {
	return mapSendStrategy[strategyType]
}
