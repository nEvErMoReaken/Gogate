package model

import (
	"fmt"
	"github.com/google/uuid"
	"gw22-train-sam/common"
	strategy2 "gw22-train-sam/strategy"
	"regexp"
	"strings"
	"time"
)

// DeviceSnapshot 代表一个设备的物模型在某时刻的快照
type DeviceSnapshot struct {
	id                 uuid.UUID                // 设备 ID
	TemplateDeviceName string                   // 模板设备名称，例如 "vobc${id}.abc"
	DeviceName         string                   // 设备名称，例如 "vobc0001.abc"
	DeviceType         string                   // 设备类型，例如 "vobc.info"
	Fields             map[string]interface{}   // 字段存储，key 为字段名称，value 为字段值
	PointMap           map[string]*PointPackage // 数据点映射，key 为策略名称，value 为数据点，仅为了方便查找
	Ts                 time.Time                // 时间戳
}

// FrameContext 每一帧中, 也就是多Chunks共享的上下文
type FrameContext map[string]*interface{}

// SnapshotCollection 代表设备快照的集合
type SnapshotCollection map[string]*DeviceSnapshot

// snapshotCollection 用于缓存设备快照，key 为设备名称和设备类型的组合，value 为设备快照
var snapshotCollection SnapshotCollection

// GetDeviceSnapshot 获取设备快照，如果设备快照已经存在，则直接返回，否则创建一个新的设备快照
func GetDeviceSnapshot(templateDeviceName string, deviceType string) *DeviceSnapshot {

	// 如果设备快照已经存在，则直接返回
	if snapshot, exists := snapshotCollection[templateDeviceName+":"+deviceType]; exists {
		return snapshot
	}
	// 如果设备快照不存在，则创建一个新的设备快照并返回
	newSnapshot := NewSnapshot(templateDeviceName, deviceType)
	snapshotCollection[templateDeviceName+":"+deviceType] = newSnapshot
	return newSnapshot
}

// NewSnapshot 创建一个新的设备快照，不允许使用 DeviceSnapshot{} 创建
func NewSnapshot(tempName, deviceType string) *DeviceSnapshot {
	// 生成一个新的 UUID
	newID, err := uuid.NewUUID()
	if err != nil {
		common.Log.Errorf("failed to generate UUID: %s", err.Error())
		return nil
	}
	return &DeviceSnapshot{
		id:                 newID,
		TemplateDeviceName: tempName,
		DeviceType:         deviceType,
		Fields:             make(map[string]interface{}),
		PointMap:           make(map[string]*PointPackage),
	}
}

// InitPointPackage 初始化设备快照的数据点映射结构
// 前提：DeviceSnapshot的TemplateDeviceName, DeviceType, Fields字段已经初始化
func (dm *DeviceSnapshot) InitPointPackage(common *common.CommonConfig) {
	for _, strategy := range common.Strategy {
		for _, filter := range strategy.Filter {
			// 遍历字段，判断是否符合策略过滤条件
			for fieldKey, fieldValue := range dm.Fields {
				if checkFilter(dm.DeviceType, dm.TemplateDeviceName, fieldKey, filter) {
					st := strategy2.GetStrategy(strategy.Type)
					// 检查 PointMap 是否已经存在该策略对应的 PointPackage
					if _, exists := dm.PointMap[strategy.Type]; !exists {
						// 创建新的 PointPackage，并使用指针引用字段
						dm.PointMap[strategy.Type] = &PointPackage{
							Point: Point{
								DeviceName: &dm.DeviceName,
								DeviceType: &dm.DeviceType,
								Field:      map[string]*interface{}{fieldKey: &fieldValue}, // 引用字段
								Ts:         &dm.Ts,                                         // 使用快照的时间戳引用
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
						pointPackage.Point.Field[fieldKey] = &fieldValue
					}
				}
			}
		}
	}
}

// checkFilter 根据filter推断Strategies
// 定义设备类型、设备名称、遥测名称的匹配
func checkFilter(deviceType, templateDeviceName, telemetryName, filter string) bool {
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
		deviceNameRe.MatchString(templateDeviceName) &&
		telemetryRe.MatchString(telemetryName)
}

// SetDeviceName 通过传入字符串替换模板设备名称
func (dm *DeviceSnapshot) SetDeviceName(context *FrameContext) {
	// 例，将 "vobc${id}.abc" 替换为 "vobc context["id"].abc"
	// 1. 通过正则表达式匹配模板设备名称中的 ${id} 字符串
	re := regexp.MustCompile(`\${(.*?)}`)
	// 2. 查找所有匹配的字符串
	matches := re.FindAllString(dm.TemplateDeviceName, -1)
	// 3. 遍历所有匹配的字符串
	for _, match := range matches {
		// 4. 从 context 中获取变量值
		varName := match[2 : len(match)-1]
		varValue := (*context)[varName]
		// 5. 替换模板设备名称中的变量
		dm.DeviceName = strings.Replace(dm.TemplateDeviceName, match, fmt.Sprintf("%v", varValue), -1)
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
// 两个 DeviceSnapshot 相等的条件是 TemplateDeviceName 和 DeviceType 都相同
func (dm *DeviceSnapshot) Equal(other *DeviceSnapshot) bool {
	if dm == nil || other == nil {
		return false
	}
	return dm.TemplateDeviceName == other.TemplateDeviceName && dm.DeviceType == other.DeviceType
}

// launch 发射所有数据点
func (dm *DeviceSnapshot) launch() {
	for _, pp := range dm.PointMap {
		pp.launch()
	}
}

// LaunchALL 发射所有数据点
func (sc *SnapshotCollection) LaunchALL() {
	for _, dm := range *sc {
		dm.launch()
	}
}
