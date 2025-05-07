package api

import (
	"encoding/json" // 导入JSON包用于打印
	"errors"
	"fmt" // 导入fmt包
	"gateway/internal/admin/db"
	"gateway/internal/admin/model" // 确保导入 model

	// 导入日志包
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	// "gopkg.in/yaml.v3" // 不再需要 YAML 转换
)

// GetVersionDefinitionHandler 处理获取协议定义的请求 (返回 JSON)
func GetVersionDefinitionHandler(c *gin.Context) {
	versionIDStr := c.Param("versionId")
	if versionIDStr == "" {
		errorResponse(c, http.StatusBadRequest, "缺少版本 ID")
		return
	}

	// 直接使用 model.ProtocolDefinition
	definition, err := db.GetVersionDefinition(versionIDStr)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || err.Error() == "无效的版本 ID 格式" {
			errorResponse(c, http.StatusNotFound, "未找到指定版本的协议定义")
		} else {
			errorResponse(c, http.StatusInternalServerError, "获取协议定义失败: "+err.Error())
		}
		return
	}

	// 检查 map 是否为 nil 或长度为 0
	if definition == nil || len(definition) == 0 {
		// 如果定义为空，创建一个空的 map 作为默认值
		definition = make(model.ProtocolDefinition) // 使用 model 类型初始化
		// 尝试从版本信息获取协议名作为 key
		version, _ := db.GetVersionByID(versionIDStr)
		protocolName := "unknown_protocol"
		if version != nil {
			protocol, _ := db.GetProtocolByID(version.ProtocolID.Hex())
			if protocol != nil {
				protocolName = protocol.Name
			}
		}
		// 初始化协议名下的步骤列表为空 slice (使用 model.ProtocolDefinitionStep)
		definition[protocolName] = []model.ProtocolDefinitionStep{}
	}

	// 不再序列化为 YAML，直接返回 JSON
	c.JSON(http.StatusOK, definition)
}

// UpdateVersionDefinitionHandler 处理更新协议定义的请求 (接收 JSON)
func UpdateVersionDefinitionHandler(c *gin.Context) {
	versionIDStr := c.Param("versionId")
	if versionIDStr == "" {
		errorResponse(c, http.StatusBadRequest, "缺少版本 ID")
		return
	}

	var definition model.ProtocolDefinition
	if err := c.ShouldBindJSON(&definition); err != nil {
		fmt.Printf("[ERROR] 绑定协议定义JSON失败: %v\n", err) // 使用 fmt
		errorResponse(c, http.StatusBadRequest, "无效的 JSON 请求数据: "+err.Error())
		return
	}

	// --- 使用 fmt 打印日志 ---
	definitionJSON, errJSON := json.MarshalIndent(definition, "", "  ")
	if errJSON != nil {
		fmt.Printf("[WARN] 序列化绑定后的 definition 为 JSON 失败（用于日志）: %v\n", errJSON) // 使用 fmt
	} else {
		fmt.Printf("[INFO] 成功绑定到 model.ProtocolDefinition，内容如下:\n%s\n", string(definitionJSON)) // 使用 fmt
	}

	fmt.Println("[INFO] 检查解析后的Next规则:")
	for key, steps := range definition {
		fmt.Printf("[INFO] Protocol Key: %s\n", key)
		for i, step := range steps {
			fmt.Printf("[INFO]   Step Index: %d\n", i)
			if step.Next != nil {
				for j, rule := range step.Next {
					fmt.Printf("[INFO]     Rule Index: %d, Condition: '%s', Target: '%s'\n", j, rule.Condition, rule.Target)
				}
			} else {
				fmt.Printf("[INFO]     Step %d has no Next rules\n", i)
			}
		}
	}
	// --- 结束日志 ---

	if definition == nil || len(definition) == 0 {
		fmt.Println("[WARN] 解析后的 JSON 协议定义为空") // 使用 fmt
		errorResponse(c, http.StatusBadRequest, "解析后的 JSON 协议定义不能为空")
		return
	}
	// ... (可选的更细致验证) ...

	// 更新数据库
	err := db.UpdateVersionDefinition(versionIDStr, definition) // 直接传递 model.ProtocolDefinition
	if err != nil {
		fmt.Printf("[ERROR] 更新数据库中的协议定义失败: %v, VersionId: %s\n", err, versionIDStr) // 使用 fmt
		if err.Error() == "未找到要更新定义的版本" || err.Error() == "无效的版本 ID 格式" {
			errorResponse(c, http.StatusNotFound, "未找到指定的版本")
		} else {
			errorResponse(c, http.StatusInternalServerError, "更新协议定义失败: "+err.Error())
		}
		return
	}

	fmt.Printf("[INFO] 协议定义更新成功, VersionId: %s\n", versionIDStr) // 使用 fmt
	// 成功响应
	c.JSON(http.StatusOK, gin.H{"message": "协议定义更新成功"})
}
