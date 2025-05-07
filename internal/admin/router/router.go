package router

import (
	"gateway/internal/admin/api"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// SetupRouter 配置 Gin 路由
func SetupRouter() *gin.Engine {
	r := gin.Default()

	// 配置 CORS
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"} // 允许所有来源，生产环境应配置具体来源
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	r.Use(cors.New(config))

	// 健康检查 (可选)
	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// API v1 分组
	apiV1 := r.Group("/api/v1")
	{
		// 协议相关路由
		protocols := apiV1.Group("/protocols")
		{
			protocols.GET("", api.GetProtocols)                  // GET /api/v1/protocols
			protocols.POST("", api.CreateProtocol)               // POST /api/v1/protocols
			protocols.GET("/:protocolId", api.GetProtocolByID)   // GET /api/v1/protocols/:protocolId
			protocols.PUT("/:protocolId", api.UpdateProtocol)    // PUT /api/v1/protocols/:protocolId
			protocols.DELETE("/:protocolId", api.DeleteProtocol) // DELETE /api/v1/protocols/:protocolId

			// 协议配置路由 (新增)
			protocols.PUT("/:protocolId/config", api.UpdateProtocolConfig) // PUT /api/v1/protocols/:protocolId/config

			// 协议版本相关路由 (嵌套在协议下)
			versions := protocols.Group("/:protocolId/versions")
			{
				versions.GET("", api.GetProtocolVersions)    // GET /api/v1/protocols/:protocolId/versions
				versions.POST("", api.CreateProtocolVersion) // POST /api/v1/protocols/:protocolId/versions
				// 导出路由
				versions.GET("/:versionId/export", api.ExportProtocolVersionYaml) // GET /api/v1/protocols/:protocolId/versions/:versionId/export
			}

			// GlobalMap相关路由 (嵌套在协议下)
			globalmaps := protocols.Group("/:protocolId/globalmaps")
			{
				globalmaps.GET("", api.GetProtocolGlobalMaps)    // GET /api/v1/protocols/:protocolId/globalmaps
				globalmaps.POST("", api.CreateProtocolGlobalMap) // POST /api/v1/protocols/:protocolId/globalmaps
			}
		}

		// 独立版本路由 (用于直接通过 ID 操作版本)
		standaloneVersions := apiV1.Group("/versions")
		{
			standaloneVersions.GET("/:versionId", api.GetVersionByID)   // GET /api/v1/versions/:versionId (获取版本基本信息)
			standaloneVersions.PUT("/:versionId", api.UpdateVersion)    // PUT /api/v1/versions/:versionId (更新版本基本信息)
			standaloneVersions.DELETE("/:versionId", api.DeleteVersion) // DELETE /api/v1/versions/:versionId

			// 新增: 协议定义接口
			standaloneVersions.GET("/:versionId/definition", api.GetVersionDefinitionHandler)    // GET /api/v1/versions/:versionId/definition
			standaloneVersions.PUT("/:versionId/definition", api.UpdateVersionDefinitionHandler) // PUT /api/v1/versions/:versionId/definition
		}

		// 独立全局映射路由 (用于直接通过 ID 操作全局映射)
		standaloneGlobalMaps := apiV1.Group("/globalmaps")
		{
			standaloneGlobalMaps.GET("/:globalMapId", api.GetGlobalMapByID)   // GET /api/v1/globalmaps/:globalMapId
			standaloneGlobalMaps.PUT("/:globalMapId", api.UpdateGlobalMap)    // PUT /api/v1/globalmaps/:globalMapId
			standaloneGlobalMaps.DELETE("/:globalMapId", api.DeleteGlobalMap) // DELETE /api/v1/globalmaps/:globalMapId
		}

		// 测试路由
		testGroup := apiV1.Group("/test")
		{
			// 指向 api 包中的 TestSectionHandler
			testGroup.POST("/section", api.TestSectionHandler) // POST /api/v1/test/section
		}
	}

	return r
}
