package pkg

import "sync"

// BytesPool 是一个字节池，用于缓存字节数组
// 减少gc， 减少内存分配
type BytesPool struct {
	pool *sync.Pool
}

// NewBytesPool 创建一个字节池
// size 是字节池的大小
func NewBytesPool(size int) *BytesPool {
	return &BytesPool{
		pool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, size)
			},
		},
	}
}

// Get 从字节池中获取一个字节数组
func (p *BytesPool) Get() []byte {
	return p.pool.Get().([]byte)
}

// Put 将一个字节数组放回字节池
func (p *BytesPool) Put(b []byte) {
	p.pool.Put(b)
}

// BytesPoolInstance 是BytesPool的单例
// 默认大小为65536， 是udp默认最大数据包大小
var BytesPoolInstance = NewBytesPool(65536)
