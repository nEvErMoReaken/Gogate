package main

import (
	"context"
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
	config, v, err := pkg.InitCommon("yaml")
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
	log.Info("程序启动", zap.String("version", config.Version))
	log.Info("配置信息", zap.Any("common", config))
	log.Info("*** 初始化流程开始 ***")

	// 3. 初始化脚本模块
	err = util.LoadAllScripts(config.Script.ScriptDir)
	if err != nil {
		log.Panic("加载脚本失败", zap.Error(err))
	}
	log.Info("已加载脚本", zap.Any("scripts", util.ByteScriptFuncCache))

	// 4. 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	// 将config挂载到ctx上
	ctxWithConfig := pkg.WithConfig(ctx, config)
	// 将logger挂载到ctx上

	// 5. 启动策略模块
	strategy.RunStrategy(pkg.WithLogger(ctxWithConfig, log, "Strategy"))

	// 6. 启动连接器
	err = connector.RunConnector(config, config.Connector.Type, ctx)
	if err != nil {
		log.Panic("启动Connector失败: %s", zap.Error(err))
	}

	// 6. 监听终止信号
	si := make(chan os.Signal, 1)
	signal.Notify(si, os.Interrupt, syscall.SIGTERM)
	<-si
	log.Info("Caught exit signal, so close channel chDone.")
	cancel() // 关闭 ctx

	log.Info("Exiting gateway...")
	os.Exit(0) // 安全退出程序
}
