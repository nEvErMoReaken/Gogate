package model

import (
	"fmt"
	"github.com/spf13/viper"
)

// Connector 是所有数据源的通用接口
type Connector interface {
	Listen() error // 建立连接或等待连接
	Close() error
}

// ConnFactoryFunc 代表一个数据源的工厂函数
type ConnFactoryFunc func(*viper.Viper) Connector

// ConnFactories 全局工厂映射，用于注册不同数据源类型的构造函数
var ConnFactories = make(map[string]ConnFactoryFunc)

// Register 注册一个数据源
func Register(connType string, factory ConnFactoryFunc) {
	ConnFactories[connType] = factory
}

// RunConnector 运行指定类型的数据源
func RunConnector(connType string, v *viper.Viper) error {
	factory, ok := ConnFactories[connType]
	if !ok {
		return fmt.Errorf("未找到数据源类型: %s", connType)
	}
	// 直接调用工厂函数
	err := factory(v).Listen()
	if err != nil {
		return err
	}
	return nil
}
