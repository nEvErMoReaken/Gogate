package pkg

import (
	"encoding/json"
	"fmt"
	"gateway/internal/strategy"
	"gateway/logger"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DeviceSnapshot 代表一个设备的物模型在某时刻的快照
type DeviceSnapshot struct {
	Id         uuid.UUID           `json:"id"`          // 设备 ID
	DeviceName string              `json:"device_name"` // 设备名称，例如 "vobc0001.abc"
	DeviceType string              `json:"device_type"` // 设备类型，例如 "vobc.info"
	Fields     map[string]any      `json:"fields"`      // 字段存储，key 为字段名称，value 为字段值
	DataSink   map[string][]string `json:"sink_map"`    // 指示策略-字段名的映射关系
	Ts         time.Time           `json:"timestamp"`   // 时间戳
}

// Clear 清空设备快照信息
func (dm *DeviceSnapshot) Clear() {
	// 将Fields中的字面量值置空
	for _, value := range dm.Fields {
		if value != nil {
			// 重置为零值，确保引用保留，但内容清空
			value = nil
		}
	}
	// 清空Ts
	dm.Ts = time.Time{}
}

// SnapshotCollection 代表设备快照的集合
type SnapshotCollection map[string]*DeviceSnapshot

// GetDeviceSnapshot 获取设备快照，如果设备快照已经存在，则直接返回，否则创建一个新的设备快照
func (sc *SnapshotCollection) GetDeviceSnapshot(deviceName string, deviceType string) *DeviceSnapshot {
	// 如果设备快照已经存在，则直接返回
	if snapshot, exists := (*sc)[deviceName+":"+deviceType]; exists {
		//common.Log.Debugf("GetDeviceSnapshot: %s:%s", deviceName, deviceType)
		return snapshot
	}
	// 如果设备快照不存在，则创建一个新的设备快照并返回
	newSnapshot := NewSnapshot(deviceName, deviceType)
	//common.Log.Debugf("%+v", newSnapshot)
	(*sc)[deviceName+":"+deviceType] = newSnapshot
	return newSnapshot
}

func (sc *SnapshotCollection) SetDeviceSnapshot(deviceName string, deviceType string, key string, value interface{}, config *Config) {
	if snapshot, exists := (*sc)[deviceName+":"+deviceType]; exists {
		snapshot.SetField(key, value, config)
	} else {
		newSnapshot := NewSnapshot(deviceName, deviceType)
		newSnapshot.SetField(key, value, config)
		(*sc)[deviceName+":"+deviceType] = newSnapshot
	}
}

// NewSnapshot 创建一个新的设备快照，不允许使用 DeviceSnapshot{} 创建
func NewSnapshot(deviceName string, deviceType string) *DeviceSnapshot {
	// 生成一个新的 UUID
	newID, err := uuid.NewUUID()
	if err != nil {
		logger.Log.Errorf("failed to generate UUID: %s", err.Error())
		return nil
	}
	return &DeviceSnapshot{
		Id:         newID,
		DeviceType: deviceType,
		DeviceName: deviceName,
		Fields:     make(map[string]interface{}),
		DataSink:   make(map[string][]string),
	}
}

// toJSON 将 DeviceSnapshot 转换为 JSON 格式的字符串
func (dm *DeviceSnapshot) toJSON() string {

	// 序列化为 JSON 字符串
	jsonBytes, err := json.MarshalIndent(dm, "", "  ")
	if err != nil {
		return fmt.Sprintf("error serializing DeviceSnapshot to JSON: %v", err)
	}

	return string(jsonBytes)
}

// InitDataSink 初始化设备快照的数据点映射结构
// 前提：DeviceSnapshot的DeviceName, DeviceType, Fields字段已经全部初始化
func (dm *DeviceSnapshot) InitDataSink(fieldKey string, common *Config) {
	for _, strategy := range common.Strategy {
		for _, filter := range strategy.Filter {
			// 遍历字段，判断是否符合策略过滤条件
			if checkFilter(dm.DeviceType, dm.DeviceName, fieldKey, filter) {
				// 检查 DataSink 是否已经存在该策略对应的 Point
				if _, exists := dm.DataSink[strategy.Type]; !exists {
					// 不存在则初始化数组并添加
					dm.DataSink[strategy.Type] = []string{fieldKey}
				} else {
					// 如果 Sink 已存在，更新其字段引用
					dm.DataSink[strategy.Type] = append(dm.DataSink[strategy.Type], fieldKey)
				}
			}

		}
	}
}

// checkFilter 根据filter推断Strategies
// 定义设备类型、设备名称、遥测名称的匹配
func checkFilter(deviceType, deviceName, telemetryName, filter string) bool {
	// 解析过滤语法，语法为：<设备类型>:<设备名称>:<遥测名称>
	parts := strings.Split(filter, ":")
	if len(parts) != 3 {
		// 如果过滤条件不符合预期语法
		logger.Log.Errorf("过滤条件格式不正确")
		return false
	}

	// 编译设备类型、设备名称和遥测名称的正则表达式
	deviceTypeRe, err1 := regexp.Compile(parts[0])
	deviceNameRe, err2 := regexp.Compile(parts[1])
	telemetryRe, err3 := regexp.Compile(parts[2])

	// 检查正则表达式编译错误
	if err1 != nil || err2 != nil || err3 != nil {
		logger.Log.Errorf("Error compiling regex: %v, %v, %v\n", err1, err2, err3)
		return false
	}
	logger.Log.Debugf("deviceType: %s, DeviceName: %s, telemetryName: %s", deviceType, deviceName, telemetryName)
	// 打印匹配结果
	logger.Log.Debugf("deviceType: %v, deviceName: %v, telemetryName: %v\n", deviceTypeRe.MatchString(deviceType), deviceNameRe.MatchString(deviceName), telemetryRe.MatchString(telemetryName))
	// 分别匹配设备类型、设备名称和遥测名称
	return deviceTypeRe.MatchString(deviceType) &&
		deviceNameRe.MatchString(deviceName) &&
		telemetryRe.MatchString(telemetryName)
}

// SetField 设置或更新字段值，支持将值存储为指针
func (dm *DeviceSnapshot) SetField(fieldName string, value interface{}, config *Config) {
	// 如果字段值为“nil”，代表是需要丢弃的值，则不进行任何操作
	if fieldName == "nil" {
		return
	}
	// 如果fileds中已经存在该字段，则更新字段值
	dm.Fields[fieldName] = value
	if _, exists := dm.Fields[fieldName]; !exists {

		dm.Fields[fieldName] = value
		// 初始化该字段的DataSink
		dm.InitDataSink(fieldName, config)
	}
}

// GetField 获取字段值
func (dm *DeviceSnapshot) GetField(fieldName string) (interface{}, bool) {
	if value, exists := dm.Fields[fieldName]; exists {
		return value, true
	}
	return nil, false
}

// Equal 方法用于比较两个 DeviceSnapshot 是否是相同设备
// 两个 DeviceSnapshot 相等的条件是 TemplateDeviceName 和 DeviceType 都相同
func (dm *DeviceSnapshot) Equal(other *DeviceSnapshot) bool {
	if dm == nil || other == nil {
		return false
	}
	return dm.DeviceName == other.DeviceName && dm.DeviceType == other.DeviceType
}

// launch 发射所有数据点
func (dm *DeviceSnapshot) launch() {
	logger.Log.Infof("launching device %+v ", dm.toJSON())
	for st := range dm.DataSink {
		select {
		case strategy.GetStrategy(st).GetChan() <- dm.makePoint(st):
			// 成功发送
		default:
			// 打印通道堵塞警告，避免影响其他通道
			logger.Log.Errorf("Failed to send data to strategy %s, current channel lenth: %d", st, len(strategy.GetStrategy(st).GetChan()))
		}
	}
	// 清空设备快照
	dm.Clear()
}

// makePoint 生成Point
func (dm *DeviceSnapshot) makePoint(st string) Point {
	point := Point{
		DeviceName: dm.DeviceName,
		DeviceType: dm.DeviceType,
		Field:      make(map[string]interface{}),
		Ts:         dm.Ts,
	}
	for _, fieldName := range dm.DataSink[st] {
		point.Field[fieldName] = dm.Fields[fieldName]
	}
	return point
}

// LaunchALL 发射所有数据点
func (sc *SnapshotCollection) LaunchALL() {
	for _, dm := range *sc {
		dm.launch()
	}
}
