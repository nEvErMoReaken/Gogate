package model

import "sync"

/*
	对象池实现: 用于减轻 GC 压力
*/

// 创建一个对象池，用于存储和重用 DeviceModel 实例
var deviceModelPool = sync.Pool{
	New: func() interface{} {
		// 当池中没有对象时，创建一个新的 DeviceModel 实例
		return &DeviceModel{
			Fields:       make(map[string]interface{}),
			CachedFields: make(map[string]interface{}),
			StableFields: make(map[string]interface{}),
		}
	},
}

// GetDeviceModel 获取一个 DeviceModel 实例
func GetDeviceModel() *DeviceModel {
	// 从池中获取一个 DeviceModel 实例
	return deviceModelPool.Get().(*DeviceModel)
}

// ReleaseDeviceModel 将 DeviceModel 实例放回池中
func ReleaseDeviceModel(dm *DeviceModel) {
	// 重置 DeviceModel 的字段（可选）
	dm.DeviceName = ""
	dm.DeviceType = ""
	dm.Fields = make(map[string]interface{})
	dm.CachedFields = make(map[string]interface{})
	dm.StableFields = make(map[string]interface{})
	dm.ts = 0

	// 将实例放回池中
	deviceModelPool.Put(dm)
}
