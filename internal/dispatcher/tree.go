package dispatcher

import (
	"fmt"
	"gateway/internal/pkg"
	"regexp"
	"strings"
	"time"
)

// Tree 是一棵设备树，主要功能如下
// - 帧过滤
// - 数据聚合
// - 数据分流
// - 超时监测
type Tree struct {
	Root               *Node
	pathCache          map[string][]string         // 缓存设备路径拆分结果
	filterList         []pkg.DataFilter            // 过滤列表
	StrategyFilterList map[string][]pkg.DataFilter // 策略过滤列表
	LatestTs           time.Time                   // 最新更新时间
	LatestFrameId      string                      // 最新帧ID , 用于追踪接收和处理的整个生命周期
	StrategyRoute      map[string]pkg.PointPackage // 策略路由
	dirtyNodes         map[*Node]struct{}          // 存储本次值有变化的节点 (优化Freeze性能)
}

// Node 节点，用于存储设备信息
type Node struct {
	Children                map[string]*Node
	Parent                  *Node
	devName                 string
	HotValue                map[string]interface{} // 本次待发送的值
	LastValue               map[string]interface{} // 上次发送的值
	repeatShouldFilterCache map[string]bool        // 缓存是否需要过滤
	// teleShouldFilterCache   map[string]bool        // 旧的缓存遥测是否需要过滤（已废弃）
	strategyTeleFilterCache map[string]map[string]bool // 按策略缓存遥测过滤结果 [strategyName][telemetryName] -> shouldFilter
	repeatFilterRule        []string                   // 过滤规则
	strategyFilterRule      map[string][]string        // 目标策略列表
}

// NewTree 创建一个新的设备树实例
//
// 该函数初始化设备树的根节点和各种映射，设置初始的过滤器列表。
//
// 参数:
//   - dispatcherConfig: 分发器配置，包含过滤规则等信息
//
// 返回:
//   - 初始化完成的设备树实例
func NewTree(dispatcherConfig *pkg.DispatcherConfig) *Tree {
	return &Tree{
		Root: &Node{
			Children:                make(map[string]*Node),
			Parent:                  nil,
			HotValue:                make(map[string]interface{}),
			LastValue:               make(map[string]interface{}),
			repeatShouldFilterCache: make(map[string]bool),
			// teleShouldFilterCache:   make(map[string]bool), // 初始化旧缓存（已废弃）
			strategyTeleFilterCache: make(map[string]map[string]bool), // 初始化新的策略缓存
			strategyFilterRule:      make(map[string][]string),
			repeatFilterRule:        make([]string, 0),
		},
		pathCache:          make(map[string][]string),
		filterList:         dispatcherConfig.RepeatDataFilters,
		StrategyFilterList: make(map[string][]pkg.DataFilter),
		StrategyRoute:      make(map[string]pkg.PointPackage),
		dirtyNodes:         make(map[*Node]struct{}), // 初始化 dirtyNodes
	}
}

// getPathParts 将设备名称分割为路径部分
//
// 该方法使用缓存来避免重复分割相同的设备名称，提高性能。
//
// 参数:
//   - devName: 设备名称，通常是以点分隔的路径格式
//
// 返回:
//   - 分割后的路径部分字符串数组
func (t *Tree) getPathParts(devName string) []string {
	if parts, ok := t.pathCache[devName]; ok {
		return parts
	}
	parts := strings.Split(devName, ".")
	t.pathCache[devName] = parts
	return parts
}

// GetNode 根据设备名称获取或创建对应的节点
//
// 该方法沿着设备路径逐级查找节点，如果不存在则创建新节点并设置过滤规则。
//
// 参数:
//   - devName: 设备名称，通常是以点分隔的路径格式
//
// 返回:
//   - 找到或创建的节点
//   - 可能的错误信息
func (t *Tree) GetNode(devName string) (*Node, error) {
	// 解析devName, 获取设备树路径
	devNameList := t.getPathParts(devName)

	// 从根节点开始查找
	currentNode := t.Root

	// 遍历设备路径，逐级查找或创建节点
	for _, name := range devNameList {
		if name == "" {
			continue
		}

		if _, ok := currentNode.Children[name]; !ok {
			prefix := ""
			if currentNode.devName != "" {
				prefix = currentNode.devName + "."
			}
			newNode := &Node{
				Children:                make(map[string]*Node),
				Parent:                  currentNode,
				devName:                 prefix + name,
				HotValue:                make(map[string]interface{}),
				LastValue:               make(map[string]interface{}),
				repeatFilterRule:        currentNode.repeatFilterRule,
				repeatShouldFilterCache: make(map[string]bool),
				// teleShouldFilterCache:   make(map[string]bool), // 初始化旧缓存（已废弃）
				strategyTeleFilterCache: make(map[string]map[string]bool), // 初始化新的策略缓存
				strategyFilterRule:      make(map[string][]string),
			}
			// 将rules中的规则分发给当前节点
			err := dispatchDevFilter(newNode, t.filterList)
			if err != nil {
				return nil, err
			}
			// 将策略过滤规则分发给当前节点
			err = dispatchStrategyFilter(newNode, t.StrategyFilterList)
			if err != nil {
				return nil, err
			}
			currentNode.Children[name] = newNode
		}
		currentNode = currentNode.Children[name]
	}

	return currentNode, nil
}

