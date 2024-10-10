package main

import (
	"fmt"
	"gateway/internal/connector"
	_ "gateway/internal/parser/ioReader"
	"gateway/internal/pkg"
	"gateway/internal/strategy"
	"gateway/util"
	"go.uber.org/zap"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// 1. 初始化common config
	comConfig, v, err := pkg.InitCommon("config")
	if err != nil {
		log.Fatalf("[main] 加载配置失败: %s", err)
		return
	}

	// 2. 初始化log
	pkg.InitLogger(&comConfig.LogConfig)
	defer func(logger *zap.SugaredLogger) {
		err := logger.Sync()
		if err != nil {
			logger.Errorf("[main]同步日志失败: %s", err)
		}
	}(pkg.Log)
	pkg.Log.Infof("[main]配置&日志加载成功:当前Common配置为%+v", comConfig)

	// 3. 初始化脚本模块
	err = util.LoadAllScripts(comConfig.Script.ScriptDir)
	if err != nil {
		pkg.Log.Errorf("[main]加载脚本失败: %s", err)
	}
	pkg.Log.Infof("已加载脚本:%v", util.ByteScriptFuncCache)

	// 4. 启动所有发送策略
	chDone := make(chan struct{})
	strategy.RunStrategy(comConfig, chDone)

	// 5. 启动所有注册的Connector
	err1 := connector.RunConnector(comConfig, comConfig.Connector.Type, v, chDone)
	if err1 != nil {
		pkg.Log.Fatalf("[main]启动Connector失败: %s", err1)
	}

	// 6. 监听终止信号
	si := make(chan os.Signal, 1)
	signal.Notify(si, os.Interrupt)
	signal.Notify(si, syscall.SIGTERM)
	go func() {
		<-si
		fmt.Printf("%s [main] Caught exit signal, so close channel chDone.\n", time.Now().Format(time.RFC3339Nano))
		pkg.Log.Info("[main] Caught exit signal, so close channel chDone.")
		close(chDone) // 关闭 chDone 通道
	}()
	<-chDone // 等待 chDone 通道关闭
	fmt.Printf("%s [main] Caught exit signal, so close channel chDone.\n", time.Now().Format(time.RFC3339Nano))
	pkg.Log.Info("[main] Caught exit signal, so close channel chDone.")
	close(chDone)
	fmt.Println("Exiting gateway...")
	os.Exit(0) // 安全退出程序
}
