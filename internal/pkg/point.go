package pkg

import (
	"time"
)

// Point 代表发送到数据源的一个数据点
type Point struct {
	DeviceName string                 // 设备名称
	DeviceType string                 // 设备类型
	Field      map[string]interface{} // 字段名称 考虑到point一旦放入chan后状态就会失控，没必要为了一点性能做危险操作
	Ts         time.Time              // 时间戳
}
