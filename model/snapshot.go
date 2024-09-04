package model

import (
	"github.com/google/uuid"
	"time"
)

// DeviceSnapshot 代表一个设备的物模型在某时刻的快照
type DeviceSnapshot struct {
	id         uuid.UUID                 // 设备 ID
	DeviceName string                    // 设备名称，例如 "vobc0001.abc"
	DeviceType string                    // 设备类型，例如 "vobc.info"
	Fields     map[string]interface{}    // 字段存储，key 为字段名称，value 为字段值
	Strategies map[string][]SendStrategy // 发送策略
	Ts         time.Time                 // 时间戳
}

// NewSnapshot 创建一个新的设备快照，尽量避免直接使用 DeviceSnapshot{} 创建
func NewSnapshot(name, deviceType string, ts time.Time) *DeviceSnapshot {
	// 生成一个新的 UUID
	newID, err := uuid.NewUUID()
	if err != nil {
		// 如果生成 UUID 失败，可以选择返回错误或处理错误，这里简单处理为返回 nil
		return nil
	}
	return &DeviceSnapshot{
		id:         newID,
		DeviceName: name,
		DeviceType: deviceType,
		Fields:     make(map[string]interface{}),
		Ts:         ts,
	}
}

// SetField 设置或更新字段值
func (dm *DeviceSnapshot) SetField(fieldName string, value interface{}) {
	dm.Fields[fieldName] = value
}

// GetField 获取字段值
func (dm *DeviceSnapshot) GetField(fieldName string) (interface{}, bool) {
	if value, exists := dm.Fields[fieldName]; exists {
		return value, true
	}

	return nil, false
}

// Equal 方法用于比较两个 DeviceSnapshot 是否是相同设备
// 两个 DeviceSnapshot 相等的条件是 DeviceName 和 DeviceType 都相同
func (dm *DeviceSnapshot) Equal(other *DeviceSnapshot) bool {
	if dm == nil || other == nil {
		return false
	}
	return dm.DeviceName == other.DeviceName && dm.DeviceType == other.DeviceType
}
