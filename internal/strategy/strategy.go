package strategy

import (
	"context"
	"gateway/internal/pkg"
	"go.uber.org/zap"
)

// Strategy 定义了所有发送策略的通用接口
type Strategy interface {
	Start()
	GetChan() chan pkg.Point // 提供访问 chan 的方法
}

// RunStrategy 因为需要等待配置文件加载完毕，所以选择手动初始化
func RunStrategy(ctx context.Context) {
	log := pkg.LoggerFromContext(ctx)
	// 0. 区分注册和启用。方便测试配置时候开多个数据源又不用删除已有配置
	log.Info("已注册的策略有", zap.Any("Factories", Factories))
	// 1. 创建数据源集：执行了这一步后，所有配置中启用了的数据源都已经初始化完成并放入了 mapSendStrategy 中
	InitMapSendStrategy(ctx)
	log.Info("已启用的策略有", zap.Any("SendStrategyMap", SendStrategyMap))
	// 2. 启动所有发送策略
	SendStrategyMap.StartALL()
}

// StFactoryFunc 代表一个发送策略的工厂函数
type StFactoryFunc func(context.Context) Strategy

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
func InitMapSendStrategy(ctx context.Context) {
	SendStrategyMap = make(MapSendStrategy)
	for _, strategyConfig := range pkg.ConfigFromContext(ctx).Strategy {
		if strategyConfig.Enable {
			if factory, exists := Factories[strategyConfig.Type]; exists {
				strategy := factory(ctx)
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
	for _, strategy := range m {
		go strategy.Start()
	}
}
