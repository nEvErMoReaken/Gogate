/*
Package strategy 主要存放下游数据源后续的发送策略相关逻辑。

strategy 接口定义了下游数据源的发送策略，包括：

- influxDB

- iotDB

- mqtt

- tcp

- udp

- sql

- file

本包包含：

- strategy 接口：定义了发送策略的通用接口。

使用示例：

	// 初始化函数，注册自定义策略
	func init() {
		// 注册发送策略
		Register("MyStrategy", NewMyStrategy)
	}

	// 实现 Template 接口
	// GetChan 返回通道
	func (b *MyStrategy) GetChan() chan pkg.Point {

	}

	// Start 启动策略
	func (b *MyStrategy) Start() {
	}
*/
package strategy
