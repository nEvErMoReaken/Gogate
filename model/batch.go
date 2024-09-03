package model

import (
	"sync"
)

// DeviceBatch 是一个设备队列的集合，每个设备对应一个队列
type DeviceBatch struct {
	pool map[string][]*DeviceModel
	mu   sync.Mutex // 用于保护对 pool 的访问
}

// 初始化单例设备池
var deviceBatch DeviceBatch

// Put 方法将 deviceModel 添加到对应设备的队列中
func (dp *DeviceBatch) Put(deviceModel DeviceModel) {
	dp.mu.Lock() // 加锁以保护 pool
	defer dp.mu.Unlock()

	// 设备标识符，通常使用设备名称或其他唯一标识符
	deviceKey := deviceModel.DeviceName

	// 检查该设备的队列是否已存在，不存在则初始化
	if _, exists := dp.pool[deviceKey]; !exists {
		dp.pool[deviceKey] = []*DeviceModel{}
	}

	// 将新的设备数据添加到队列中
	dp.pool[deviceKey] = append(dp.pool[deviceKey], &deviceModel)
}

// GetAll 方法返回设备池的副本，并清空设备池
func (dp *DeviceBatch) GetAll() map[string][]*DeviceModel {
	dp.mu.Lock() // 加锁以保护 pool
	defer dp.mu.Unlock()

	// 获取当前所有设备数据的副本
	copiedPool := make(map[string][]*DeviceModel)
	for key, devices := range dp.pool {
		copiedPool[key] = devices
	}

	// 清空设备池
	dp.pool = make(map[string][]*DeviceModel)

	return copiedPool
}

func Init() {
	// 初始化 devicePool 或其他初始化逻辑
	deviceBatch = DeviceBatch{
		pool: make(map[string][]*DeviceModel),
	}
}
