package model

import (
	"time"
)

// SendStrategy 定义了所有发送策略的通用接口
type SendStrategy interface {
	Start()
	GetChan() chan Point // 提供访问 chan 的方法
}

// Point 代表发送到数据源的一个数据点
type Point struct {
	DeviceName string                  // 设备名称
	DeviceType string                  // 设备类型
	Field      map[string]*interface{} // 字段名称 考虑到point一旦放入chan后状态就会失控，没必要为了一点性能做危险操作
	Ts         *time.Time              // 时间戳
}

// DeepCopyPoint 深拷贝 Point 对象
func DeepCopyPoint(original Point) Point {

	// 复制 Field，创建一个新的 map 并复制每个 *interface{}
	fieldCopy := make(map[string]*interface{})
	for key, valuePtr := range original.Field {
		if valuePtr != nil {
			valueCopy := *valuePtr
			fieldCopy[key] = &valueCopy
		}
	}

	// 复制时间戳
	tsCopy := *original.Ts

	return Point{
		DeviceName: original.DeviceName,
		DeviceType: original.DeviceType,
		Field:      fieldCopy,
		Ts:         &tsCopy,
	}
}
