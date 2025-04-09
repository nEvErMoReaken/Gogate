package main

import (
	"context"
	"fmt"
	"gateway/internal"
	"gateway/internal/pkg"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// syncLog 安全地同步日志，忽略与标准输出相关的错误
func syncLog(log *zap.Logger) {
	// Windows平台上，同步标准输出时会出现"The handle is invalid"错误
	// 这是zap的已知问题，我们可以安全地忽略它
	err := log.Sync()
	if err != nil && !strings.Contains(err.Error(), "The handle is invalid") {
		log.Error("程序退出时同步日志失败", zap.Error(err))
	}
}

func main() {

	// 1. 初始化common yaml
	config, err := pkg.InitCommon("yaml")
	if err != nil {
		fmt.Printf("[main] 加载配置失败: %s", err)
		return
	}

	// 2. 初始化log
	log := pkg.NewLogger(&config.Log)

	log.Info("程序启动", zap.String("version", config.Version))
	log.Info("配置信息", zap.Any("common", config))
	log.Info("==== 初始化流程开始 ====")

	// 3. 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	errChan := make(chan error, 10) // 创建一个只写的全局错误通道, 缓存大小为10
	ctx = pkg.WithErrChan(ctx, errChan)
	// 将config挂载到ctx上
	ctxWithConfig := pkg.WithConfig(ctx, config)
	// 将logger挂载到ctx上
	ctxWithConfigAndLogger := pkg.WithLogger(ctxWithConfig, log)

	pipeline, err := internal.NewPipeline(ctxWithConfigAndLogger)
	if err != nil {
		log.Error("创建管道失败", zap.Error(err))
		cancel()
		return
	}
	printStartupLogo()
	// 4. 启动管道
	pipeline.Start(ctxWithConfigAndLogger)

	// 5. 主线程监听终止信号
	si := make(chan os.Signal, 1)
	signal.Notify(si, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case <-si:
			log.Info("Caught exit signal, so close channel chDone.")
			log.Info("Exiting gateway...")
			cancel()                    // 取消上下文
			time.Sleep(1 * time.Second) // 给其他协程时间处理取消
			syncLog(log)                // 使用安全的同步函数
			os.Exit(0)                  // 安全退出程序
		case bad := <-errChan:
			log.Error("Error occurred", zap.Error(bad))
			cancel() // 取消上下文
			// 等待其他可能的错误
			go func() {
				for err := range errChan {
					log.Error("Error occurred before shutdown", zap.Error(err))
				}
			}()
			time.Sleep(1 * time.Second) // 确保日志输出完整
			syncLog(log)                // 使用安全的同步函数
			os.Exit(1)
		}
	}
}

func printStartupLogo() {
	logo := `
		 ________  ________  ________  ________  _________  _______
		|\   ____\|\   __  \|\   ____\|\   __  \|\___   ___\\  ___ \
		\ \  \___|\ \  \|\  \ \  \___|\ \  \|\  \|___ \  \_\ \   __/|
		 \ \  \  __\ \  \\\  \ \  \  __\ \   __  \   \ \  \ \ \  \_|/__
		  \ \  \|\  \ \  \\\  \ \  \|\  \ \  \ \  \   \ \  \ \ \  \_|\ \
		   \ \_______\ \_______\ \_______\ \__\ \__\   \ \__\ \ \_______\
			\|_______|\|_______|\|_______|\|__|\|__|    \|__|  \|_______|

`
	fmt.Print(logo)
}
