package api

import (
	"errors"
	"gateway/internal/admin/db"
	"gateway/internal/admin/model"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

// --- Standalone Version Handlers ---

// GetVersionByID 获取单个版本的详细信息 (不常用，但可能需要)
func GetVersionByID(c *gin.Context) {
	versionIDStr := c.Param("versionId")
	version, err := db.GetVersionByID(versionIDStr)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || err.Error() == "无效的版本 ID 格式" {
			errorResponse(c, http.StatusNotFound, "版本未找到")
		} else {
			errorResponse(c, http.StatusInternalServerError, "获取版本详情失败: "+err.Error())
		}
		return
	}
	if version == nil {
		errorResponse(c, http.StatusNotFound, "版本未找到")
		return
	}
	c.JSON(http.StatusOK, version)
}

// UpdateVersion 更新版本基本信息 (version string, description, is_active)
func UpdateVersion(c *gin.Context) {
	versionIDStr := c.Param("versionId")
	var updatePayload model.ProtocolVersion // 绑定 Version, Description, IsActive
	if err := c.ShouldBindJSON(&updatePayload); err != nil {
		errorResponse(c, http.StatusBadRequest, "无效的请求数据: "+err.Error())
		return
	}

	// 版本号不能为空检查
	updatePayload.Version = strings.TrimSpace(updatePayload.Version)
	if updatePayload.Version == "" {
		errorResponse(c, http.StatusBadRequest, "版本号不能为空")
		return
	}

	// TODO: 在 DB 层或此处添加版本号在协议下的唯一性检查 (UpdateVersion)

	updatedVersion, err := db.UpdateVersion(versionIDStr, &updatePayload)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || err.Error() == "无效的版本 ID 格式" {
			errorResponse(c, http.StatusNotFound, "版本未找到")
		} else if mongo.IsDuplicateKeyError(err) { // 假设 version 在 protocolId 下有唯一索引
			errorResponse(c, http.StatusConflict, "该协议下已存在相同的版本号")
		} else {
			errorResponse(c, http.StatusInternalServerError, "更新版本失败: "+err.Error())
		}
		return
	}
	if updatedVersion == nil {
		errorResponse(c, http.StatusNotFound, "版本未找到")
		return
	}
	c.JSON(http.StatusOK, updatedVersion)
}

// DeleteVersion 删除单个版本
func DeleteVersion(c *gin.Context) {
	versionIDStr := c.Param("versionId")
	err := db.DeleteVersion(versionIDStr)
	if err != nil {
		if err.Error() == "未找到要删除的版本" || err.Error() == "无效的版本 ID 格式" {
			errorResponse(c, http.StatusNotFound, err.Error())
		} else {
			errorResponse(c, http.StatusInternalServerError, "删除版本失败: "+err.Error())
		}
		return
	}
	c.Status(http.StatusNoContent) // 成功删除，返回 204
}
