package pkg

import (
	"fmt"
	"time"
)

// Point 是Parser和Strategy之间传递的数据结构
type Point struct {
	Device string         // 设备标识
	Field  map[string]any // 字段名称
	Ts     time.Time      // 时间戳
}

// 全局 Point 对象池
var pointPool = NewPointPool()

// NewPoint 创建一个新的 Point 对象
func NewPoint() *Point {
	return pointPool.Get()
}

// Release 释放 Point 对象回对象池
func (p *Point) Release() {
	pointPool.Put(p)
}

// Merge 方法用于合并两个 Point 实例
func (p *Point) Merge(point Point) {
	for k, v := range point.Field {
		p.Field[k] = v
	}
}

// String 方法实现
func (p *Point) String() string {
	// 格式化 Field 映射为字符串
	fieldStr := "{"
	for key, value := range p.Field {
		fieldStr += fmt.Sprintf("%s: %v, ", key, value)
	}
	// 去掉最后的逗号和空格
	if len(p.Field) > 0 {
		fieldStr = fieldStr[:len(fieldStr)-2]
	}
	fieldStr += "}"

	// 格式化整个 Point
	return fmt.Sprintf("Point(DeviceName=%s, Field=%s, Ts=%s)",
		p.Device, fieldStr, p.Ts.Format(time.RFC3339))
}

type Frame2Point struct {
	FrameId string
	Point   map[string]*Point
}

// AggregatePoints 将 rawPointList 中的数据点进行聚合
func AggregatePoints(rawPointList []*Point) (map[string]*Point, error) {
	// 1. 创建一个 map 用于存储聚合后的数据点
	aggregatedPoints := make(map[string]*Point)

	// 2. 遍历 rawPointList 中的每个数据点
	for _, point := range rawPointList {
		// 3. 如果 map 中不存在该设备，则创建一个新点
		if _, ok := aggregatedPoints[point.Device]; !ok {
			aggregatedPoints[point.Device] = &Point{
				Device: point.Device,
				Field:  make(map[string]any),
				Ts:     point.Ts,
			}
		}
		// 4. 将当前数据点的字段添加到聚合点中
		for key, value := range point.Field {
			aggregatedPoints[point.Device].Field[key] = value
		}
	}

	// 5. 返回聚合后的数据点列表
	return aggregatedPoints, nil
}
