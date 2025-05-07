package api

import (
	"errors"
	"gateway/internal/admin/db"
	"gateway/internal/admin/model"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// --- Protocol GlobalMap Handlers (Nested under Protocol) ---

// GetProtocolGlobalMaps 获取指定协议的全局映射列表
func GetProtocolGlobalMaps(c *gin.Context) {
	protocolId := c.Param("protocolId")
	globalmaps, err := db.GetGlobalMapsByProtocolID(protocolId)
	if err != nil {
		if err.Error() == "无效的协议 ID 格式" {
			errorResponse(c, http.StatusBadRequest, err.Error())
		} else {
			errorResponse(c, http.StatusInternalServerError, "获取全局映射列表失败: "+err.Error())
		}
		return
	}
	c.JSON(http.StatusOK, globalmaps)
}

// CreateProtocolGlobalMap 为特定协议创建新的全局映射
func CreateProtocolGlobalMap(c *gin.Context) {
	protocolId := c.Param("protocolId")
	var newGlobalMap model.GlobalMap
	if err := c.ShouldBindJSON(&newGlobalMap); err != nil {
		errorResponse(c, http.StatusBadRequest, "无效的请求数据: "+err.Error())
		return
	}

	// 名称不能为空
	newGlobalMap.Name = strings.TrimSpace(newGlobalMap.Name)
	if newGlobalMap.Name == "" {
		errorResponse(c, http.StatusBadRequest, "全局映射名称不能为空")
		return
	}

	// 将 URL 中的 protocolId 设置到新全局映射对象中
	objPID, err := primitive.ObjectIDFromHex(protocolId)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "无效的协议 ID 格式")
		return
	}
	newGlobalMap.ProtocolID = objPID
	// 清除 ID 和时间戳
	newGlobalMap.ID = primitive.NilObjectID
	newGlobalMap.CreatedAt = primitive.DateTime(0)
	newGlobalMap.UpdatedAt = primitive.DateTime(0)

	// 如果内容为空，初始化为空对象
	if newGlobalMap.Content == nil {
		newGlobalMap.Content = make(map[string]interface{})
	}

	createdGlobalMap, err := db.CreateGlobalMap(&newGlobalMap)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "创建全局映射失败: "+err.Error())
		return
	}
	c.JSON(http.StatusCreated, createdGlobalMap)
}

// --- Standalone GlobalMap Handlers ---

// GetGlobalMapByID 获取单个全局映射详情
func GetGlobalMapByID(c *gin.Context) {
	globalMapID := c.Param("globalMapId")
	globalMap, err := db.GetGlobalMapByID(globalMapID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || err.Error() == "无效的全局映射 ID 格式" {
			errorResponse(c, http.StatusNotFound, "全局映射未找到")
		} else {
			errorResponse(c, http.StatusInternalServerError, "获取全局映射详情失败: "+err.Error())
		}
		return
	}
	if globalMap == nil {
		errorResponse(c, http.StatusNotFound, "全局映射未找到")
		return
	}
	c.JSON(http.StatusOK, globalMap)
}

// UpdateGlobalMap 更新全局映射信息
func UpdateGlobalMap(c *gin.Context) {
	globalMapID := c.Param("globalMapId")
	var updatePayload model.GlobalMap
	if err := c.ShouldBindJSON(&updatePayload); err != nil {
		errorResponse(c, http.StatusBadRequest, "无效的请求数据: "+err.Error())
		return
	}

	// 名称不能为空检查
	updatePayload.Name = strings.TrimSpace(updatePayload.Name)
	if updatePayload.Name == "" {
		errorResponse(c, http.StatusBadRequest, "全局映射名称不能为空")
		return
	}

	updatedGlobalMap, err := db.UpdateGlobalMap(globalMapID, &updatePayload)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || err.Error() == "无效的全局映射 ID 格式" {
			errorResponse(c, http.StatusNotFound, "全局映射未找到")
		} else {
			errorResponse(c, http.StatusInternalServerError, "更新全局映射失败: "+err.Error())
		}
		return
	}
	if updatedGlobalMap == nil {
		errorResponse(c, http.StatusNotFound, "全局映射未找到")
		return
	}
	c.JSON(http.StatusOK, updatedGlobalMap)
}

// DeleteGlobalMap 删除全局映射
func DeleteGlobalMap(c *gin.Context) {
	globalMapID := c.Param("globalMapId")
	err := db.DeleteGlobalMap(globalMapID)
	if err != nil {
		if err.Error() == "未找到要删除的全局映射" || err.Error() == "无效的全局映射 ID 格式" {
			errorResponse(c, http.StatusNotFound, err.Error())
		} else {
			errorResponse(c, http.StatusInternalServerError, "删除全局映射失败: "+err.Error())
		}
		return
	}
	c.Status(http.StatusNoContent) // 成功删除，返回 204
}
