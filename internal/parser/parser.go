package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/internal/pkg"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"regexp"
	"strings"
	"time"
)

// Parser 定义一个通用的接口，用于处理各种数据源，并持续维护一个快照集合
type Parser interface {
	Start() // 启动解析器
}

// FactoryFunc 代表一个发送策略的工厂函数
type FactoryFunc func(dataSource pkg.DataSource, mapChan map[string]chan pkg.Point, ctx context.Context) (Parser, error)

// Factories 全局工厂映射，用于注册不同策略类型的构造函数  这里面可能包含了没有启用的数据源
var Factories = make(map[string]FactoryFunc)

// Register 注册一个发送策略
func Register(parserType string, factory FactoryFunc) {
	Factories[parserType] = factory
}

func New(ctx context.Context, dataSource pkg.DataSource, mapChan map[string]chan pkg.Point) (Parser, error) {
	config := pkg.ConfigFromContext(ctx)
	factory, ok := Factories[config.Connector.Type]
	if !ok {
		return nil, fmt.Errorf("未找到解析器类型: %s", config.Connector.Type)
	}

	// 1. 初始化脚本模块
	err := LoadAllScripts(ctx, config.Parser.Para["dir"].(string))
	if err != nil {
		return nil, fmt.Errorf("加载脚本失败: %+v ", zap.Error(err))
	}
	pkg.LoggerFromContext(ctx).Info("已加载Byte脚本", zap.Any("ByteScripts", ByteScriptFuncCache))
	pkg.LoggerFromContext(ctx).Info("已加载Json脚本", zap.Any("JsonScripts", JsonScriptFuncCache))

	// 2. 直接调用工厂函数
	parser, err := factory(dataSource, mapChan, ctx)
	if err != nil {
		return nil, fmt.Errorf("初始化解析器失败: %v", err)
	}
	return parser, nil
}

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
	// 清空Fields
	for key := range dm.Fields {
		dm.Fields[key] = nil // 将每个字段的值置为 nil
	}
	// 清空时间戳
	dm.Ts = time.Time{}
}

// SnapshotCollection 代表设备快照的管理器
type SnapshotCollection map[string]*DeviceSnapshot

// GetDeviceSnapshot 获取设备快照，如果设备快照已经存在，则直接返回，否则创建一个新的设备快照
func (sc *SnapshotCollection) GetDeviceSnapshot(deviceName string, deviceType string) (*DeviceSnapshot, error) {
	// 如果设备快照已经存在，则直接返回
	key := deviceName + ":" + deviceType
	if snapshot, exists := (*sc)[key]; exists {
		return snapshot, nil // 返回指针而不是副本
	}

	// 如果设备快照不存在，则创建一个新的设备快照并返回
	newSnapshot, err := NewSnapshot(deviceName, deviceType)
	if err != nil {
		return nil, err
	}

	// 将新快照存入 map，并返回指针
	(*sc)[key] = newSnapshot
	return newSnapshot, nil
}

func (sc *SnapshotCollection) SetDeviceSnapshot(deviceName string, deviceType string, key string, value interface{}, ctx context.Context) error {
	if snapshot, exists := (*sc)[deviceName+":"+deviceType]; exists {
		err := snapshot.SetField(ctx, key, value)
		if err != nil {
			return err
		}
	} else {
		newSnapshot, err := NewSnapshot(deviceName, deviceType)
		if err != nil {
			return err
		}
		err = newSnapshot.SetField(ctx, key, value)
		if err != nil {
			return err
		}
		(*sc)[deviceName+":"+deviceType] = newSnapshot
	}
	return nil
}

