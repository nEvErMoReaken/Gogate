package connector

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
)

// Connector 是所有数据源的通用接口
type Connector interface {
	Start() // 启动数据源, 接收一个回调函数，用于将数据源和解析器绑定
	// Ready 函数：懒连接器在接收到新连接时，通知外部准备好数据流
	Ready() <-chan struct{}
	Close() error
	GetDataSource() (interface{}, error)
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
func New(ctx context.Context) (connector Connector, err error) {
	config := pkg.ConfigFromContext(ctx)
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
