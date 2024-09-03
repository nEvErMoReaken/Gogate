package eventbus

import (
	"fmt"
	"gw22-train-sam/model"
	"sync"
	"time"
)

// DevicePool 是一个设备队列的集合，每个设备对应一个队列
type DevicePool struct {
	pool map[string][]*model.DeviceModel
	mu   sync.Mutex // 用于保护对 pool 的访问
}

// 初始化单例设备池
var devicePool DevicePool

// Put 方法将 deviceModel 添加到对应设备的队列中
func (dp *DevicePool) Put(deviceModel model.DeviceModel) {
	dp.mu.Lock() // 加锁以保护 pool
	defer dp.mu.Unlock()

	// 设备标识符，通常使用设备名称或其他唯一标识符
	deviceKey := deviceModel.DeviceName

	// 检查该设备的队列是否已存在，不存在则初始化
	if _, exists := dp.pool[deviceKey]; !exists {
		dp.pool[deviceKey] = []*model.DeviceModel{}
	}

	// 将新的设备数据添加到队列中
	dp.pool[deviceKey] = append(dp.pool[deviceKey], &deviceModel)
}

// GetAll 方法返回设备池的副本，并清空设备池
func (dp *DevicePool) GetAll() map[string][]*model.DeviceModel {
	dp.mu.Lock() // 加锁以保护 pool
	defer dp.mu.Unlock()

	// 获取当前所有设备数据的副本
	copiedPool := make(map[string][]*model.DeviceModel)
	for key, devices := range dp.pool {
		copiedPool[key] = devices
	}

	// 清空设备池
	dp.pool = make(map[string][]*model.DeviceModel)

	return copiedPool
}

func Init() {
	// 初始化 devicePool 或其他初始化逻辑
	devicePool = DevicePool{
		pool: make(map[string][]*model.DeviceModel),
	}
	InitBus()
}

// PublishToEventBus 每隔2秒将设备池的数据发布到 EventBus
func PublishToEventBus() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 获取并清空设备池
		allDevices := devicePool.GetAll()

		// 将设备数据发布到 EventBus 中
		for deviceKey, devices := range allDevices {
			bus.Publish(deviceKey, devices) // 发布事件到 EventBus 中，设备的 key 作为事件名
		}

		// 你也可以在此处进行日志记录，或者其他操作
		fmt.Println("Published to EventBus")
	}
}
