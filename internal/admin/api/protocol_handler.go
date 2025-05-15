package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gateway/internal/admin/db"
	"gateway/internal/admin/model"

	// "gateway/internal/parser" // Parser validation removed for now, add back if needed
	"net/http"
	"strings" // Import strings package for TrimSpace
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"gopkg.in/yaml.v3"
	// "gopkg.in/yaml.v3" // 暂时移除 YAML 依赖，除非 Export 函数需要恢复
)

// Helper function (假设已存在或添加)
func errorResponse(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{"error": message})
}

// --- Protocol Handlers ---

// GetProtocols 获取协议列表 (保持现有实现，仅返回列表项)
func GetProtocols(c *gin.Context) {
	protocols, err := db.GetAllProtocols() // 使用新的 db 函数
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "获取协议列表失败: "+err.Error())
		return
	}
	// 转换成列表项返回，避免传输完整的 config
	listItems := make([]model.ProtocolListItem, 0, len(protocols))
	for _, p := range protocols {
		listItems = append(listItems, model.ProtocolListItem{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			UpdatedAt:   p.UpdatedAt.Time(), // Convert primitive.DateTime to time.Time
		})
	}
	c.JSON(http.StatusOK, listItems)
}

// CreateProtocol 创建新协议 (使用更新后的逻辑)
func CreateProtocol(c *gin.Context) {
	var newProtocol model.Protocol
	if err := c.ShouldBindJSON(&newProtocol); err != nil {
		errorResponse(c, http.StatusBadRequest, "无效的请求数据: "+err.Error())
		return
	}

	// 名称不能为空
	newProtocol.Name = strings.TrimSpace(newProtocol.Name)
	if newProtocol.Name == "" {
		errorResponse(c, http.StatusBadRequest, "协议名称不能为空")
		return
	}
	// 清除 config，不允许在创建时直接传入完整配置
	newProtocol.Config = nil
	// 清除 ID 和时间戳，由 DB 层处理
	newProtocol.ID = primitive.NilObjectID
	newProtocol.CreatedAt = primitive.DateTime(0)
	newProtocol.UpdatedAt = primitive.DateTime(0)

	createdProtocol, err := db.CreateProtocol(&newProtocol)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) { // 假设 name 有唯一索引
			errorResponse(c, http.StatusConflict, "协议名称已存在")
		} else {
			errorResponse(c, http.StatusInternalServerError, "创建协议失败: "+err.Error())
		}
		return
	}
	c.JSON(http.StatusCreated, createdProtocol)
}

// GetProtocolByID 获取单个协议的详细信息 (包含 config)
func GetProtocolByID(c *gin.Context) {
	protocolIDStr := c.Param("protocolId")
	protocol, err := db.GetProtocolByID(protocolIDStr)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || err.Error() == "无效的协议 ID 格式" {
			errorResponse(c, http.StatusNotFound, "协议未找到")
		} else {
			errorResponse(c, http.StatusInternalServerError, "获取协议详情失败: "+err.Error())
		}
		return
	}
	if protocol == nil {
		errorResponse(c, http.StatusNotFound, "协议未找到")
		return
	}
	// 确保返回的数据结构与前端 API Client 期望一致
	c.JSON(http.StatusOK, gin.H{"protocol": protocol})
}

// UpdateProtocol 更新协议基本信息 (name, description) - 不含 config
func UpdateProtocol(c *gin.Context) {
	protocolIDStr := c.Param("protocolId")
	var updatePayload model.Protocol // 只绑定 name 和 description
	if err := c.ShouldBindJSON(&updatePayload); err != nil {
		errorResponse(c, http.StatusBadRequest, "无效的请求数据: "+err.Error())
		return
	}

	// 名称不能为空检查
	updatePayload.Name = strings.TrimSpace(updatePayload.Name)
	if updatePayload.Name == "" {
		errorResponse(c, http.StatusBadRequest, "协议名称不能为空")
		return
	}

	// TODO: 在 DB 层或此处添加名称唯一性检查 (UpdateProtocol)

	updatedProtocol, err := db.UpdateProtocol(protocolIDStr, &updatePayload)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || err.Error() == "无效的协议 ID 格式" {
			errorResponse(c, http.StatusNotFound, "协议未找到")
		} else if mongo.IsDuplicateKeyError(err) { // 假设 name 有唯一索引
			errorResponse(c, http.StatusConflict, "协议名称已存在")
		} else {
			errorResponse(c, http.StatusInternalServerError, "更新协议失败: "+err.Error())
		}
		return
	}
	if updatedProtocol == nil {
		errorResponse(c, http.StatusNotFound, "协议未找到")
		return
	}
	c.JSON(http.StatusOK, updatedProtocol)
}

