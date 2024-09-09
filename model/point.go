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
	pp.Strategy.GetChan() <- pp.Point
}
