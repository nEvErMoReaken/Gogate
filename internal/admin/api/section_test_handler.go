package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"gateway/internal/dispatcher"
	"gateway/internal/parser" // Assuming parser package path
	"gateway/internal/pkg"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TestSectionRequest defines the structure for the section test request body.
// Updated to handle both single section and sequence of sections.
type TestSectionRequest struct {
	SectionConfigs []map[string]interface{} `json:"sectionConfigs"` // Array of section configs
	SectionConfig  map[string]interface{}   `json:"sectionConfig"`  // Single section config (for backward compatibility)
	HexPayload     string                   `json:"hexPayload" binding:"required"`
	InitialVars    map[string]interface{}   `json:"initialVars"`    // Optional: Pre-populate VarStore
	GlobalMap      map[string]interface{}   `json:"globalMap"`      // Optional: Provide GlobalMap for Env
	ProtocolConfig map[string]interface{}   `json:"protocolConfig"` // Optional: Protocol configuration
}

// ProcessingStepInfo 记录单个处理节点的详细信息
type ProcessingStepInfo struct {
	NodeLabel     string                 `json:"nodeLabel"`       // 节点标签
	StartIndex    int                    `json:"startIndex"`      // 处理开始时的光标
	EndIndex      int                    `json:"endIndex"`        // 处理结束时的光标
	ConsumedBytes string                 `json:"consumedBytes"`   // 消耗的字节 (Hex)
	VarsBefore    map[string]interface{} `json:"varsBefore"`      // 此步骤处理前的变量状态 (深拷贝)
	VarsAfter     map[string]interface{} `json:"varsAfter"`       // 此步骤处理后的变量状态 (深拷贝)
	Error         string                 `json:"error,omitempty"` // 此步骤的错误信息
}

// Helper function for deep copying map[string]interface{}
// Note: This is a basic deep copy, might need improvement for nested complex types
func deepCopyMap(original map[string]interface{}) map[string]interface{} {
	if original == nil {
		return nil
	}
	newMap := make(map[string]interface{})
	for key, value := range original {
		// Basic type handling, needs enhancement for slices, nested maps etc. if necessary
		// For now, assume primitive types or rely on standard assignment copy for complexity
		newMap[key] = value // Caution: This is shallow for nested structures!
		// TODO: Implement a more robust deep copy if vars can contain maps/slices
	}
	return newMap
}

// StrategyResult 包含每个策略处理后的点数据
type StrategyResult struct {
	StrategyName string       `json:"strategyName"` // 策略名称
	Points       []*pkg.Point `json:"points"`       // 该策略处理的点
}

// TestSectionResponse defines the structure for a successful section test response.
type TestSectionResponse struct {
	Points            []*pkg.Point           `json:"points"`            // 所有生成的点
	FinalVars         map[string]interface{} `json:"finalVars"`         // 最终变量状态
	FinalCursor       int                    `json:"finalCursor"`       // 最终游标位置
	TotalBytes        string                 `json:"totalBytes"`        // 总处理字节(十六进制)
	ProcessingSteps   []ProcessingStepInfo   `json:"processingSteps"`   // 处理步骤详情
	ProcessingTime    int64                  `json:"processingTime"`    // 处理时间(纳秒)
	Debug             map[string]interface{} `json:"debug,omitempty"`   // 调试信息(可选)
	DispatcherResults []StrategyResult       `json:"dispatcherResults"` // Dispatcher处理后的结果
}

