package main

import (
	"fmt"
	"go.uber.org/zap"
	"gw22-train-sam/common"
	_ "gw22-train-sam/dataSource/byteType"
	"gw22-train-sam/model"
	"gw22-train-sam/strategy"
	"gw22-train-sam/util"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// 1. 初始化config
	comConfig, v, err := common.InitCommon("config")
	if err != nil {
		log.Fatalf("[main] 加载配置失败: %s", err)
		return
	}
	// 2. 初始化log
	common.InitLogger(&comConfig.LogConfig)
	defer func(logger *zap.SugaredLogger) {
		err := logger.Sync()
		if err != nil {
			logger.Errorf("[main]同步日志失败: %s", err)
		}
	}(common.Log)
	common.Log.Infof("[main]配置&日志加载成功:当前Common配置为%+v", comConfig)

	// 3. 初始化脚本模块
	err = util.LoadAllScripts(comConfig.Script.ScriptDir)
	if err != nil {
		common.Log.Errorf("[main]加载脚本失败: %s", err)
	}
	common.Log.Infof("已加载脚本:%v", util.ScriptFuncCache)
	// 4. 启动所有发送策略
	chDone := make(chan struct{})
	strategy.RunStrategy(comConfig, chDone)
	// 5. 启动所有注册的Connector
	err1 := model.RunConnector(comConfig, comConfig.Connector.Type, v, chDone)
	if err1 != nil {
		common.Log.Fatalf("[main]启动Connector失败: %s", err1)
	}
	// 6. 监听终止信号
	si := make(chan os.Signal, 1)
	signal.Notify(si, os.Interrupt)
	signal.Notify(si, syscall.SIGTERM)
	go func() {
		<-si
		fmt.Printf("%s [main] Caught exit signal, so close channel chDone.\n", time.Now().Format(time.RFC3339Nano))
		common.Log.Info("[main] Caught exit signal, so close channel chDone.")
		close(chDone) // 关闭 chDone 通道
	}()
	<-chDone // 等待 chDone 通道关闭
	fmt.Printf("%s [main] Caught exit signal, so close channel chDone.\n", time.Now().Format(time.RFC3339Nano))
	common.Log.Info("[main] Caught exit signal, so close channel chDone.")
	close(chDone)
	fmt.Println("Exiting gateway...")
	os.Exit(0) // 安全退出程序
}
