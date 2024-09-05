package model

// Connector 是所有数据源的通用接口
type Connector interface {
	Listen() error // 建立连接或等待连接
	Close() error
}

// ConnFactoryFunc 代表一个数据源的工厂函数
type ConnFactoryFunc func() Connector

// ConnFactories 全局工厂映射，用于注册不同数据源类型的构造函数
var ConnFactories = make(map[string]ConnFactoryFunc)

// Register 注册一个数据源
func Register(connType string, factory ConnFactoryFunc) {
	ConnFactories[connType] = factory
}
