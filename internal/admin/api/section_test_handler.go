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
				"protoFile":  "test_proto",
				"globalMap":  request.GlobalMap,
				"debug":      true,
				"logDetails": true,
			},
		},
		Others: map[string]interface{}{
			"test_proto": request.SectionConfigs,
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

	// 6. 设置初始变量
	if len(request.InitialVars) > 0 && byteParser.Env != nil {
		for k, v := range request.InitialVars {
			byteParser.Env.Vars[k] = v
		}
		log.Debug("应用了初始变量", zap.Any("initialVars", request.InitialVars))
	}

	// --- 开始同步处理 ---
	startTime = time.Now()

	testState := parser.NewByteState(byteParser.Env, byteParser.LabelMap, byteParser.Nodes)
	testState.Data = byteData
	var processingErr error

	processingSteps := make([]ProcessingStepInfo, 0, len(byteParser.Nodes))

	processedNodeCount := 0
	current := testState.Nodes[0]
	nodeIndex := 0
	const maxNodesForTest = 100

	for current != nil {
		if processedNodeCount >= maxNodesForTest {
			processingErr = fmt.Errorf("死循环防护触发：处理节点数 (%d) 超过最大限制 (%d)", processedNodeCount, maxNodesForTest)
			log.Error(processingErr.Error(), zap.Stringer("lastNode", current))
			lastStepInfo := ProcessingStepInfo{
				NodeLabel:  "LoopProtectionTriggered",
				StartIndex: testState.Cursor,
				EndIndex:   testState.Cursor,
				Error:      processingErr.Error(),
				VarsBefore: deepCopyMap(testState.Env.Vars),
				VarsAfter:  deepCopyMap(testState.Env.Vars),
			}
			processingSteps = append(processingSteps, lastStepInfo)
			break
		}
		processedNodeCount++

		startIndex := testState.Cursor
		nodeLabel := "Unknown"

		if labelProvider, ok := current.(interface{ GetLabel() string }); ok {
			nodeLabel = labelProvider.GetLabel()
			if nodeLabel != "" {
				log.Debug("使用节点的 Label", zap.String("label", nodeLabel))
			}
		}

		if nodeLabel == "" || nodeLabel == "Unknown" {
			if descProvider, ok := current.(interface{ GetDescription() string }); ok {
				desc := descProvider.GetDescription()
				if desc != "" {
					nodeLabel = desc
					log.Debug("使用节点的 Description", zap.String("description", nodeLabel))
				}
			}
		}

		if nodeLabel == "" || nodeLabel == "Unknown" {
			if sectionNode, ok := current.(interface{ SectionType() string }); ok {
				sectionType := sectionNode.SectionType()
				if sectionType != "" {
					nodeLabel = sectionType
					log.Debug("使用节点的 SectionType", zap.String("sectionType", nodeLabel))
				}
			}
		}

		if nodeLabel == "" || nodeLabel == "Unknown" {
			nodeLabel = fmt.Sprintf("%T (步骤 %d)", current, nodeIndex)
			log.Debug("无法获取友好名称，使用类型+索引", zap.String("generatedLabel", nodeLabel), zap.Stringer("node", current))
		}

		stepInfo := ProcessingStepInfo{
			NodeLabel:  nodeLabel,
			StartIndex: startIndex,
		}

		stepInfo.VarsBefore = deepCopyMap(testState.Env.Vars)

		var tmp parser.BProcessor
		log.Debug("同步处理节点", zap.Int("index", nodeIndex), zap.String("label", nodeLabel), zap.Stringer("node", current), zap.Int("cursor", startIndex))
		tmp, processingErr = current.ProcessWithBytes(processCtx, testState)

		endIndex := testState.Cursor
		stepInfo.EndIndex = endIndex

		consumedBytes := ""
		if endIndex > startIndex && endIndex <= len(testState.Data) {
			consumedBytes = hex.EncodeToString(testState.Data[startIndex:endIndex])
		} else if endIndex == startIndex {
			consumedBytes = ""
		} else {
			log.Warn("光标位置异常，可能回退或超出范围", zap.Int("start", startIndex), zap.Int("end", endIndex))
			consumedBytes = fmt.Sprintf("[光标异常: start=%d, end=%d]", startIndex, endIndex)
		}
		stepInfo.ConsumedBytes = consumedBytes

		stepInfo.VarsAfter = deepCopyMap(testState.Env.Vars)

		if processingErr != nil {
			log.Error("同步处理节点时发生错误",
				zap.Int("nodeIndex", nodeIndex),
				zap.String("label", nodeLabel),
				zap.Stringer("node", current),
				zap.Error(processingErr))
			stepInfo.Error = processingErr.Error()
			processingSteps = append(processingSteps, stepInfo)
			break
		}

		processingSteps = append(processingSteps, stepInfo)

		current = tmp
		nodeIndex++
	}
	log.Debug("同步处理循环结束", zap.Error(processingErr), zap.Int("stepsRecorded", len(processingSteps)), zap.Int("pointsCollected", len(testState.Env.Points)), zap.Int("finalCursor", testState.Cursor), zap.Any("finalEnvVars", testState.Env.Vars))

	parserEndTime := time.Now()
	processingTimeNs := parserEndTime.Sub(startTime).Nanoseconds()

	finalCollectedPoints := testState.Env.Points

	resultPackage := &pkg.PointPackage{
		FrameId: fmt.Sprintf("test_%d", startTime.UnixNano()),
		Points:  finalCollectedPoints,
		Ts:      parserEndTime,
	}

	// --- New Dispatcher Handling ---
	var dispatcherResults []StrategyResult
	strategyConfigs := []pkg.StrategyConfig{}

	if request.ProtocolConfig != nil {
		if strategyCfgMaps, ok := request.ProtocolConfig["strategy"].([]interface{}); ok && len(strategyCfgMaps) > 0 {
			log.Debug("使用协议中的strategy配置构建StrategyConfig")
			for _, scm := range strategyCfgMaps {
				if strategyMap, ok := scm.(map[string]interface{}); ok {
					strategyConf := pkg.StrategyConfig{}
					if stype, ok := strategyMap["type"].(string); ok {
						strategyConf.Type = stype
					}
					if enable, ok := strategyMap["enable"].(bool); ok {
						strategyConf.Enable = enable
					} else {
						strategyConf.Enable = true // Default to enabled if not specified
					}
					if filters, ok := strategyMap["filter"].([]interface{}); ok {
						for _, f := range filters {
							if filterStr, ok := f.(string); ok { // Assuming filters are strings now
								strategyConf.Filter = append(strategyConf.Filter, filterStr)
							}
						}
					}
					if len(strategyConf.Filter) == 0 { // Default filter if none provided
						strategyConf.Filter = []string{"true"}
					}
					if strategyConf.Type != "" && strategyConf.Enable { // Only add enabled strategies with a type
						strategyConfigs = append(strategyConfigs, strategyConf)
					}
				}
			}
		}
	}

	if len(strategyConfigs) == 0 { // Fallback to default strategies if none parsed from config
		log.Debug("无有效策略从ProtocolConfig解析，使用默认策略配置")
		strategyConfigs = []pkg.StrategyConfig{
			{Type: "default_all", Filter: []string{"true"}, Enable: true},
			{Type: "default_dev2", Filter: []string{`Tag.id == "dev2"`}, Enable: true}, // Example filter
		}
	}
	log.Info("最终使用的Strategy配置", zap.Any("strategyConfigs", strategyConfigs))

	dispatchHandler, err := dispatcher.NewHandler(strategyConfigs)
	if err != nil {
		log.Error("创建Dispatcher Handler失败", zap.Error(err))
		// Consider how to report this error; for now, dispatcherResults will be empty
	} else {
		if resultPackage != nil && len(resultPackage.Points) > 0 {
			dispatchedPkgs, dispatchErr := dispatchHandler.Dispatch(resultPackage)
			if dispatchErr != nil {
				log.Warn("Dispatcher Handler处理点数据失败", zap.Error(dispatchErr))
			} else {
				for strategyName, pointPackage := range dispatchedPkgs {
					if pointPackage != nil && len(pointPackage.Points) > 0 {
						dispatcherResults = append(dispatcherResults, StrategyResult{
							StrategyName: strategyName,
							Points:       pointPackage.Points,
						})
					}
				}
				dispatcherEndTime := time.Now()
				processingTimeNs = dispatcherEndTime.Sub(startTime).Nanoseconds() // Update total processing time
				log.Debug("Dispatcher Handler处理完成", zap.Int64("updatedProcessingTimeNs", processingTimeNs))
			}
		} else {
			log.Warn("Parser未生成有效的Point数据，跳过Dispatcher Handler处理", zap.Int("pointsCount", len(finalCollectedPoints)))
		}
	}
	// --- End New Dispatcher Handling ---

	finalVars := make(map[string]interface{})
	if request.InitialVars != nil {
		for k, v := range request.InitialVars {
			finalVars[k] = v
		}
	}

	if byteParser.Env != nil && byteParser.Env.Vars != nil {
		for k, v := range byteParser.Env.Vars {
			finalVars[k] = v
		}
	}
	log.Debug("最终变量状态", zap.Any("finalVars", finalVars))

	finalCursor := testState.Cursor

	debugInfo := make(map[string]interface{})
	debugInfo["timestamp"] = time.Now().UnixNano()
	if resultPackage != nil {
		debugInfo["frame_id"] = resultPackage.FrameId
		debugInfo["points_count"] = len(resultPackage.Points)
	} else {
		debugInfo["points_count"] = len(finalCollectedPoints)
	}
	debugInfo["processed_node_count"] = processedNodeCount
	debugInfo["final_cursor"] = finalCursor

	finalProcessingTime := processingTimeNs
	if finalProcessingTime == 0 {
		finalProcessingTime = 1
	}

	response := TestSectionResponse{
		Points:            finalCollectedPoints,
		FinalVars:         finalVars,
		FinalCursor:       finalCursor,
		TotalBytes:        request.HexPayload,
		ProcessingSteps:   processingSteps,
		ProcessingTime:    finalProcessingTime,
		Debug:             debugInfo,
		DispatcherResults: dispatcherResults,
	}

	log.Info("Section test successful (synchronous with steps)",
		zap.Int("stepsRecorded", len(processingSteps)),
		zap.Int("bytesInPayload", len(byteData)),
		zap.Int("pointsGenerated", len(response.Points)),
		zap.Any("finalVars", response.FinalVars),
		zap.Int("finalCursor", response.FinalCursor),
		zap.Int64("processingTimeNs", response.ProcessingTime),
		zap.Int("dispatcherResultsCount", len(dispatcherResults)),
	)

	if processingErr != nil {
		log.Warn("处理完成，但过程中出现错误", zap.Error(processingErr))
	}

	c.JSON(http.StatusOK, response)
}
