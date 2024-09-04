package main

import (
	"fmt"
	"go.uber.org/zap"
	"gw22-train-sam/config"
	"gw22-train-sam/logger"
	"gw22-train-sam/plugin"
)

func main() {
	// 1. 初始化config
	Common, Proto, err := config.NewConfig("config")
	if Common == nil || Proto == nil || err != nil {
		fmt.Printf("[main]加载配置失败: %s\n", err)
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
	logger.Log.Infof("[main]配置&日志加载成功:当前Proto配置为%+v", Proto)

	// 3. 初始化脚本模块
	err = plugin.LoadAllScripts(Common.Script.ScriptDir, Common.Script.Methods)
	if err != nil {
		logger.Log.Errorf("[main]加载脚本失败: %s", err)
	}
	logger.Log.Infof("已加载脚本:%v", plugin.ScriptFuncCache)
	// 4. 初始化所有物模型相关

	// 5. 初始化所有正则结果
	// 6. 创建所有管道
	// 7. 启动tcp fetch协程
	// 8. 启动解析+发送协程
	// 9. 启动命令行终止监听协程
	// TODO
}