// UpdateProtocolConfig 更新协议配置 (新增/替换现有)
func UpdateProtocolConfig(c *gin.Context) {
	protocolId := c.Param("protocolId")
	var config model.GatewayConfig // 绑定整个 GatewayConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		errorResponse(c, http.StatusBadRequest, "无效的配置数据: "+err.Error())
		return
	}

	// TODO: 添加对 config 内容的验证逻辑

	updatedProtocol, err := db.UpdateProtocolConfig(protocolId, &config)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || err.Error() == "无效的协议 ID 格式" {
			errorResponse(c, http.StatusNotFound, "协议未找到")
		} else {
			errorResponse(c, http.StatusInternalServerError, "更新协议配置失败: "+err.Error())
		}
		return
	}
	if updatedProtocol == nil {
		errorResponse(c, http.StatusNotFound, "协议未找到")
		return
	}
	c.JSON(http.StatusOK, updatedProtocol)
}

// DeleteProtocol 删除协议 (新增)
func DeleteProtocol(c *gin.Context) {
	protocolId := c.Param("protocolId")
	err := db.DeleteProtocol(protocolId)
	if err != nil {
		if err.Error() == "未找到要删除的协议" || err.Error() == "无效的协议 ID 格式" {
			errorResponse(c, http.StatusNotFound, err.Error())
		} else {
			errorResponse(c, http.StatusInternalServerError, "删除协议失败: "+err.Error())
		}
		return
	}
	c.Status(http.StatusNoContent) // 成功删除，返回 204
}

// --- Protocol Version Handlers (Nested under Protocol) ---

// GetProtocolVersions 获取指定协议的版本列表 (使用更新后的逻辑)
func GetProtocolVersions(c *gin.Context) {
	protocolId := c.Param("protocolId")
	versions, err := db.GetVersionsByProtocolID(protocolId)
	if err != nil {
		if err.Error() == "无效的协议 ID 格式" {
			errorResponse(c, http.StatusBadRequest, err.Error())
		} else {
			errorResponse(c, http.StatusInternalServerError, "获取版本列表失败: "+err.Error())
		}
		return
	}
	c.JSON(http.StatusOK, versions)
}

// CreateProtocolVersion 为特定协议创建新版本 (使用更新后的逻辑)
func CreateProtocolVersion(c *gin.Context) {
	protocolId := c.Param("protocolId")
	var newVersion model.ProtocolVersion
	if err := c.ShouldBindJSON(&newVersion); err != nil {
		errorResponse(c, http.StatusBadRequest, "无效的请求数据: "+err.Error())
		return
	}

	// 版本号不能为空
	newVersion.Version = strings.TrimSpace(newVersion.Version)
	if newVersion.Version == "" {
		errorResponse(c, http.StatusBadRequest, "版本号不能为空")
		return
	}

	// 将 URL 中的 protocolId 设置到新版本对象中
	objPID, err := primitive.ObjectIDFromHex(protocolId)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "无效的协议 ID 格式")
		return
	}
	newVersion.ProtocolID = objPID
	// 清除 ID 和时间戳
	newVersion.ID = primitive.NilObjectID
	newVersion.CreatedAt = primitive.DateTime(0)
	newVersion.UpdatedAt = primitive.DateTime(0)

	// 检查版本号在该协议下是否唯一
	exists, err := db.CheckVersionExists(protocolId, newVersion.Version)
	if err != nil {
		// 处理检查过程中可能发生的数据库错误
		errorResponse(c, http.StatusInternalServerError, "检查版本唯一性时出错: "+err.Error())
		return
	}
	if exists {
		errorResponse(c, http.StatusConflict, "该协议下已存在相同的版本号")
		return
	}

	createdVersion, err := db.CreateVersion(&newVersion)
	if err != nil {
		// 数据库层面的唯一索引错误处理（作为后备）
		// 这里假设 db.CreateVersion 内部没有处理 IsDuplicateKeyError，如果处理了，这里的判断可能多余
		if mongo.IsDuplicateKeyError(err) {
			errorResponse(c, http.StatusConflict, "该协议下已存在相同的版本号")
		} else {
			errorResponse(c, http.StatusInternalServerError, "创建版本失败: "+err.Error())
		}
		return
	}
	c.JSON(http.StatusCreated, createdVersion)
}

