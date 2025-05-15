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
// Package sink 定义了 GoGate 数据网关中数据处理的最终环节和数据输出目标。
//
// 本包接收来自 parser 包或 dispatcher 包的结构化数据点 (pkg.PointPackage)，
// 并根据配置将其发送到一个或多个目的地。它也可能包含数据过滤、转换或
// 批量处理等策略。主要职责包括：
//   - 实现与各种外部存储或服务（如 InfluxDB、控制台输出、HTTP API）的集成。
//   - 根据用户配置的过滤规则筛选数据。
//   - 高效地将数据点写入目标系统。
package sink
