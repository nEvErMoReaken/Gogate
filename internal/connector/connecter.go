package connector

import (
	"fmt"
	"gateway/internal/pkg"
)

// Connector 是所有数据源的通用接口
type Connector interface {
	Listen() error // 建立连接或等待连接
	Close() error
}

// ConnFactoryFunc 代表一个数据源的工厂函数
type ConnFactoryFunc func(*pkg.Config, chan struct{}) Connector

// ConnFactories 全局工厂映射，用于注册不同数据源类型的构造函数
var ConnFactories = make(map[string]ConnFactoryFunc)

// RegisterConn 注册一个数据源
func RegisterConn(connType string, factory ConnFactoryFunc) {
	ConnFactories[connType] = factory
}

// RunConnector 运行指定类型的数据源
func RunConnector(common *pkg.Config, connType string, chDone chan struct{}) error {
	factory, ok := ConnFactories[connType]
	if !ok {
		return fmt.Errorf("未找到数据源类型: %s", connType)
	}
	// 直接调用工厂函数
	err := factory(common, chDone).Listen()
	if err != nil {
		return err
	}
	return nil
}
