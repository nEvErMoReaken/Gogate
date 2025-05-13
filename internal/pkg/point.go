package pkg

import (
	"fmt"
	"sync"
	"time"
)

// Point 是Parser和Strategy之间传递的数据结构
type Point struct {
	Tag   map[string]any // 设备标识
	Field map[string]any // 字段名称
}

// PointPackage 准备好待发送的point
type PointPackage struct {
	FrameId string // 帧ID , 用于追踪接收和处理的整个生命周期
	Points  []*Point
	Ts      time.Time // 本帧时间
}

// Merge 方法用于合并两个 Point 实例
func (p *Point) Merge(point Point) {
	for k, v := range point.Field {
		p.Field[k] = v
	}
}

// Reset 方法用于重置 Point 实例
func (p *Point) Reset() {
	for k := range p.Field {
		delete(p.Field, k)
	}
	for k := range p.Tag {
		delete(p.Tag, k)
	}
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
	return fmt.Sprintf("Point(Field=%s,Tag=%s)", fieldStr, p.Tag)
}

// PointPool 是 Point 对象的对象池
type PointPool struct {
	pool sync.Pool
}

var PointPoolInstance = NewPointPool()

// NewPointPool 创建一个新的 Point 对象池
func NewPointPool() *PointPool {
	return &PointPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &Point{
					Tag:   make(map[string]any),
					Field: make(map[string]any),
				}
			},
		},
	}
}

// Get 从对象池中获取一个 Point 对象
func (p *PointPool) Get() *Point {
	point := p.pool.Get().(*Point)
	return point
}

// Put 将 Point 对象放回对象池
func (p *PointPool) Put(point *Point) {

	point.Reset()
	// 将对象放回池中
	p.pool.Put(point)
}
