package model

import "time"

// Point 代表发送到数据源的一个数据点
type Point struct {
	DeviceName string                 // 设备名称
	DeviceType string                 // 设备类型
	Field      map[string]interface{} // 字段名称
	Ts         time.Time              // 时间戳
}

// NewPoint 创建一个新的 Point
func NewPoint(deviceName, deviceType string, fields map[string]interface{}, timestamp time.Time) *Point {
	return &Point{
		DeviceName: deviceName,
		DeviceType: deviceType,
		Field:      fields,
		Ts:         timestamp,
	}
}