// NewSnapshot 创建一个新的设备快照，不允许使用 DeviceSnapshot{} 创建
func NewSnapshot(deviceName string, deviceType string) (*DeviceSnapshot, error) {
	// 生成一个新的 UUID
	newID, err := uuid.NewUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID: %s", err.Error())
	}
	return &DeviceSnapshot{
		Id:         newID,
		DeviceType: deviceType,
		DeviceName: deviceName,
		Fields:     make(map[string]interface{}),
		DataSink:   make(map[string][]string),
	}, nil
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
func (dm *DeviceSnapshot) InitDataSink(fieldKey string, strategies *[]pkg.StrategyConfig) error {
	for _, strategy := range *strategies {
		if strategy.Filter == nil {
			// 如果策略没有过滤条件，则直接添加字段
			if _, exists := dm.DataSink[strategy.Type]; !exists {
				dm.DataSink[strategy.Type] = []string{fieldKey}
			} else {
				dm.DataSink[strategy.Type] = append(dm.DataSink[strategy.Type], fieldKey)
			}
			continue
		}
		for _, filter := range strategy.Filter {
			// 遍历字段，判断是否符合策略过滤条件
			if ok, err := checkFilter(dm.DeviceType, dm.DeviceName, fieldKey, filter); ok && (err == nil) {
				// 检查 DataSink 是否已经存在该策略对应的 Point
				if _, exists := dm.DataSink[strategy.Type]; !exists {
					// 不存在则初始化数组并添加
					dm.DataSink[strategy.Type] = []string{fieldKey}
				} else {
					// 如果 Sink 已存在，更新其字段引用
					dm.DataSink[strategy.Type] = append(dm.DataSink[strategy.Type], fieldKey)
				}
			} else if err != nil {
				// 如果过滤条件不符合预期语法
				return fmt.Errorf("error compiling regex: %v", err)
			}
		}
	}
	return nil
}

// checkFilter 根据filter推断Strategies
// 定义设备类型、设备名称、遥测名称的匹配
func checkFilter(deviceType, deviceName, telemetryName, filter string) (bool, error) {
	// 解析过滤语法，语法为：<设备类型>:<设备名称>:<遥测名称>
	parts := strings.Split(filter, ":")
	if len(parts) != 3 {
		// 如果过滤条件不符合预期语法
		return false, fmt.Errorf("filter syntax error: %s", filter)
	}

	// 编译设备类型、设备名称和遥测名称的正则表达式
	deviceTypeRe, err1 := regexp.Compile(parts[0])
	deviceNameRe, err2 := regexp.Compile(parts[1])
	telemetryRe, err3 := regexp.Compile(parts[2])

	// 检查正则表达式编译错误
	if err1 != nil || err2 != nil || err3 != nil {
		return false, fmt.Errorf("Error compiling regex: %v, %v, %v\n", err1, err2, err3)
	}
	// 分别匹配设备类型、设备名称和遥测名称
	return deviceTypeRe.MatchString(deviceType) &&
		deviceNameRe.MatchString(deviceName) &&
		telemetryRe.MatchString(telemetryName), nil
}

// SetField 设置或更新字段值，支持将值存储为指针, 需要策略配置来路由具体地发送策略
func (dm *DeviceSnapshot) SetField(ctx context.Context, fieldName string, value interface{}) error {
	// 如果字段值为“nil”，代表是需要丢弃的值，则不进行任何操作
	if fieldName == "nil" {
		return nil
	}
	// 如果fileds中已经不存在该字段，则初始化
	if _, exists := dm.Fields[fieldName]; !exists {

		dm.Fields[fieldName] = value

		// 初始化该字段的DataSink
		err := dm.InitDataSink(fieldName, &pkg.ConfigFromContext(ctx).Strategy)
		if err != nil {
			return err
		}
	} else {
		// 如果已存在， 直接更新
		dm.Fields[fieldName] = value
	}
	return nil
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
func (dm *DeviceSnapshot) launch(ctx context.Context, mapChan map[string]chan pkg.Point) {
	pkg.LoggerFromContext(ctx).Info("launching device snapshot", zap.Any("snapshot", dm))
	//fmt.Printf("launching device snapshot: %v\n", dm)
	//fmt.Printf("mapChan: %v\n", mapChan)
	//fmt.Printf("DataSink: %v\n", dm.DataSink)
	for st := range dm.DataSink {
		select {
		case mapChan[st] <- dm.makePoint(st):
			fmt.Printf("suucessfully sent\n")
			// 成功发送
		default:
			fmt.Sprintln("channel blocked")
			// 打印通道堵塞警告，避免影响其他通道
			pkg.LoggerFromContext(ctx).Warn("channel blocked", zap.String("strategy", st))
		}
	}
	// 清空设备快照
	dm.Clear()
}

// makePoint 组合数据点
func (dm *DeviceSnapshot) makePoint(st string) pkg.Point {
	point := pkg.Point{
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
func (sc *SnapshotCollection) LaunchALL(ctx context.Context, mapChan map[string]chan pkg.Point) {
	for _, dm := range *sc {
		dm.launch(ctx, mapChan)
	}
}
