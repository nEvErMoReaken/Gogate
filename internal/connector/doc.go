/*
Package connector 主要提供了与上游数据源连接相关的代码逻辑。

Template.go中为主接口，负责建立连接、监听数据源中的变化，并使用合适的解析器
将数据进行转换以便后续处理。

可以选择的连接器包括：（可能有些连接器还未实现）

- mqtt

- tcp(server|client)

- udp(server|client)

- sql

- nosql

本包包含：

- Template 接口：定义连接、断开连接和接收数据的方法。
- 工厂函数：为各种数据源创建特定连接器的实例。
- 具体连接器的实现：如 mqtt、tcp、udp 等。

使用示例：

	// 实现 Template 接口
	type MyConnector struct{}

	func (c *MyConnector) Start() error {
	    // 连接逻辑
	}

	func (c *MyConnector) Close() error {
	    // 断开连接逻辑
	}

	// 使用工厂函数将连接器注册
	func init() {
		Register("MyConnector", NewMyConnector)
	}
*/
package connector