// GetProtocolVersionByID 获取单个协议版本
func GetProtocolVersionByID(c *gin.Context) {
	versionIDStr := c.Param("versionId")
	versionObjID, err := primitive.ObjectIDFromHex(versionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的版本 ID 格式"})
		return
	}

	var version model.ProtocolVersion
	collection := db.GetProtocolVersionCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": versionObjID}
	err = collection.FindOne(ctx, filter).Decode(&version)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "未找到指定的协议版本"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取协议版本失败: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, version) // 返回完整的版本对象
}

// UpdateProtocolVersion 更新协议版本
func UpdateProtocolVersion(c *gin.Context) {
	versionIDStr := c.Param("versionId")
	versionObjID, err := primitive.ObjectIDFromHex(versionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的版本 ID 格式"})
		return
	}

	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据: " + err.Error()})
		return
	}

	updateFields := bson.M{}
	var wasActiveSet bool

	// --- Handle Top-Level Fields ---
	if val, ok := payload["version"]; ok {
		updateFields["version"] = val
	}
	if val, ok := payload["description"]; ok {
		updateFields["description"] = val
	}
	if val, ok := payload["isActive"]; ok {
		updateFields["is_active"] = val
		wasActiveSet = true
	}

	// TODO: Decide how to handle parserConfig from payload.
	// It exists in the payload from VersionForm but outside protocolConfig.
	// Current DB schema seems to expect it inside protocol_config.parser?
	// For now, we ignore payload["parserConfig"] to avoid incorrect placement.
	/*
		if pcPayload, ok := payload["parserConfig"].(map[string]interface{}); ok {
			// Option 1: Update a separate top-level field?
			// updateFields["parser_config"] = pcPayload
			// Option 2: Update inside protocol_config?
			// updateFields["protocol_config.parser"] = pcPayload
			// Needs clarification based on intended DB structure.
		}
	*/

	// --- Handle protocolConfig Sub-Fields (Partial Update) ---
	if protoConfPayload, ok := payload["protocolConfig"].(map[string]interface{}); ok {
		// Iterate through keys provided in the payload's protocolConfig
		for key, value := range protoConfPayload {
			// Use dot notation to update specific fields within protocol_config
			dbKey := "protocol_config." + key // e.g., protocol_config.crc
			updateFields[dbKey] = value
			fmt.Printf("Debug Update: Adding field %s to $set\n", dbKey) // Debug log
		}
	} else {
		// If protocolConfig is not in the payload or not a map, we don't update it at all.
		fmt.Println("Debug Update: No valid protocolConfig in payload, skipping related updates.") // Debug log
	}

	// Always update the timestamp
	updateFields["updated_at"] = time.Now()

	collection := db.GetProtocolVersionCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": versionObjID}
	updateDoc := bson.M{"$set": updateFields}

	fmt.Printf("Debug Update: Update document: %v\n", updateDoc) // Debug log

	// Database update and subsequent logic (unchanged)
	updateResult, err := collection.UpdateOne(ctx, filter, updateDoc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新协议版本失败: " + err.Error()})
		return
	}

	if updateResult.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到要更新的版本"})
		return
	}

	// Handle deactivating other versions if isActive was set to true (unchanged)
	if wasActiveSet {
		if isActive, ok := updateFields["is_active"].(bool); ok && isActive {
			var currentVersion model.ProtocolVersion
			readCtx, readCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer readCancel()
			if err := collection.FindOne(readCtx, filter).Decode(&currentVersion); err == nil {
				filterOthers := bson.M{
					"protocol_id": currentVersion.ProtocolID,
					"_id":         bson.M{"$ne": versionObjID},
				}
				updateOthers := bson.M{"$set": bson.M{"is_active": false}}
				updateCtx, updateCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer updateCancel()
				_, updateErr := collection.UpdateMany(updateCtx, filterOthers, updateOthers)
				if updateErr != nil {
					fmt.Printf("Warning: Failed to deactivate other versions for protocol %s: %v\n", currentVersion.ProtocolID.Hex(), updateErr)
				}
			} else {
				fmt.Printf("Warning: Could not fetch protocolId to deactivate other versions: %v\n", err)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "协议版本更新成功",
	})
}

