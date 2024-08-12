package config

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
)

func NewConfig() *viper.Viper {
	v := viper.New()
	// 设置配置文件的目录
	configDir := "./config"
	v.AddConfigPath(configDir)
	v.AutomaticEnv()
	// 获取配置目录下的所有文件
	files, err := os.ReadDir(configDir)
	if err != nil {
		fmt.Println("读取配置文件失败：", err)
	}

	// 遍历所有文件
	for _, file := range files {
		// 获取文件的完整路径
		filePath := filepath.Join(configDir, file.Name())

		// 获取文件的扩展名
		ext := filepath.Ext(filePath)

		// 只处理.yaml文件
		if ext == ".yaml" || ext == ".yml" {
			// 设置配置文件的名称（不包括扩展名）
			baseName := filepath.Base(filePath) // 这里是包含了扩展名的
			configName := baseName[0 : len(baseName)-len(ext)]
			v.SetConfigName(configName)

			// 读取配置文件 (会覆盖之前的配置)
			if err := v.MergeInConfig(); err != nil {
				fmt.Println("读取配置文件失败：", err)
			}
		}
	}
	return v
}
