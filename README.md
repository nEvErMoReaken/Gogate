## Intro

一个零代码、完全依赖配置驱动的数据网关。

## Connector 连接器

支持多种类型的数据源

- Json类型数据源
    - Mqtt
    - Kafka
- Byte类型数据源
    - TcpServer (作为TcpServer接收报文)
    - TcpClient (作为TcpClient主动fetch报文)
    - UdpServer (作为TcpServer接收报文)
    - UdpClient (作为UdpClient主动fetch报文)
- Protobuf (?)

## 解析

不同数据源解析逻辑不同，共有思路是通过读取配置的解析流程，灵活的对不同协议进行解析。

以Tcp Byte为例：

特点：Tcp协议需要指定包的起始与末尾，需要有一份配置文件指示报文中的偏移量和解码逻辑。

实现思路：

1. 首先完整的报文可以拆分为多个堆叠的Chunk组成的ChunkSequence，每个Chunk中可以有多个Section。会在后续流程中顺序解析。

- FixedLengthChunk （定长Chunk） 用于可以确定整个报文长度的协议
- ConditionalChunk （条件Chunk） 用于需要通过上下文得知协议类型后的动态Chunk
- DelimiterChunk (分隔符Chunk) 用于通过固定的帧头帧尾分隔符指示报文结尾的Chunk
- DynamicChunk (不定长Chunk) 以上Chunk所需信息都没有，只能完全依赖Section解析的动态Chunk（性能有所降低）

2. 每个Chunk中理应维护一个Section数组，一个Section为一个最小的不可分割的部分，它指示了几个字节长度的解析生命周期的完整逻辑，结构如下：

```go
// SectionConfig 定义
type SectionConfig struct {
	From     FromConfig `mapstructure:"from"` // 偏移量
	Decoding Decoding   `mapstructure:"decoding"`
	For      ForConfig  `mapstructure:"for"`  // 赋值变量
	To       ToConfig   `mapstructure:"to"`   // 字段转换配置
	Desc     string     `mapstructure:"desc"` // 字段说明
}
```
- From
```go
type FromConfig struct {
	Byte   int         `mapstructure:"byte"` // 字节长度
	Repeat interface{} `mapstructure:"repeat"` // 重复次数
}
```




通过使用yaegi库允许使用者编写动态脚本，并在配置中解析动态脚本实现纯配置的报文的动态解析。







## 更新快照

通过构建DeviceSnapshot物模型快照实现上下游发送数据源的解耦。后续的数据发送策略完全基于物模型。理论上只要实现数据->物模型的方法可以兼容各种上游数据类型。

DeviceSnapshot是懒汉式创建的，一旦创建，后续操作仅更新其中值，减少内存开销。

## 