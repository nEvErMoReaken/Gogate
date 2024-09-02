package main

import (
	"go.uber.org/zap"
	"gw22-train-sam/config"
	"gw22-train-sam/logger"
)

func main() {
	// 1. 初始化config
	viper := config.NewConfig()
	// 2. 初始化log
	logger.InitLogger(
		viper.GetString("log.log_path"),
		viper.GetInt("log.max_size"),
		viper.GetInt("log.max_backups"),
		viper.GetInt("log.max_age"),
		viper.GetBool("log.compress"),
	)
	defer func(logger *zap.SugaredLogger) {
		err := logger.Sync()
		if err != nil {
			logger.Errorf("[main]同步日志失败: %s", err)
		}
	}(logger.Log)
	logger.Log.Infof("[main]日志加载成功: %+v", viper)

	// 3. 初始化脚本模块
	// 4. 初始化所有物模型
	// 5. 初始化所有正则结果
	// 6. 创建所有管道
	// 7. 启动tcp fetch协程
	// 8. 启动解析+发送协程
	// 9. 启动命令行终止监听协程
	// TODO
}
