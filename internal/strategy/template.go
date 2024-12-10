package strategy

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
	"go.uber.org/zap"
)

// Template 定义了所有发送策略的通用接口
type Template interface {
	GetType() string      // Step:1 强制要求所有策略都有一个核心结构
	Start(chan pkg.Point) // Step:3 强制要求所有策略都有一个启动方法
}

// FactoryFunc 代表一个发送策略的工厂函数
type FactoryFunc func(context.Context) (Template, error)

// Factories 全局工厂映射，用于注册不同策略类型的构造函数  这里面可能包含了没有启用的数据源
var Factories = make(map[string]FactoryFunc)

// Register 注册一个发送策略
func Register(strategyType string, factory FactoryFunc) {
	Factories[strategyType] = factory
}

// TemplateCollection 代表发送策略集 这里面是所有已启用的数据源
type TemplateCollection map[string]Template

func (c *TemplateCollection) Start(pointChan *pkg.StrategyDataSource) {
	for key, strategy := range *c {
		go strategy.Start(pointChan.PointChan[key])
	}
}

//var SendStrategyMap TemplateCollection

// New 初始化一个发送策略集
var New = func(ctx context.Context) (TemplateCollection, error) {
	SendStrategyMap := make(TemplateCollection)
	// 记录可用的工厂类型
	factoryTypes := make([]string, 0, len(Factories))
	for key := range Factories {
		factoryTypes = append(factoryTypes, key)
	}
	pkg.LoggerFromContext(ctx).Debug("Template Factory:", zap.Strings("Factories", factoryTypes))
	for _, strategyConfig := range pkg.ConfigFromContext(ctx).Strategy {
		if strategyConfig.Enable {
			pkg.LoggerFromContext(ctx).Info(fmt.Sprintf("===正在启动Strategy: %s===", strategyConfig.Type))
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
