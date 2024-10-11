package main

import (
	"fmt"
	"gateway/internal/connector"
	_ "gateway/internal/parser/ioReader"
	"gateway/internal/pkg"
	"gateway/internal/strategy"
	"gateway/util"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// 1. 初始化common yaml
	comConfig, v, err := pkg.InitCommon("yaml")
	if err != nil {
		fmt.Printf("[main] 加载配置失败: %s", err)
		return
	}

	// 2. 初始化log
	log := pkg.NewLogger(v)
	defer func(logger *zap.Logger) {
		err := logger.Sync()
		if err != nil {
			logger.Error("程序退出时同步日志失败: %s", zap.Error(err))
		}
	}(log)
	log.Info("程序启动", zap.String("version", comConfig.Version))
	log.Info("配置信息", zap.Any("common", comConfig))
	log.Info("*** 初始化流程开始 ***")
	// 3. 初始化脚本模块
	err = util.LoadAllScripts(comConfig.Script.ScriptDir)
	if err != nil {
		log.Panic("加载脚本失败", zap.Error(err))
	}
	log.Info("已加载脚本", zap.Any("scripts", util.ByteScriptFuncCache))

	// 4. 启动所有发送策略
	chDone := make(chan struct{})
	strategy.RunStrategy(comConfig, chDone)

	// 5. 启动所有注册的Connector
	err = connector.RunConnector(comConfig, comConfig.Connector.Type, chDone)
	if err != nil {
		log.Panic("[main]启动Connector失败: %s", zap.Error(err))
	}

	// 6. 监听终止信号
	si := make(chan os.Signal, 1)
	signal.Notify(si, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-si
		log.Info("[main] Caught exit signal, so close channel chDone.")
		close(chDone) // 关闭 chDone 通道
	}()
	<-chDone // 等待 chDone 通道关闭
	log.Info("Exiting gateway...")
	os.Exit(0) // 安全退出程序
}
