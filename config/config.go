package config

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
)

func NewConfig(configDir string) (*Common, *Proto, error) {
	v := viper.NewWithOptions(viper.KeyDelimiter("::"))
	v.AddConfigPath(configDir)
	v.AutomaticEnv()
	// 获取配置目录下的所有文件
	files, err := os.ReadDir(configDir)
	if err != nil {
		return nil, nil, fmt.Errorf("读取配置文件失败：%w", err)
	}

	// 遍历所有文件并合并配置
	for _, file := range files {
		// 获取文件的完整路径
		filePath := filepath.Join(configDir, file.Name())

		// 获取文件的扩展名
		ext := filepath.Ext(filePath)

		// 只处理 .yaml 或 .yml 文件
		if ext == ".yaml" || ext == ".yml" {
			// 设置配置文件的名称（不包括扩展名）
			baseName := filepath.Base(filePath)
			configName := baseName[0 : len(baseName)-len(ext)]
			v.SetConfigName(configName)

			// 读取并合并配置文件 (会覆盖之前的配置)
			if err := v.MergeInConfig(); err != nil {
				return nil, nil, fmt.Errorf("读取配置文件失败 %s: %w", filePath, err)
			}
		}
	}

	// 反序列化到结构体
	var config Common
	if err := v.Unmarshal(&config); err != nil {
		return nil, nil, fmt.Errorf("反序列化配置失败: %w", err)
	}

	var proto Proto
	if err := v.Unmarshal(&proto); err != nil {
		return nil, nil, fmt.Errorf("反序列化配置失败: %w", err)
	}

	return &config, &proto, nil
}
