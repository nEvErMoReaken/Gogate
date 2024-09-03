package model

import "sync"

/*
	对象池实现: 用于减轻 GC 压力
*/

// 创建一个对象池，用于存储和重用 DeviceSnapshot 实例
var snapshotPool = sync.Pool{
	New: func() interface{} {
		// 当池中没有对象时，创建一个新的 DeviceSnapshot 实例
		return &DeviceSnapshot{
			Fields:       make(map[string]interface{}),
			CachedFields: make(map[string]interface{}),
			StableFields: make(map[string]interface{}),
		}
	},
}

// GetSnapshot 获取一个 DeviceSnapshot 实例
func GetSnapshot() *DeviceSnapshot {
	// 从池中获取一个 DeviceSnapshot 实例
	return snapshotPool.Get().(*DeviceSnapshot)
}

// ReleaseSnapshot 将 DeviceSnapshot 实例放回池中
func ReleaseSnapshot(dm *DeviceSnapshot) {
	// 重置 DeviceSnapshot 的字段（可选）
	dm.DeviceName = ""
	dm.DeviceType = ""
	dm.Fields = make(map[string]interface{})
	dm.CachedFields = make(map[string]interface{})
	dm.StableFields = make(map[string]interface{})
	dm.ts = 0

	// 将实例放回池中
	snapshotPool.Put(dm)
}
