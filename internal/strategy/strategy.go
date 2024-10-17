package strategy

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
)

// Strategy 定义了所有发送策略的通用接口
type Strategy interface {
	GetCore() Core           // Step:1 强制要求所有策略都有一个核心结构
	GetChan() chan pkg.Point // Step:2 强制要求所有策略都有一个数据通道和放入方法
	Start()                  // Step:3 强制要求所有策略都有一个启动方法
}

// Core 代表一个发送策略的核心结构, 其他策略必须组合它
type Core struct {
	StrategyType string
	PointChan    chan pkg.Point
	ctx          context.Context
}

// FactoryFunc 代表一个发送策略的工厂函数
type FactoryFunc func(context.Context) (Strategy, error)

// Factories 全局工厂映射，用于注册不同策略类型的构造函数  这里面可能包含了没有启用的数据源
var Factories = make(map[string]FactoryFunc)

// Register 注册一个发送策略
func Register(strategyType string, factory FactoryFunc) {
	Factories[strategyType] = factory
}

// MapSendStrategy 代表发送策略集 这里面是所有已启用的数据源
type MapSendStrategy map[string]Strategy

//var SendStrategyMap MapSendStrategy

// New 初始化一个发送策略集
var New = func(ctx context.Context) (MapSendStrategy, error) {
	SendStrategyMap := make(MapSendStrategy)
	for _, strategyConfig := range pkg.ConfigFromContext(ctx).Strategy {
		if strategyConfig.Enable {
			if factory, exists := Factories[strategyConfig.Type]; exists {
				strategy, err := factory(ctx)
				if err != nil {
					return nil, fmt.Errorf("初始化策略 %s 失败: %w", strategyConfig.Type, err)
				}
				SendStrategyMap[strategyConfig.Type] = strategy
			}
		}
	}
	return SendStrategyMap, nil
}