// Helper function to get map keys for debugging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Helper function to recursively remove empty maps from data structure
func cleanValueForYAML(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		cleanedMap := make(map[string]interface{})
		for key, val := range v {
			cleanedVal := cleanValueForYAML(val) // Recursively clean the value
			// Keep the key only if the cleaned value is not nil
			if cleanedVal != nil {
				// Check if the cleaned value is itself an empty map
				if subMap, ok := cleanedVal.(map[string]interface{}); ok && len(subMap) == 0 {
					continue // Skip adding if it's an empty map
				}
				cleanedMap[key] = cleanedVal
			}
		}
		// If the cleaned map itself is empty, return nil so the parent can remove it
		if len(cleanedMap) == 0 {
			return nil
		}
		return cleanedMap
	case []interface{}:
		// Only keep non-nil items after cleaning
		if len(v) == 0 { // Handle empty slice directly
			return nil
		}
		cleanedSlice := make([]interface{}, 0, len(v))
		for _, item := range v {
			cleanedItem := cleanValueForYAML(item)
			if cleanedItem != nil {
				cleanedSlice = append(cleanedSlice, cleanedItem)
			}
		}
		// If the cleaned slice becomes empty, return nil
		if len(cleanedSlice) == 0 {
			return nil
		}
		return cleanedSlice
	default:
		// Return non-map/slice values as is (including nil, strings, numbers, bools)
		return v
	}
}

