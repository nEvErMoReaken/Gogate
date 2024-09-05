package model

import (
	"fmt"
	"github.com/google/uuid"
	"gw22-train-sam/config"
	"gw22-train-sam/logger"
	strategy2 "gw22-train-sam/strategy"
	"regexp"
	"strings"
	"time"
)

// DeviceSnapshot 代表一个设备的物模型在某时刻的快照
type DeviceSnapshot struct {
	id         uuid.UUID                // 设备 ID
	DeviceName string                   // 设备名称，例如 "vobc0001.abc"
	DeviceType string                   // 设备类型，例如 "vobc.info"
	Fields     map[string]interface{}   // 字段存储，key 为字段名称，value 为字段值
	PointMap   map[string]*PointPackage // 数据点映射，key 为策略名称，value 为数据点，仅为了方便查找
	Ts         time.Time                // 时间戳
}

// SnapshotCollection 代表设备快照的集合
type SnapshotCollection map[string]*DeviceSnapshot

// snapshotCollection 用于缓存设备快照，key 为设备名称和设备类型的组合，value 为设备快照
var snapshotCollection SnapshotCollection

// InitSnapshotCollection 初始化设备快照的数据点映射
func InitSnapshotCollection(proto *config.Proto, comm *config.Common) {
	snapshotCollection = make(map[string]*DeviceSnapshot)
	// 遍历所有的 PreParsing 和 Parsing 步骤，初始化设备快照
	for _, step := range proto.PreParsing {
		deviceSnapshot := GetDeviceSnapshot(step.To.Device, step.To.Type)
		for _, field := range step.To.Fields {
			deviceSnapshot.SetField(field, nil)
		}
	}
	for _, step := range proto.Parsing {
		deviceSnapshot := GetDeviceSnapshot(step.To.Device, step.To.Type)
		for _, field := range step.To.Fields {
			deviceSnapshot.SetField(field, nil)
		}
	}
	// 初始化发送策略
	for _, deviceSnapshot := range snapshotCollection {
		deviceSnapshot.initPointPackage(comm)
	}
}

// GetDeviceSnapshot 获取设备快照，如果设备快照已经存在，则直接返回，否则创建一个新的设备快照
func GetDeviceSnapshot(deviceName string, deviceType string) *DeviceSnapshot {

	// 如果设备快照已经存在，则直接返回
	if snapshot, exists := snapshotCollection[deviceName+":"+deviceType]; exists {
		return snapshot
	}
	// 如果设备快照不存在，则创建一个新的设备快照并返回
	newSnapshot := NewSnapshot(deviceName, deviceType)
	snapshotCollection[deviceName+":"+deviceType] = newSnapshot
	return newSnapshot
}

// NewSnapshot 创建一个新的设备快照，不允许使用 DeviceSnapshot{} 创建
func NewSnapshot(deviceName, deviceType string) *DeviceSnapshot {
	// 生成一个新的 UUID
	newID, err := uuid.NewUUID()
	if err != nil {
		logger.Log.Errorf("failed to generate UUID: %s", err.Error())
		return nil
	}
	return &DeviceSnapshot{
		id:         newID,
		DeviceName: deviceName,
		DeviceType: deviceType,
		Fields:     make(map[string]interface{}),
		PointMap:   make(map[string]*PointPackage),
	}
}

// initPointPackage 初始化设备快照的数据点映射
func (dm *DeviceSnapshot) initPointPackage(common *config.Common) {
	for _, strategy := range common.Strategy {
		for _, filter := range strategy.Filter {
			// 遍历field，判断是否符合策略过滤条件
			for fieldKey, fieldValue := range dm.Fields {
				if checkFilter(dm.DeviceType, dm.DeviceName, fieldKey, filter) {
					st := strategy2.GetStrategy(strategy.Type)
					// 不存在，则创建一个新的PointPackage，否则更新PointPackage
					// 初始化PointMap
					if _, exists := dm.PointMap[strategy.Type]; !exists {
						dm.PointMap[strategy.Type] = &PointPackage{
							Point: Point{
								DeviceName: dm.DeviceName,
								DeviceType: dm.DeviceType,
								Field:      map[string]interface{}{fieldKey: fieldValue},
								Ts:         time.Now(),
							},
							Strategy: st,
						}
					} else {
						dm.PointMap[strategy.Type].merge(fieldKey, fieldValue)
					}
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

	// 分别匹配设备类型、设备名称和遥测名称
	return deviceTypeRe.MatchString(deviceType) &&
		deviceNameRe.MatchString(deviceName) &&
		telemetryRe.MatchString(telemetryName)
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

// launch 发射所有数据点
func (dm *DeviceSnapshot) launch() {
	for _, pp := range dm.PointMap {
		pp.launch()
	}
}

// launchALL 发射所有数据点
func (sc SnapshotCollection) launchALL() {
	for _, dm := range sc {
		dm.launch()
	}
}
