package model

import (
	"github.com/google/uuid"
	"sync"
)

/*
	Batch 实现：为了减轻发送数据源的压力，我们可以将数据缓存到内存中，然后定期发送。
	**Q: 为什么数据库已经有了缓存机制，还需要在程序中实现缓存机制？
	首先: 不是所有的数据库都有缓存机制，InfluxDB 的异步写入机制是通过批量写入来实现的，但是如果是mqtt或者kafka等数据源就没有
	其次: 有些数据源的缓存机制不够灵活，比如InfluxDB的缓存机制是通过设置batch size来实现的，但是有时候我们需要更灵活的控制：，比如可以设置定时发送，或者在数据量达到一定量时发送
    最后：自己写的batch可以实现一些帧过滤机制
*/

// SnapshotBatch 是一个设备快照队列的集合，每个设备快照对应一个队列
type SnapshotBatch struct {
	batch map[uuid.UUID][]*DeviceSnapshot
	mu    sync.Mutex // 用于保护对 batch 的访问
}

// NewSnapshotBatch 创建一个新的设备快照队列集合
func NewSnapshotBatch() *SnapshotBatch {
	return &SnapshotBatch{
		batch: make(map[uuid.UUID][]*DeviceSnapshot),
	}
}

// Put 方法将 deviceModel 添加到对应设备的队列中
func (dp *SnapshotBatch) Put(snapshot DeviceSnapshot) {
	dp.mu.Lock() // 加锁以保护 pool
	defer dp.mu.Unlock()

	// 设备标识符，通常使用设备名称或其他唯一标识符
	deviceKey := snapshot.id

	// 检查该设备的队列是否已存在，不存在则初始化
	if _, exists := dp.batch[deviceKey]; !exists {
		dp.batch[deviceKey] = []*DeviceSnapshot{}
	}

	// 将新的设备数据添加到队列中
	dp.batch[deviceKey] = append(dp.batch[deviceKey], &snapshot)
}

// GetAll 方法返回设备池的副本，并清空设备池
func (dp *SnapshotBatch) GetAll() map[uuid.UUID][]*DeviceSnapshot {
	dp.mu.Lock() // 加锁以保护 pool
	defer dp.mu.Unlock()

	// 获取当前所有设备数据的副本
	copiedBatch := make(map[uuid.UUID][]*DeviceSnapshot)
	for key, devices := range dp.batch {
		copiedBatch[key] = devices
	}

	// 清空设备池
	dp.batch = make(map[uuid.UUID][]*DeviceSnapshot)

	return copiedBatch
}
