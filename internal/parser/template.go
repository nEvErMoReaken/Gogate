package parser

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
	"go.uber.org/zap"
)

// Template 定义一个通用的接口，用于处理各种数据源，并持续维护一个快照集合
type Template interface {
	Start(source *pkg.DataSource, sink *pkg.AggregatorDataSource) // 启动解析器
	GetType() string
}

// FactoryFunc 代表一个发送策略的工厂函数
type FactoryFunc func(ctx context.Context) (Template, error)

// Factories 全局工厂映射，用于注册不同策略类型的构造函数  这里面可能包含了没有启用的数据源
var Factories = make(map[string]FactoryFunc)

// Register 注册一个发送策略
func Register(parserType string, factory FactoryFunc) {
	Factories[parserType] = factory
}

var New = func(ctx context.Context) (Template, error) {
	config := pkg.ConfigFromContext(ctx)
	factory, ok := Factories[config.Parser.Type]
	if !ok {
		return nil, fmt.Errorf("未找到解析器类型: %s", config.Parser.Type)
	}
	factoryTypes := make([]string, 0, len(Factories))
	for key := range Factories {
		factoryTypes = append(factoryTypes, key)
	}
	pkg.LoggerFromContext(ctx).Debug("Strategy Factory:", zap.Strings("Factories", factoryTypes))

	// 1. 初始化脚本模块
	err := LoadAllScripts(ctx, config.Parser.Para["dir"].(string))
	if err != nil {
		return nil, fmt.Errorf("加载脚本失败: %+v ", zap.Error(err))
	}
	pkg.LoggerFromContext(ctx).Info("已加载Byte脚本", zap.Any("ByteScripts", getKeys(ByteScriptFuncCache)))
	pkg.LoggerFromContext(ctx).Info("已加载Json脚本", zap.Any("JsonScripts", getKeys(JsonScriptFuncCache)))
	pkg.LoggerFromContext(ctx).Info(fmt.Sprintf("===正在启动Parser: %s===", config.Parser.Type))

	// 2. 直接调用工厂函数
	parser, err := factory(ctx)
	if err != nil {
		return nil, fmt.Errorf("初始化解析器失败: %v", err)
	}
	return parser, nil
}

// getKeys 获取 map 的所有 key
func getKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