// ExportProtocolVersionYaml 将协议版本配置导出为 YAML 格式
// 支持查询参数 exportType=protocol (默认) 或 exportType=chunks
func ExportProtocolVersionYaml(c *gin.Context) {
	versionIDStr := c.Param("versionId")
	versionObjID, err := primitive.ObjectIDFromHex(versionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的版本 ID 格式"})
		return
	}

	// 获取导出类型，默认为 "protocol"
	exportType := c.DefaultQuery("exportType", "protocol")
	if exportType != "protocol" && exportType != "chunks" && exportType != "definition" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的导出类型，必须是 'protocol'、'chunks' 或 'definition'"})
		return
	}

	var version model.ProtocolVersion
	collection := db.GetProtocolVersionCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = collection.FindOne(ctx, bson.M{"_id": versionObjID}).Decode(&version)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "未找到指定的版本"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取版本数据失败: " + err.Error()})
		}
		return
	}

	var protocol model.Protocol
	protocolCollection := db.GetProtocolCollection()
	protoCtx, protoCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer protoCancel()
	err = protocolCollection.FindOne(protoCtx, bson.M{"_id": version.ProtocolID}).Decode(&protocol)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "未找到所属协议"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取协议数据失败: " + err.Error()})
		}
		return
	}

	// --- JSON Roundtrip for ProtocolConfig (Fetch from Protocol) ---
	var cleanProtocolConfig map[string]interface{}
	// Use the fetched protocol.Config for export
	if protocol.Config != nil {
		// Convert protocol.Config (which is *model.GatewayConfig) to map[string]interface{} via JSON roundtrip
		configJSONBytes, err := json.Marshal(protocol.Config)
		if err != nil {
			fmt.Printf("Error marshalling protocol.Config to JSON: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "处理协议配置时出错 (JSON Marshal)"})
			return
		}
		err = json.Unmarshal(configJSONBytes, &cleanProtocolConfig)
		if err != nil {
			fmt.Printf("Error unmarshalling protocol.Config from JSON: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "处理协议配置时出错 (JSON Unmarshal)"})
			return
		}
		fmt.Println("Debug Export: Successfully performed JSON roundtrip for protocol.Config.")
	} else {
		cleanProtocolConfig = make(map[string]interface{}) // Use empty map if protocol.Config is nil
		fmt.Println("Debug Export: protocol.Config was nil, using empty map.")
	}
	// --------------------------------------

	exportData := make(map[string]interface{}) // Initialize empty export data
	var protoFileName string
	var yamlHeader string
	var fileNameSuffix string

	// --- Determine Proto Filename ---
	if parserConfRaw, ok := cleanProtocolConfig["parser"]; ok {
		if pcMap, ok := parserConfRaw.(map[string]interface{}); ok {
			if configMap, ok := pcMap["config"].(map[string]interface{}); ok {
				if pf, ok := configMap["protoFile"].(string); ok && pf != "" {
					protoFileName = pf
				}
			}
		}
	}
	if protoFileName == "" {
		protoFileName = fmt.Sprintf("%s.proto", protocol.Name)
	}

	// --- Populate Export Data based on exportType ---
	if exportType == "protocol" {
		fileNameSuffix = "protocol"
		yamlHeader = fmt.Sprintf("# Protocol: %s - v%s\n", protocol.Name, version.Version)
		if version.Description != "" {
			yamlHeader += fmt.Sprintf("# Description: %s\n", version.Description)
		}

		exportData["version"] = version.Version

		// --- 处理 Parser 配置 ---
		var parserConfigData map[string]interface{}
		if parserConfRaw, ok := cleanProtocolConfig["parser"]; ok {
			if pcMap, ok := parserConfRaw.(map[string]interface{}); ok {
				parserConfigData = pcMap // Already clean map
			}
		}
		if parserConfigData == nil {
			parserConfigData = map[string]interface{}{
				"config": map[string]interface{}{"dir": "."},
			}
		}
		if configMap, ok := parserConfigData["config"].(map[string]interface{}); ok {
			configMap["protoFile"] = protoFileName // Ensure protoFile is set
		} else {
			parserConfigData["config"] = map[string]interface{}{"protoFile": protoFileName, "dir": "."}
		}
		exportData["parser"] = parserConfigData

		// --- 处理其他顶层配置 ---
		if connectorConfig, ok := cleanProtocolConfig["connector"]; ok {
			exportData["connector"] = connectorConfig
		} else {
			exportData["connector"] = map[string]interface{}{"type": "TCPServer", "config": map[string]interface{}{"host": "0.0.0.0", "port": 8000}}
		}
		if dispatcherConfig, ok := cleanProtocolConfig["dispatcher"]; ok {
			exportData["dispatcher"] = dispatcherConfig
		}
		if strategyConfig, ok := cleanProtocolConfig["strategy"]; ok {
			exportData["strategy"] = strategyConfig
		} else {
			exportData["strategy"] = []interface{}{}
		}
		if logConfig, ok := cleanProtocolConfig["log"]; ok {
			exportData["log"] = logConfig
		} else {
			exportData["log"] = map[string]interface{}{"log_path": "./logs", "level": "info"}
		}
		// NOTE: Chunks are explicitly NOT included here

	} else if exportType == "chunks" {
		fileNameSuffix = "chunks"
		yamlHeader = fmt.Sprintf("# Protocol: %s - v%s - Chunks\n", protocol.Name, version.Version)
		// --- 处理 Chunks 配置 ---
		var chunksSlice []interface{}
		if chunksConfigRaw, keyExists := cleanProtocolConfig["chunks"]; keyExists {
			fmt.Printf("Debug Export (Chunks): Found 'chunks' key. Type: %T\n", chunksConfigRaw)
			if cs, ok := chunksConfigRaw.([]interface{}); ok && len(cs) > 0 {
				chunksSlice = cs
				exportData[protoFileName] = map[string]interface{}{
					"chunks": chunksSlice,
				}
				fmt.Printf("Debug Export (Chunks): Added clean chunks. Count: %d\n", len(chunksSlice))
			} else if ok {
				fmt.Println("Debug Export (Chunks): Clean chunks data is empty.")
				// Optionally return 404 or empty file if no chunks exist
				// c.JSON(http.StatusNotFound, gin.H{"error": "此版本没有 Chunks 配置"})
				// return
			} else {
				fmt.Printf("Debug Export (Chunks): Failed to assert 'chunks' as []interface{}. Type: %T\n", chunksConfigRaw)
			}
		} else {
			fmt.Printf("Debug Export (Chunks): Key 'chunks' not found. Keys: %v\n", getMapKeys(cleanProtocolConfig))
			// Optionally return 404 or empty file
			// c.JSON(http.StatusNotFound, gin.H{"error": "此版本没有 Chunks 配置"})
			// return
		}
		// If exportData remains empty (no chunks found), we'll export an empty structure
		if len(exportData) == 0 {
			fmt.Println("Debug Export (Chunks): No chunks data found, exporting empty structure.")
			// Ensure the structure { protoFileName: { chunks: [] } } is exported even if empty
			exportData[protoFileName] = map[string]interface{}{
				"chunks": []interface{}{},
			}
		}
	} else if exportType == "definition" {
		// 处理 Definition 配置导出
		fileNameSuffix = "sections"
		yamlHeader = fmt.Sprintf("# Protocol: %s - v%s - Section Orchestration\n", protocol.Name, version.Version)
		if version.Description != "" {
			yamlHeader += fmt.Sprintf("# Description: %s\n", version.Description)
		}

		// 使用版本的 Definition 字段
		if version.Definition != nil && len(version.Definition) > 0 {
			fmt.Println("Debug Export (Definition): Found valid Definition data")
			// 转换 Definition 为 map[string]interface{} 类型
			definitionJSON, err := json.Marshal(version.Definition)
			if err != nil {
				fmt.Printf("Error marshalling version.Definition to JSON: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "处理版本定义时出错 (JSON Marshal)"})
				return
			}

			var definitionMap map[string]interface{}
			err = json.Unmarshal(definitionJSON, &definitionMap)
			if err != nil {
				fmt.Printf("Error unmarshalling version.Definition from JSON: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "处理版本定义时出错 (JSON Unmarshal)"})
				return
			}

			exportData = definitionMap
		} else {
			fmt.Println("Debug Export (Definition): No Definition data found, exporting empty structure")
			// 如果 Definition 为空，导出空结构
			// 尝试使用协议名称作为键
			exportData = map[string]interface{}{
				protocol.Name: []interface{}{},
			}
		}
	}

	// *** NEW: Clean the final exportData before marshalling ***
	cleanedExportData := cleanValueForYAML(exportData)
	if cleanedExportData == nil {
		cleanedExportData = map[string]interface{}{}
	}

	// --- 生成 YAML 和 HTTP 响应 ---
	yamlHeader += fmt.Sprintf("# Export Type: %s\n", exportType)
	yamlHeader += fmt.Sprintf("# Generated At: %s\n---\n", time.Now().Format("2006-01-02 15:04:05"))

	yamlData, err := yaml.Marshal(cleanedExportData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 YAML 失败: " + err.Error()})
		return
	}

	finalYaml := yamlHeader + string(yamlData)
	// Use the determined suffix for the filename
	fileName := fmt.Sprintf("%s-v%s-%s.yaml", protocol.Name, version.Version, fileNameSuffix)

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Header("Content-Type", "application/x-yaml")
	c.Header("Content-Length", fmt.Sprintf("%d", len(finalYaml)))
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Cache-Control", "no-cache")

	c.Writer.Write([]byte(finalYaml))
}

// Placeholder for Delete handlers if needed later