// BatchAddPoint 批量添加点数据到设备树
//
// 该方法处理一个点数据列表，更新树的时间戳和帧ID，然后逐个添加点数据。
//
// 参数:
//   - pointList: 包含多个点数据的帧对象
func (t *Tree) BatchAddPoint(pointList *pkg.PointPackage) error {
	t.LatestFrameId = pointList.FrameId
	t.LatestTs = pointList.Ts
	for _, point := range pointList.Points {
		err := t.AddPoint(point.Device, point.Field)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateTime 更新设备树的最新时间戳
//
// 参数:
//   - ts: 新的时间戳
func (t *Tree) UpdateTime(ts time.Time) {
	t.LatestTs = ts
}

// AddPoint 添加单个点数据到设备树
//
// 该方法获取点对应的节点，然后根据过滤规则决定是否更新节点的值。
// 如果数据有变化，会将其添加到 dirtyNodes 列表。
//
// 参数:
//   - devName: 设备名称
//   - value: 包含键值对的遥测数据映射
//
// 返回:
//   - 可能的错误信息
func (t *Tree) AddPoint(devName string, value map[string]interface{}) error {
	node, err := t.GetNode(devName)
	if err != nil {
		return err
	}

	updated := false // 标记节点值是否有实际更新

	// 检查是否需要过滤
	if len(node.repeatFilterRule) > 0 {
		// 如果设备名称需要过滤，继续检查遥测名称是否需要过滤
		for k, v := range value {
			// 继续逐个检查遥测名称是否需要过滤
			shouldFilterTelemetry, err := checkFilter(k, node.repeatFilterRule, nil)
			if err != nil {
				return err
			}
			// 如果遥测名称需要过滤，则检查值是否变化
			if shouldFilterTelemetry {
				// 检查上一帧变量值与本帧变量值是否相同
				if lastValue, ok := node.LastValue[k]; ok {
					// 如果上一帧变量值与本帧变量值相同，则不更新
					if lastValue == v {
						continue
					}
					// 如果上一帧变量值与本帧变量值不同，则同时更新LastValue和HotValue
					node.LastValue[k] = v
					node.HotValue[k] = v
					updated = true // 标记有更新
				} else {
					// 如果上一帧变量值不存在，则直接更新LastValue和HotValue
					node.LastValue[k] = v
					node.HotValue[k] = v
					updated = true // 标记有更新
				}
			} else {
				// 如果该遥测名称不需要过滤，则直接更新HotValue
				node.HotValue[k] = v
				updated = true // 标记有更新
			}
		}
	} else {
		// 整个设备都不需要过滤，则直接更新HotValue
		for k, v := range value {
			node.HotValue[k] = v
		}
		if len(value) > 0 {
			updated = true // 标记有更新
		}
	}

	// 如果节点值有更新，则添加到 dirtyNodes 列表
	if updated && len(node.HotValue) > 0 { // 确保 HotValue 不为空才添加
		t.dirtyNodes[node] = struct{}{}
	}

	return nil
}

// Freeze 冻结当前设备树状态，收集所有变化的点数据
//
// 该方法遍历 dirtyNodes 列表，收集所有标记为已更改的节点的数据，
// 并根据策略过滤规则将数据分发到不同的策略包中。
// 处理完成后，会重置所有已处理节点的状态并清空 dirtyNodes 列表。
//
// 返回:
//   - 按策略分组的点数据映射
//   - 可能的错误信息
func (t *Tree) Freeze() (map[string]*pkg.PointPackage, error) {
	finalResult := make(map[string]*pkg.PointPackage)

	// 遍历所有脏节点
	for node := range t.dirtyNodes {
		// 确保节点确实有待处理的数据 (理论上应该总是有，但增加检查更健壮)
		if len(node.HotValue) == 0 {
			continue
		}

		// 为每个策略创建一个字段映射，收集所有匹配的遥测点
		strategyFields := make(map[string]map[string]interface{})

		// 遍历当前节点的热数据
		for hotKey, hotValue := range node.HotValue {
			// 遍历该节点关联的所有策略过滤规则
			for strategy, filter := range node.strategyFilterRule {
				// 获取或初始化该策略的缓存
				if _, ok := node.strategyTeleFilterCache[strategy]; !ok {
					node.strategyTeleFilterCache[strategy] = make(map[string]bool)
				}
				strategyCache := node.strategyTeleFilterCache[strategy]

				// **使用策略特定的缓存进行检查**
				shouldFilterTelemetry, err := checkFilter(hotKey, filter, strategyCache)
				if err != nil {
					// 记录错误但继续处理其他遥测点或策略
					fmt.Printf("过滤策略 %s 的遥测点 %s 时出错: %v\n", strategy, hotKey, err)
					continue
				}

				// 如果遥测点匹配当前策略的过滤规则
				if shouldFilterTelemetry {
					// 懒初始化策略字段映射
					if _, exists := strategyFields[strategy]; !exists {
						strategyFields[strategy] = make(map[string]interface{})
					}
					// 将遥测点添加到对应策略的字段映射中
					strategyFields[strategy][hotKey] = hotValue
				}
			}
		}

		// 为每个策略创建一个Point对象，包含所有匹配的遥测点
		for strategy, fields := range strategyFields {
			if len(fields) > 0 {
				// 创建Point对象
				point := &pkg.Point{
					Device: node.devName,
					Field:  fields,
				}

				// 确保策略包已初始化
				if _, exists := finalResult[strategy]; !exists {
					finalResult[strategy] = &pkg.PointPackage{
						Points:  make([]*pkg.Point, 0),
						FrameId: t.LatestFrameId, // 使用 Tree 的最新 FrameId
						Ts:      t.LatestTs,      // 使用 Tree 的最新 Ts
					}
				}

				// 将点添加到策略包中
				finalResult[strategy].Points = append(finalResult[strategy].Points, point)
			}
		}

		// 清空当前已处理节点的 HotValue
		// 使用 map 清空最高效的方式是重新 make
		node.HotValue = make(map[string]interface{})

	} // 结束遍历 dirtyNodes

	// 清空脏节点列表，为下一次收集做准备
	t.dirtyNodes = make(map[*Node]struct{})

	return finalResult, nil
}

// dispatchDevFilter 检查并分发设备名称过滤规则
//
// 该函数将全局过滤规则应用到特定节点，检查节点名称是否匹配过滤规则，
// 如果匹配则将对应的遥测过滤规则添加到节点的过滤规则列表中。
//
// 参数:
//   - node: 需要应用过滤规则的节点
//   - filterList: 过滤规则列表
//
// 返回:
//   - 可能的错误信息
func dispatchDevFilter(node *Node, filterList []pkg.DataFilter) error {
	for _, filter := range filterList {
		deviceRe, err := regexp.Compile(filter.DevFilter)
		if err != nil {
			return fmt.Errorf("error compiling regex: %v", err)
		}
		if deviceRe.MatchString(node.devName) {
			node.repeatFilterRule = append(node.repeatFilterRule, filter.TeleFilter)
		}
	}
	return nil
}

// dispatchStrategyFilter 检查并分发策略过滤规则
//
// 该函数将策略特定的过滤规则应用到节点，检查节点名称是否匹配过滤规则，
// 如果匹配则将对应的遥测过滤规则添加到节点的策略过滤规则映射中。
//
// 参数:
//   - node: 需要应用策略过滤规则的节点
//   - strategyFilter: 按策略分组的过滤规则映射
//
// 返回:
//   - 可能的错误信息
func dispatchStrategyFilter(node *Node, strategyFilter map[string][]pkg.DataFilter) error {
	for strategy, filterList := range strategyFilter {
		for _, filter := range filterList {
			deviceRe, err := regexp.Compile(filter.DevFilter)
			if err != nil {
				return fmt.Errorf("error compiling regex: %v", err)
			}
			if deviceRe.MatchString(node.devName) {
				// 将遥测名称的过滤规则分发给当前节点
				node.strategyFilterRule[strategy] = append(node.strategyFilterRule[strategy], filter.TeleFilter)
			} else {
				continue
			}
		}
	}
	return nil
}

// checkFilter 根据过滤规则检查名称是否匹配
//
// 该函数使用正则表达式检查给定名称是否匹配过滤规则列表中的任何规则，
// 并使用缓存存储结果以提高性能。
//
// 参数:
//   - name: 要检查的名称
//   - filterList: 过滤规则列表（正则表达式字符串）
//   - cache: 用于缓存结果的映射
//
// 返回:
//   - 是否匹配任何过滤规则
//   - 可能的错误信息
func checkFilter(name string, filterList []string, cache map[string]bool) (bool, error) {
	// 检查缓存
	if cache != nil {
		if result, ok := cache[name]; ok {
			return result, nil
		}
	}

	// 遍历过滤列表
	for _, filter := range filterList {
		// 解析过滤语法，语法为：xagx.vobc.vobc0001.speed.v
		deviceRe, err := regexp.Compile(filter)

		// 检查正则表达式编译错误
		if err != nil {
			return false, fmt.Errorf("error compiling regex: %v", err)
		}
		// 分别匹配设备类型、设备名称和遥测名称
		if deviceRe.MatchString(name) {
			if cache != nil {
				cache[name] = true
			}
			return true, nil
		}
	}

	// 如果没有匹配，也将结果缓存
	if cache != nil {
		cache[name] = false
	}
	return false, nil
}
