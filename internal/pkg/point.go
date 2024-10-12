package pkg

import "time"

// Point 是Parser和Strategy之间传递的数据结构
type Point struct {
	DeviceName string                 // 设备名称
	DeviceType string                 // 设备类型
	Field      map[string]interface{} // 字段名称 考虑到point一旦放入chan后状态就会失控，没必要为了一点性能做危险操作
	Ts         time.Time              // 时间戳
}
