package bucketImpl

import (
	"fmt"
	"gw22-train-sam/model"
)

// InfluxDbBucket 实现将数据发布到 InfluxDB 的逻辑
type InfluxDbBucket struct {
	batch model.DeviceBatch
}

func (b *InfluxDbBucket) AddDevice(device model.DeviceModel) {
	b.batch.Put(device)
}

func (b *InfluxDbBucket) Publish() {
	// 模拟将批次数据发布到 InfluxDB
	fmt.Println("Publishing to InfluxDB:")
	for key, deviceList := range b.batch.GetAll() {
		fmt.Printf("InfluxDB -> Device: %s, Type: %s, Data: %v\n", deviceList[key].DeviceName, device.DeviceType, device.Fields)
	}
	b.batch.Clear()
}
