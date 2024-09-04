package model

// Bucket 是一个接口，定义了不同类型 Bucket 的 publish 方法

type Bucket interface {
	AddDevice(device *DeviceSnapshot)
	Start()
}
