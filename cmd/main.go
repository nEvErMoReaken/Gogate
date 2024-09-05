package main

import (
	"go.uber.org/zap"
	"gw22-train-sam/logger"
	"gw22-train-sam/util"
	"log"
)

func main() {
	// 1. 初始化config
	err := initCommon("config")
	if err != nil {
		log.Fatalf("[main] 加载配置失败: %s", err)
		return
	}
	// 2. 初始化log
	logger.InitLogger(
		Common.Log.LogPath,
		Common.Log.MaxSize,
		Common.Log.MaxBackups,
		Common.Log.MaxAge,
		Common.Log.Compress,
	)
	defer func(logger *zap.SugaredLogger) {
		err := logger.Sync()
		if err != nil {
			logger.Errorf("[main]同步日志失败: %s", err)
		}
	}(logger.Log)
	logger.Log.Infof("[main]配置&日志加载成功:当前Common配置为%+v", Common)

	// 3. 初始化脚本模块
	err = util.LoadAllScripts(Common.Script.ScriptDir, Common.Script.Methods)
	if err != nil {
		logger.Log.Errorf("[main]加载脚本失败: %s", err)
	}
	logger.Log.Infof("已加载脚本:%v", util.ScriptFuncCache)

	// 4. 初始化Connector

	// 5. 初始化所有正则结果
	// 6. 创建所有管道
	// 7. 启动tcp fetch协程
	// 8. 启动解析+发送协程
	// 9. 启动命令行终止监听协程
	// TODO
}
