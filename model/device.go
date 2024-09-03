package model

// DeviceModel 代表一个设备的物模型
type DeviceModel struct {
	id           string                 // 设备 ID
	DeviceName   string                 // 设备名称，例如 "vobc0001.abc"
	DeviceType   string                 // 设备类型，例如 "vobc.info"
	Fields       map[string]interface{} // 动态字段存储，key 为字段名称，value 为字段值
	CachedFields map[string]interface{} // 需要缓存的字段存储，key 为字段名称，value 为字段值
	StableFields map[string]interface{} // 稳定字段存储，key 为字段名称，value 为字段值
	ts           int64                  // 时间戳
}

// NewDeviceModel 创建一个新的设备物模型
func NewDeviceModel(name, deviceType string) *DeviceModel {

	return &DeviceModel{
		DeviceName:   name,
		DeviceType:   deviceType,
		Fields:       make(map[string]interface{}),
		CachedFields: make(map[string]interface{}),
		StableFields: make(map[string]interface{}),
	}
}

// SetField 设置或更新字段值
func (dm *DeviceModel) SetField(fieldName string, value interface{}, fieldType string) {
	if fieldType == "cached" {
		dm.CachedFields[fieldName] = value
	} else if fieldType == "stable" {
		dm.StableFields[fieldName] = value
	} else {
		dm.Fields[fieldName] = value
	}
}

// GetField 获取字段值
func (dm *DeviceModel) GetField(fieldName string) (interface{}, bool) {
	if value, exists := dm.Fields[fieldName]; exists {
		return value, true
	}
	if value, exists := dm.CachedFields[fieldName]; exists {
		return value, true
	}
	if value, exists := dm.StableFields[fieldName]; exists {
		return value, true
	}
	return nil, false
}

// Equal 方法用于比较两个 DeviceModel 是否是相同设备
// 两个 DeviceModel 相等的条件是 DeviceName 和 DeviceType 都相同
func (dm *DeviceModel) Equal(other *DeviceModel) bool {
	if dm == nil || other == nil {
		return false
	}
	return dm.DeviceName == other.DeviceName && dm.DeviceType == other.DeviceType
}
