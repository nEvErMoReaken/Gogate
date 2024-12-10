package pkg

import (
	"fmt"
	"io"
	"time"
)

// DataSourceType 使用 const 定义枚举值
type DataSourceType int

const (
	StreamType DataSourceType = iota
	MessageType
	StrategyType
	AggregatorType
)

func (d DataSourceType) String() string {
	switch d {
	case StreamType:
		return "stream"
	case MessageType:
		return "message"
	case StrategyType:
		return "strategy"
	case AggregatorType:
		return "aggregator"
	default:
		return "Unknown"
	}
}

// Point 是Parser和Strategy之间传递的数据结构
type Point struct {
	Device string                 // 设备标识
	Field  map[string]interface{} // 字段名称
	Ts     time.Time              // 时间戳
}

// Merge 方法用于合并两个 Point 实例
func (p *Point) Merge(point Point) {
	for k, v := range point.Field {
		p.Field[k] = v
	}
}

// DataSource 是Connector和Parser之间传递的数据结构
type DataSource interface {
	Type() DataSourceType // 用于标识数据源类型

}

// StreamDataSource 实现了 StreamSource 接口
type StreamDataSource struct {
	MetaData map[string]string
	Reader   io.Reader
	Writer   io.Writer
}

// String 方法实现
func (p *Point) String() string {
	// 格式化 Field 映射为字符串
	fieldStr := "{"
	for key, value := range p.Field {
		fieldStr += fmt.Sprintf("%s: %v, ", key, value)
	}
	// 去掉最后的逗号和空格
	if len(p.Field) > 0 {
		fieldStr = fieldStr[:len(fieldStr)-2]
	}
	fieldStr += "}"

	// 格式化整个 Point
	return fmt.Sprintf("Point(DeviceName=%s, Field=%s, Ts=%s)",
		p.Device, fieldStr, p.Ts.Format(time.RFC3339))
}

// NewStreamDataSource 使用指定的 reader 和 writer 创建 StreamDataSource 实例
func NewStreamDataSource() *StreamDataSource {
	reader, writer := io.Pipe()
	return &StreamDataSource{
		Reader: reader,
		Writer: writer,
	}
}

// NewMessageDataSource 创建一个 MessageDataSource 实例
func NewMessageDataSource() *MessageDataSource {
	return &MessageDataSource{
		DataChan: make(chan []byte, 200),
		MetaData: make(map[string]string),
	}
}
func (s *StreamDataSource) Type() DataSourceType {
	return StreamType
}

// ReadFully 阻塞读取直到缓冲区填满
func (s *StreamDataSource) ReadFully(p []byte) (int, error) {
	return io.ReadFull(s.Reader, p)
}

// Read 立即读取
func (s *StreamDataSource) Read(p []byte) (int, error) {
	return (s.Reader).Read(p)
}

// WriteASAP 立即写入数据
func (s *StreamDataSource) WriteASAP(data []byte) (int, error) {
	return (s.Writer).Write(data)
}

// MessageDataSource 实现了 MessageSource 接口
type MessageDataSource struct {
	DataChan chan []byte
	MetaData map[string]string
}

func (m *MessageDataSource) Type() DataSourceType {
	return MessageType
}

// ReadOne 从通道中读取一个完整的数据包
func (m *MessageDataSource) ReadOne() ([]byte, error) {
	data, ok := <-m.DataChan
	if !ok {
		return nil, io.EOF
	}
	return data, nil
}

// WriteOne 从通道中读取一个完整的数据包
func (m *MessageDataSource) WriteOne(data []byte) error {
	// 如果通道已关闭，返回 EOF
	if m.DataChan == nil {
		return io.EOF
	}
	m.DataChan <- data
	return nil
}

type AggregatorDataSource struct {
	PointChan chan Point
	EndChan   chan struct{}
}

func (a *AggregatorDataSource) Type() DataSourceType {
	return AggregatorType
}

type StrategyDataSource struct {
	PointChan map[string]chan Point
}

func (p *StrategyDataSource) Type() DataSourceType {
	return StrategyType
}
