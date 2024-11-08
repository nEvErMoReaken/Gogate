package connector

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
	"go.uber.org/zap"
)

// Connector 是所有数据源的通用接口
type Connector interface {
	Start()                         // 启动连接器
	SinkType() string               // 返回流出数据源类型
	SetSink(source *pkg.DataSource) // 设置流出数据源
}

// FactoryFunc 代表一个数据源的工厂函数, 返回数据源和连接器实例
type FactoryFunc func(ctx context.Context) (connector Connector, err error)

// Factories 全局工厂映射，用于注册不同数据源类型的构造函数
var Factories = make(map[string]FactoryFunc)

// Register 注册一个数据源
func Register(connType string, factory FactoryFunc) {
	Factories[connType] = factory
}

// New 运行指定类型的数据源
var New = func(ctx context.Context) (connector Connector, err error) {
	config := pkg.ConfigFromContext(ctx)
	// 记录可用的工厂类型
	factoryTypes := make([]string, 0, len(Factories))
	for key := range Factories {
		factoryTypes = append(factoryTypes, key)
	}
	pkg.LoggerFromContext(ctx).Debug("Connector Factory:", zap.Strings("Factories", factoryTypes))
	pkg.LoggerFromContext(ctx).Debug(fmt.Sprintf("===正在启动Connector: %s===", config.Connector.Type))
	factory, ok := Factories[config.Connector.Type]
	if !ok {
		return nil, fmt.Errorf("未找到数据源类型: %s", config.Connector.Type)
	}
	// 直接调用工厂函数
	connector, err = factory(ctx)
	if err != nil {
		return nil, fmt.Errorf("初始化数据源失败: %v", err)
	}
	return connector, nil
}