// TestSectionHandler handles requests to test a section configuration.
func TestSectionHandler(c *gin.Context) {
	log := pkg.LoggerFromContext(c.Request.Context()) // Use request context for logging
	var request TestSectionRequest
	var startTime time.Time // 在函数顶部声明 startTime

	// 1. Bind JSON request body
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Warn("Failed to bind request JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	// Handle both single section and array of sections
	if len(request.SectionConfigs) == 0 && request.SectionConfig != nil {
		// If SectionConfigs is empty but SectionConfig is provided, use the single config
		request.SectionConfigs = []map[string]interface{}{request.SectionConfig}
	}

	// Validate that we have at least one section config
	if len(request.SectionConfigs) == 0 {
		errMsg := "No section configuration provided"
		log.Warn(errMsg)
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	// 2. Decode Hex Payload
	if len(request.HexPayload)%2 != 0 {
		errMsg := "Invalid hex payload: length must be even"
		log.Warn(errMsg, zap.String("payload", request.HexPayload))
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}
	byteData, err := hex.DecodeString(request.HexPayload)
	if err != nil {
		log.Warn("Failed to decode hex payload", zap.String("payload", request.HexPayload), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid hex payload: " + err.Error()})
		return
	}

	// 3. 创建解析器配置
	config := &pkg.Config{
		Parser: pkg.ParserConfig{
			Type: "byteParser",
			Para: map[string]interface{}{
				"protoFile": "test_proto", // 使用固定名称
				"globalMap": request.GlobalMap,
				// 添加调试标志，请求内部解析器记录详细信息
				"debug":      true,
				"logDetails": true,
			},
		},
		Others: map[string]interface{}{
			"test_proto": request.SectionConfigs, // 使用用户提供的配置
		},
	}

	// 4. 创建上下文并添加配置
	processCtx := pkg.WithLogger(context.Background(), log)
	processCtx = pkg.WithConfig(processCtx, config)

	// 5. 创建ByteParser
	byteParser, err := parser.NewByteParser(processCtx)
	if err != nil {
		log.Warn("Failed to create parser", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create parser: " + err.Error()})
		return
	}

	// 6. 设置初始变量（直接使用内部env.Vars）
	if len(request.InitialVars) > 0 && byteParser.Env != nil {
		for k, v := range request.InitialVars {
			byteParser.Env.Vars[k] = v
		}
		log.Debug("应用了初始变量", zap.Any("initialVars", request.InitialVars))
	}

	// --- 开始同步处理 ---
	startTime = time.Now() // 记录处理开始时间

	// 创建用于本次测试的 ByteState
	testState := parser.NewByteState(byteParser.Env, byteParser.LabelMap, byteParser.Nodes)
	testState.Data = byteData        // 设置输入数据
	out := make([]*pkg.Point, 0, 10) // 初始化用于收集 Points 的 slice
	var processingErr error          // 声明用于捕获处理错误的变量

	// 初始化步骤记录 slice
	processingSteps := make([]ProcessingStepInfo, 0, len(byteParser.Nodes))

	// 执行同步处理循环
	processedNodeCount := 0
	current := testState.Nodes[0] // 从第一个节点开始
	nodeIndex := 0                // 用于日志记录
	const maxNodesForTest = 100   // 定义一个测试用的最大节点数限制，避免无限循环

	for current != nil {
		// 添加死循环防护
		if processedNodeCount >= maxNodesForTest {
			processingErr = fmt.Errorf("死循环防护触发：处理节点数 (%d) 超过最大限制 (%d)", processedNodeCount, maxNodesForTest)
			log.Error(processingErr.Error(), zap.Stringer("lastNode", current))
			// 记录带有错误的最后一步
			lastStepInfo := ProcessingStepInfo{
				NodeLabel:  "LoopProtectionTriggered",
				StartIndex: testState.Cursor, // 使用当前光标作为起始和结束
				EndIndex:   testState.Cursor,
				Error:      processingErr.Error(),
				VarsBefore: deepCopyMap(testState.Env.Vars), // 记录触发时当前的变量状态
				VarsAfter:  deepCopyMap(testState.Env.Vars), // 记录相同状态，因为没有实际处理
			}
			processingSteps = append(processingSteps, lastStepInfo)
			break // 跳出循环
		}
		processedNodeCount++

		startIndex := testState.Cursor
		nodeLabel := "Unknown" // 默认标签

		// 优先尝试获取 Label
		if labelProvider, ok := current.(interface{ GetLabel() string }); ok {
			nodeLabel = labelProvider.GetLabel()
			if nodeLabel != "" {
				// 成功获取有效的 Label
				log.Debug("使用节点的 Label", zap.String("label", nodeLabel))
			}
		}

		// 如果 Label 为空或获取失败，尝试获取 Description
		if nodeLabel == "" || nodeLabel == "Unknown" {
			if descProvider, ok := current.(interface{ GetDescription() string }); ok {
				desc := descProvider.GetDescription()
				if desc != "" {
					nodeLabel = desc
					log.Debug("使用节点的 Description", zap.String("description", nodeLabel))
				}
			}
		}

		// 如果上述方法都失败，尝试获取其他可能的标识符
		if nodeLabel == "" || nodeLabel == "Unknown" {
			// 尝试获取 Section 类型和描述 (如果适用)
			if sectionNode, ok := current.(interface{ SectionType() string }); ok {
				sectionType := sectionNode.SectionType()
				if sectionType != "" {
					nodeLabel = sectionType
					log.Debug("使用节点的 SectionType", zap.String("sectionType", nodeLabel))
				}
			}
		}

		// 最后回退到类型+索引组合
		if nodeLabel == "" || nodeLabel == "Unknown" {
			// 使用类型名和索引的组合作为最后的回退
			nodeLabel = fmt.Sprintf("%T (步骤 %d)", current, nodeIndex)
			log.Debug("无法获取友好名称，使用类型+索引", zap.String("generatedLabel", nodeLabel), zap.Stringer("node", current))
		}
		// --- 获取结束 ---

		stepInfo := ProcessingStepInfo{
			NodeLabel:  nodeLabel,
			StartIndex: startIndex,
		}

		// 记录处理前的变量状态
		stepInfo.VarsBefore = deepCopyMap(testState.Env.Vars)

		// 调用 ProcessWithBytes
		var tmp parser.BProcessor
		log.Debug("同步处理节点", zap.Int("index", nodeIndex), zap.String("label", nodeLabel), zap.Stringer("node", current), zap.Int("cursor", startIndex))
		out, tmp, processingErr = current.ProcessWithBytes(processCtx, testState, out) // processCtx 来自 handler 参数

		endIndex := testState.Cursor // 获取结束光标
		stepInfo.EndIndex = endIndex

		// 计算消耗的字节
		consumedBytes := ""
		if endIndex > startIndex && endIndex <= len(testState.Data) {
			consumedBytes = hex.EncodeToString(testState.Data[startIndex:endIndex])
		} else if endIndex == startIndex {
			consumedBytes = "" // 没有消耗字节
		} else {
			log.Warn("光标位置异常，可能回退或超出范围", zap.Int("start", startIndex), zap.Int("end", endIndex))
			consumedBytes = fmt.Sprintf("[光标异常: start=%d, end=%d]", startIndex, endIndex)
		}
		stepInfo.ConsumedBytes = consumedBytes

		// 记录处理后的变量状态
		stepInfo.VarsAfter = deepCopyMap(testState.Env.Vars)

		// 检查错误
		if processingErr != nil {
			log.Error("同步处理节点时发生错误",
				zap.Int("nodeIndex", nodeIndex),
				zap.String("label", nodeLabel),
				zap.Stringer("node", current),
				zap.Error(processingErr))
			stepInfo.Error = processingErr.Error()
			processingSteps = append(processingSteps, stepInfo) // 添加包含错误的步骤信息
			break                                               // 跳出循环
		}

		// 如果没有错误，添加步骤信息
		processingSteps = append(processingSteps, stepInfo)

		// 移动到下一个节点
		current = tmp
		nodeIndex++
	}
	log.Debug("同步处理循环结束", zap.Error(processingErr), zap.Int("stepsRecorded", len(processingSteps)), zap.Int("pointsCollected", len(out)), zap.Int("finalCursor", testState.Cursor), zap.Any("finalEnvVars", testState.Env.Vars))

	// --- 记录 Parser 结束时间 ---
	parserEndTime := time.Now()
	processingTimeNs := parserEndTime.Sub(startTime).Nanoseconds() // 计算解析耗时

	// 检查处理循环后的错误 (循环内部已记录错误步骤，此检查通常非必要，除非要捕获特定情况)
	// if processingErr != nil && stepInfo.Error == "" { // stepInfo 在此作用域无效
	// 	 log.Warn("处理循环因未记录的错误而中止", zap.Error(processingErr))
	// }

	// 准备 Dispatcher 输入
	// 手动创建一个 PointPackage
	result := &pkg.PointPackage{
		FrameId: fmt.Sprintf("test_%d", startTime.UnixNano()), // 使用时间戳生成唯一ID
		Points:  out,                                          // 使用处理循环收集到的 'out'
		Ts:      parserEndTime,                                // 使用解析结束时间
	}

	// 7. 设置Dispatcher处理
	// 创建dispatcher配置
	dispatcherConfig := &pkg.DispatcherConfig{
		RepeatDataFilters: []pkg.DataFilter{},
	}

	// 如果请求中包含协议配置且包含dispatcher配置，使用传入的配置
	if request.ProtocolConfig != nil {
		// 从协议配置中提取dispatcher配置
		if dispatcherCfg, ok := request.ProtocolConfig["dispatcher"].(map[string]interface{}); ok {
			log.Debug("使用协议中的dispatcher配置")

			// 尝试获取repeat_data_filter
			if repeatDataFilter, ok := dispatcherCfg["repeat_data_filter"].([]interface{}); ok {
				for _, filter := range repeatDataFilter {
					if filterMap, ok := filter.(map[string]interface{}); ok {
						dataFilter := pkg.DataFilter{}

						// 获取dev_filter
						if devFilter, ok := filterMap["dev_filter"].(string); ok {
							dataFilter.DevFilter = devFilter
						} else {
							dataFilter.DevFilter = ".*" // 默认匹配所有设备
						}

						// 获取tele_filter
						if teleFilter, ok := filterMap["tele_filter"].(string); ok {
							dataFilter.TeleFilter = teleFilter
						} else {
							dataFilter.TeleFilter = ".*" // 默认匹配所有遥测
						}

						dispatcherConfig.RepeatDataFilters = append(dispatcherConfig.RepeatDataFilters, dataFilter)
					}
				}
			}
		}
	}

	// 创建树并设置策略过滤器
	tree := dispatcher.NewTree(dispatcherConfig)

	// 设置策略过滤器 - 优先使用协议配置中的策略
	strategyFilterList := map[string][]pkg.DataFilter{}

	if request.ProtocolConfig != nil {
		// 从协议配置中提取策略配置
		if strategyCfg, ok := request.ProtocolConfig["strategy"].([]interface{}); ok && len(strategyCfg) > 0 {
			log.Debug("使用协议中的strategy配置")

			for _, strategy := range strategyCfg {
				if strategyMap, ok := strategy.(map[string]interface{}); ok {
					// 获取策略类型和启用状态
					strategyType, typeOk := strategyMap["type"].(string)
					enabled, enabledOk := strategyMap["enable"].(bool)

					// 跳过未启用的策略
					if enabledOk && !enabled {
						continue
					}

					// 使用默认策略名称
					if !typeOk || strategyType == "" {
						strategyType = "default_strategy"
					}

					// 获取策略过滤器
					strategyFilters := []pkg.DataFilter{}

					if filters, ok := strategyMap["filter"].([]interface{}); ok {
						for _, filter := range filters {
							if filterMap, ok := filter.(map[string]interface{}); ok {
								dataFilter := pkg.DataFilter{}

								// 获取dev_filter
								if devFilter, ok := filterMap["dev_filter"].(string); ok {
									dataFilter.DevFilter = devFilter
								} else {
									dataFilter.DevFilter = ".*" // 默认匹配所有设备
								}

								// 获取tele_filter
								if teleFilter, ok := filterMap["tele_filter"].(string); ok {
									dataFilter.TeleFilter = teleFilter
								} else {
									dataFilter.TeleFilter = ".*" // 默认匹配所有遥测
								}

								strategyFilters = append(strategyFilters, dataFilter)
							}
						}
					}

					// 如果策略没有过滤器，添加一个默认的全匹配过滤器
					if len(strategyFilters) == 0 {
						strategyFilters = append(strategyFilters, pkg.DataFilter{
							DevFilter:  ".*", // 匹配所有设备
							TeleFilter: ".*", // 匹配所有遥测
						})
					}

					// 将策略添加到列表中
					strategyFilterList[strategyType] = strategyFilters
				}
			}
		}
	}

	// 如果没有提取到有效的策略，使用默认的策略配置
	if len(strategyFilterList) == 0 {
		log.Debug("使用默认策略配置")
		strategyFilterList = map[string][]pkg.DataFilter{
			"strategy1": {
				{
					DevFilter:  ".*", // 匹配所有设备
					TeleFilter: ".*", // 匹配所有遥测
				},
			},
			"strategy2": {
				{
					DevFilter:  "dev2.*", // 匹配dev2开头的设备
					TeleFilter: ".*",     // 匹配所有遥测
				},
			},
		}
	}

	// 设置树的策略过滤器列表
	tree.StrategyFilterList = strategyFilterList

	// 记录实际使用的配置
	log.Info("使用的dispatcher配置",
		zap.Any("repeatDataFilters", dispatcherConfig.RepeatDataFilters),
		zap.Any("strategyFilterList", strategyFilterList),
	)

	// 保存dispatcher处理结果
	var dispatcherResults []StrategyResult

	// 检查result是否有效 (现在 result 是手动创建的)
	if result != nil && result.Points != nil && len(result.Points) > 0 {
		log.Debug("开始 Dispatcher BatchAddPoint")
		// 手动使用tree处理点
		err = tree.BatchAddPoint(result)
		if err != nil {
			log.Warn("Dispatcher处理点数据失败", zap.Error(err))
		} else {
			log.Debug("开始 Dispatcher Freeze")
			// 冻结树并获取结果
			finalResult, err := tree.Freeze()
			if err != nil {
				log.Warn("Dispatcher冻结树失败", zap.Error(err))
			} else {
				// --- 记录 Dispatcher 结束时间并更新总时间 ---
				dispatcherEndTime := time.Now()
				processingTimeNs = dispatcherEndTime.Sub(startTime).Nanoseconds() // 更新总处理时间
				log.Debug("Dispatcher 处理完成", zap.Int64("updatedProcessingTimeNs", processingTimeNs))

				// 将结果转换为API响应格式
				for strategyName, pointPackage := range finalResult {
					if pointPackage != nil && len(pointPackage.Points) > 0 {
						dispatcherResults = append(dispatcherResults, StrategyResult{
							StrategyName: strategyName,
							Points:       pointPackage.Points,
						})
					}
				}
			}
		}
	} else {
		log.Warn("Parser未生成有效的Point数据，跳过Dispatcher处理", zap.Int("pointsCount", len(out)))
	}

	// 8. 准备响应
	// 从处理结果中提取变量
	finalVars := make(map[string]interface{})
	// 如果初始变量存在，先复制它们
	if request.InitialVars != nil { // 检查 nil map
		for k, v := range request.InitialVars {
			finalVars[k] = v
		}
	}

	// 从ByteParser的env.Vars中获取最终变量状态(现在应该包含正确的值)
	if byteParser.Env != nil && byteParser.Env.Vars != nil {
		for k, v := range byteParser.Env.Vars {
			finalVars[k] = v // 合并/覆盖
		}
	}
	log.Debug("最终变量状态", zap.Any("finalVars", finalVars))

	finalCursor := testState.Cursor // 从同步处理状态中获取最终游标

	// 手动收集调试信息
	debugInfo := make(map[string]interface{}) // 重新初始化
	debugInfo["timestamp"] = time.Now().UnixNano()
	if result != nil { // 检查 result 是否为 nil
		debugInfo["frame_id"] = result.FrameId
		debugInfo["points_count"] = len(result.Points)
	} else {
		debugInfo["points_count"] = len(out) // 如果 result 为 nil，至少记录收集到的点数
	}
	debugInfo["processed_node_count"] = processedNodeCount
	debugInfo["final_cursor"] = finalCursor // 添加最终游标到调试信息
	// 可以考虑添加 processingSteps 到 debugInfo (如果响应体大小允许)
	// debugInfo["processing_steps_summary"] = fmt.Sprintf("%d steps recorded", len(processingSteps))

	// 确保 processingTime 至少为 1 ns
	finalProcessingTime := processingTimeNs
	if finalProcessingTime == 0 {
		finalProcessingTime = 1
	}

	response := TestSectionResponse{
		Points:            result.Points,       // 使用来自 result 的 points
		FinalVars:         finalVars,           // 应该是正确的
		FinalCursor:       finalCursor,         // 正确的最终游标
		TotalBytes:        request.HexPayload,  // 原始输入
		ProcessingSteps:   processingSteps,     // 新增：处理步骤详情
		ProcessingTime:    finalProcessingTime, // 总处理时间, 保证最小为1ns
		Debug:             debugInfo,
		DispatcherResults: dispatcherResults,
	}

	log.Info("Section test successful (synchronous with steps)",
		zap.Int("stepsRecorded", len(processingSteps)),
		zap.Int("bytesInPayload", len(byteData)),
		zap.Int("pointsGenerated", len(response.Points)),
		zap.Any("finalVars", response.FinalVars),               // 记录最终变量
		zap.Int("finalCursor", response.FinalCursor),           // 记录最终游标
		zap.Int64("processingTimeNs", response.ProcessingTime), // 使用 response 中的值记录日志
		zap.Int("dispatcherResultsCount", len(dispatcherResults)),
	)
	c.JSON(http.StatusOK, response)
}
