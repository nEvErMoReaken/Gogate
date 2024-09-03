package model

// DeviceModel 代表一个设备的物模型
type DeviceModel struct {
	DeviceName string                 // 设备名称，例如 "vobc0001.abc"
	DeviceType string                 // 设备类型，例如 "vobc.info"
	Fields     map[string]interface{} // 动态字段存储，key 为字段名称，value 为字段值
	ts         int64                  // 时间戳
}

// NewDeviceModel 创建一个新的设备物模型
func NewDeviceModel(name, deviceType string) *DeviceModel {
	return &DeviceModel{
		DeviceName: name,
		DeviceType: deviceType,
		Fields:     make(map[string]interface{}),
	}
}

// SetField 设置或更新字段值
func (dm *DeviceModel) SetField(fieldName string, value interface{}) {
	dm.Fields[fieldName] = value
}

// GetField 获取字段值
func (dm *DeviceModel) GetField(fieldName string) (interface{}, bool) {
	val, exists := dm.Fields[fieldName]
	return val, exists
}

// GetAllFields 返回所有字段的拷贝
func (dm *DeviceModel) GetAllFields() map[string]interface{} {
	fieldsCopy := make(map[string]interface{}, len(dm.Fields))
	for k, v := range dm.Fields {
		fieldsCopy[k] = v
	}
	return fieldsCopy
}
