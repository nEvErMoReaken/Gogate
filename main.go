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
	}(logger.SugarLogger)
	logger.SugarLogger.Infof("[main]日志加载成功: %+v", viper)

}
