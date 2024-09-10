package main

import (
	"go.uber.org/zap"
	"gw22-train-sam/common"
	"gw22-train-sam/dataSource/byteType/tcpServer"
	"gw22-train-sam/util"
	"log"
)

func main() {
	// 1. 初始化config
	comConfig, v, err := common.InitCommon("config")
	if err != nil {
		log.Fatalf("[main] 加载配置失败: %s", err)
		return
	}
	// 2. 初始化log
	common.InitLogger(
		comConfig.Log.LogPath,
		comConfig.Log.MaxSize,
		comConfig.Log.MaxBackups,
		comConfig.Log.MaxAge,
		comConfig.Log.Compress,
	)
	defer func(logger *zap.SugaredLogger) {
		err := logger.Sync()
		if err != nil {
			logger.Errorf("[main]同步日志失败: %s", err)
		}
	}(common.Log)
	common.Log.Infof("[main]配置&日志加载成功:当前Common配置为%+v", comConfig)

	// 3. 初始化脚本模块
	err = util.LoadAllScripts(comConfig.Script.ScriptDir, comConfig.Script.Methods)
	if err != nil {
		common.Log.Errorf("[main]加载脚本失败: %s", err)
	}
	common.Log.Infof("已加载脚本:%v", util.ScriptFuncCache)

	// 4. 初始化激活的Connector
	chunkList, err := tcpServer.InitChunks(v)
	for _, chunk := range chunkList.Chunks {
		common.Log.Infof("已加载chunk:%v", chunk)
	}
	//fmt.Printf("%+v", chunkList)
	// 5. 初始化所有正则结果
	// 6. 创建所有管道
	// 7. 启动tcp fetch协程
	// 8. 启动解析+发送协程
	// 9. 启动命令行终止监听协程
	// TODO
}
