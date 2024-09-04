package model

// SendStrategy 定义了所有发送策略的通用接口

type SendStrategy interface {
	AddDevice(device *DeviceSnapshot)
	Start()
}
