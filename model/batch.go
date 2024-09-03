package model

import (
	"sync"
)

// SnapshotBatch 是一个设备快照队列的集合，每个设备快照对应一个队列
type SnapshotBatch struct {
	pool map[string][]*DeviceSnapshot
	mu   sync.Mutex // 用于保护对 pool 的访问
}

// 初始化单例设备池
var snapshotBatch SnapshotBatch

// Put 方法将 deviceModel 添加到对应设备的队列中
func (dp *SnapshotBatch) Put(snapshot DeviceSnapshot) {
	dp.mu.Lock() // 加锁以保护 pool
	defer dp.mu.Unlock()

	// 设备标识符，通常使用设备名称或其他唯一标识符
	deviceKey := snapshot.DeviceName

	// 检查该设备的队列是否已存在，不存在则初始化
	if _, exists := dp.pool[deviceKey]; !exists {
		dp.pool[deviceKey] = []*DeviceSnapshot{}
	}

	// 将新的设备数据添加到队列中
	dp.pool[deviceKey] = append(dp.pool[deviceKey], &snapshot)
}

// GetAll 方法返回设备池的副本，并清空设备池
func (dp *SnapshotBatch) GetAll() map[string][]*DeviceSnapshot {
	dp.mu.Lock() // 加锁以保护 pool
	defer dp.mu.Unlock()

	// 获取当前所有设备数据的副本
	copiedPool := make(map[string][]*DeviceSnapshot)
	for key, devices := range dp.pool {
		copiedPool[key] = devices
	}

	// 清空设备池
	dp.pool = make(map[string][]*DeviceSnapshot)

	return copiedPool
}
