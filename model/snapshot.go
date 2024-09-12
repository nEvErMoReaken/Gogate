package model

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"gw22-train-sam/common"
	"regexp"
	"strings"
	"time"
)

// DeviceSnapshot 代表一个设备的物模型在某时刻的快照
type DeviceSnapshot struct {
	id         uuid.UUID                // 设备 ID
	DeviceName string                   // 设备名称，例如 "vobc0001.abc"
	DeviceType string                   // 设备类型，例如 "vobc.info"
	Fields     map[string]*interface{}  // 字段存储，key 为字段名称，value 为字段值
	PointMap   map[string]*PointPackage // 数据点映射，key 为策略名称，value 为数据点，仅为了方便查找
	Ts         time.Time                // 时间戳
}

// Clear 清空设备快照信息
func (dm *DeviceSnapshot) Clear() {
	// 将Fields中的字面量值置空
	for _, value := range dm.Fields {
		if value != nil {
			// 重置为零值，确保引用保留，但内容清空
			*value = nil
		}
	}
	// 清空Ts
	dm.Ts = time.Time{}
}

// FrameContext 每一帧中, 也就是多Chunks共享的上下文
type FrameContext map[string]*interface{}

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

func (sc *SnapshotCollection) SetDeviceSnapshot(deviceName string, deviceType string, key string, value interface{}, config *common.Config) {
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
		common.Log.Errorf("failed to generate UUID: %s", err.Error())
		return nil
	}
	return &DeviceSnapshot{
		id:         newID,
		DeviceType: deviceType,
		DeviceName: deviceName,
		Fields:     make(map[string]*interface{}),
		PointMap:   make(map[string]*PointPackage),
	}
}

// toJSON 将 DeviceSnapshot 转换为 JSON 格式的字符串
func (dm *DeviceSnapshot) toJSON() string {
	// 创建一个临时结构体，用来序列化成 JSON
	type jsonSnapshot struct {
		ID         uuid.UUID                          `json:"id"`
		DeviceName string                             `json:"device_name"`
		DeviceType string                             `json:"device_type"`
		Fields     map[string]*interface{}            `json:"fields"`
		PointMap   map[string]map[string]*interface{} `json:"point_map"` // 用来存储数据点和策略映射
		Timestamp  string                             `json:"timestamp"`
	}

	// 将 PointMap 转换为简单的形式
	pointMap := make(map[string]map[string]*interface{})
	for key, value := range dm.PointMap {
		pointMap[key] = value.Point.Field
	}

	// 创建序列化时使用的快照结构体
	jsonStruct := jsonSnapshot{
		ID: dm.id,

		DeviceName: dm.DeviceName,
		DeviceType: dm.DeviceType,
		Fields:     dm.Fields,
		PointMap:   pointMap,
		Timestamp:  dm.Ts.Format(time.RFC3339),
	}

	// 序列化为 JSON 字符串
	jsonBytes, err := json.MarshalIndent(jsonStruct, "", "  ")
	if err != nil {
		return fmt.Sprintf("error serializing DeviceSnapshot to JSON: %v", err)
	}

	return string(jsonBytes)
}

// InitPointPackage 初始化设备快照的数据点映射结构
// 前提：DeviceSnapshot的TemplateDeviceName, DeviceType, Fields字段已经初始化
func (dm *DeviceSnapshot) InitPointPackage(fieldKey string, common *common.Config) {
	for _, strategy := range common.Strategy {
		for _, filter := range strategy.Filter {
			// 遍历字段，判断是否符合策略过滤条件
			if checkFilter(dm.DeviceType, dm.DeviceName, fieldKey, filter) {
				st := GetStrategy(strategy.Type)
				// 检查 PointMap 是否已经存在该策略对应的 PointPackage
				if _, exists := dm.PointMap[strategy.Type]; !exists {
					// 创建新的 PointPackage，并使用指针引用字段
					dm.PointMap[strategy.Type] = &PointPackage{
						Point: Point{
							DeviceName: &dm.DeviceName,
							DeviceType: &dm.DeviceType,
							Field:      map[string]*interface{}{fieldKey: dm.Fields[fieldKey]}, // 引用字段
							Ts:         &dm.Ts,                                                 // 使用快照的时间戳引用
						},
						Strategy: st,
					}
				} else {
					// 如果 PointPackage 已存在，更新其字段引用
					pointPackage := dm.PointMap[strategy.Type]
					if pointPackage.Point.Field == nil {
						pointPackage.Point.Field = make(map[string]*interface{})
					}
					// 更新 PointPackage 中的字段引用
					pointPackage.Point.Field[fieldKey] = dm.Fields[fieldKey]
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
		fmt.Println("过滤条件格式不正确")
		return false
	}

	// 编译设备类型、设备名称和遥测名称的正则表达式
	deviceTypeRe, err1 := regexp.Compile(parts[0])
	deviceNameRe, err2 := regexp.Compile(parts[1])
	telemetryRe, err3 := regexp.Compile(parts[2])

	// 检查正则表达式编译错误
	if err1 != nil || err2 != nil || err3 != nil {
		fmt.Printf("Error compiling regex: %v, %v, %v\n", err1, err2, err3)
		return false
	}
	common.Log.Debugf("deviceType: %s, DeviceName: %s, telemetryName: %s", deviceType, deviceName, telemetryName)
	// 打印匹配结果
	common.Log.Debugf("deviceType: %v, deviceName: %v, telemetryName: %v\n", deviceTypeRe.MatchString(deviceType), deviceNameRe.MatchString(deviceName), telemetryRe.MatchString(telemetryName))
	// 分别匹配设备类型、设备名称和遥测名称
	return deviceTypeRe.MatchString(deviceType) &&
		deviceNameRe.MatchString(deviceName) &&
		telemetryRe.MatchString(telemetryName)
}

// SetField 设置或更新字段值，支持将值存储为指针
func (dm *DeviceSnapshot) SetField(fieldName string, value interface{}, config *common.Config) {
	// 如果字段值为“nil”，代表是需要丢弃的值，则不进行任何操作
	if fieldName == "nil" {
		return
	}
	// 如果fileds中已经存在该字段，则更新字段值，不创建新的指针
	if _, exists := dm.Fields[fieldName]; exists {
		*dm.Fields[fieldName] = value
		return
	} else {
		// 将传入的值转换为指针
		ptr := new(interface{})
		*ptr = value
		// 将指针存入 Fields
		dm.Fields[fieldName] = ptr
		// 更新PointMap中的字段引用
		dm.InitPointPackage(fieldName, config)
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
	common.Log.Info(dm.toJSON())
	for _, pp := range dm.PointMap {
		pp.launch()
	}
	// 清空设备快照
	dm.Clear()
}

// LaunchALL 发射所有数据点
func (sc *SnapshotCollection) LaunchALL() {
	for _, dm := range *sc {
		dm.launch()
	}
}
