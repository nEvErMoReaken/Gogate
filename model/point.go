package model

import "time"

// Point 代表发送到数据源的一个数据点 数据源所有的数据点都是 Point 类型
// Point 和数据源是一对一的，即一个数据源对应一个 Point 类型
// 后续Point要放在chan中，所以PointPackage用于封装Point和发送策略列表
type Point struct {
	DeviceName string                 // 设备名称
	DeviceType string                 // 设备类型
	Field      map[string]interface{} // 字段名称
	Ts         time.Time              // 时间戳
}

// PointPackage 数据点的高级封装，包含多个数据点和发送策略列表
type PointPackage struct {
	Point    Point
	Strategy SendStrategy // 发送策略列表
}

// merge 合并两个数据点
func (pp *PointPackage) merge(key string, value interface{}) {
	pp.Point.Field[key] = value
}

// launch 发射数据点
func (pp *PointPackage) launch() {
	pp.Strategy.GetChan() <- pp.Point
}
