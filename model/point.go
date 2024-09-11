package model

import (
	"time"
)

// SendStrategy 定义了所有发送策略的通用接口
type SendStrategy interface {
	Start()
	GetChan() chan *Point // 提供访问 chan 的方法
}

// Point 代表发送到数据源的一个数据点
type Point struct {
	DeviceName *string                 // 设备名称
	DeviceType *string                 // 设备类型
	Field      map[string]*interface{} // 字段名称
	Ts         *time.Time              // 时间戳
}

// PointPackage 数据点的封装，包含一个数据点和发送策略
type PointPackage struct {
	Point    Point
	Strategy SendStrategy // 使用接口解耦
}

// launch 发射数据点
func (pp *PointPackage) launch() {
	// 发送深拷贝的 Point
	pointCopy := DeepCopyPoint(pp.Point)
	pp.Strategy.GetChan() <- &pointCopy
}

// DeepCopyPoint 深拷贝 Point 对象
func DeepCopyPoint(original Point) Point {
	// 复制 DeviceName
	deviceNameCopy := *original.DeviceName

	// 复制 DeviceType
	deviceTypeCopy := *original.DeviceType

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
		DeviceName: &deviceNameCopy,
		DeviceType: &deviceTypeCopy,
		Field:      fieldCopy,
		Ts:         &tsCopy,
	}
}
