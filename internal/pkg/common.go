package pkg

import (
	"io"
	"time"
)

// Point 是Parser和Strategy之间传递的数据结构
type Point struct {
	DeviceName string                 // 设备名称
	DeviceType string                 // 设备类型
	Field      map[string]interface{} // 字段名称 考虑到point一旦放入chan后状态就会失控，没必要为了一点性能做危险操作
	Ts         time.Time              // 时间戳
}

// DataSource 是Connector和Parser之间传递的数据结构
type DataSource interface {
	Type() string // 用于标识数据源类型
}

// StreamDataSource 实现了 StreamSource 接口
type StreamDataSource struct {
	reader *io.Reader
	writer *io.Writer
}

// NewStreamDataSource 使用指定的 reader 和 writer 创建 StreamDataSource 实例
func NewStreamDataSource(reader *io.Reader, writer *io.Writer) *StreamDataSource {
	return &StreamDataSource{
		reader: reader,
		writer: writer,
	}
}

func (s *StreamDataSource) Type() string {
	return "stream"
}

// ReadFully 阻塞读取直到缓冲区填满
func (s *StreamDataSource) ReadFully(p []byte) (int, error) {
	return io.ReadFull(*s.reader, p)
}

// WriteASAP 立即写入数据
func (s *StreamDataSource) WriteASAP(data []byte) (int, error) {
	return (*s.writer).Write(data)
}

// MessageDataSource 实现了 MessageSource 接口
type MessageDataSource struct {
	dataChan chan []byte
}

func (m *MessageDataSource) Type() string {
	return "message"
}

// ReadOne 从通道中读取一个完整的数据包
func (m *MessageDataSource) ReadOne() ([]byte, error) {
	data, ok := <-m.dataChan
	if !ok {
		return nil, io.EOF
	}
	return data, nil
}

// WriteOne 从通道中读取一个完整的数据包
func (m *MessageDataSource) WriteOne(data []byte) error {
	// 如果通道已关闭，返回 EOF
	if m.dataChan == nil {
		return io.EOF
	}
	m.dataChan <- data
	return nil
}
