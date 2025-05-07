package pkg

import (
	"sync"
	"time"
)

// PointPool 是 Point 对象的对象池
type PointPool struct {
	pool sync.Pool
}

// NewPointPool 创建一个新的 Point 对象池
func NewPointPool() *PointPool {
	return &PointPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &Point{
					Field: make(map[string]interface{}),
				}
			},
		},
	}
}

// Get 从对象池中获取一个 Point 对象
func (p *PointPool) Get() *Point {
	point := p.pool.Get().(*Point)
	point.Ts = time.Now()
	point.Device = ""
	return point
}

// Put 将 Point 对象放回对象池
func (p *PointPool) Put(point *Point) {
	// 清空字段
	for k := range point.Field {
		delete(point.Field, k)
	}
	// 将对象放回池中
	p.pool.Put(point)
}
