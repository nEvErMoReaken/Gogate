package main

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
)

type LogConfig struct {
	LogPath    string `mapstructure:"log_path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
	Level      string `mapstructure:"level"`
}

type ScriptConfig struct {
	ScriptDir string   `mapstructure:"dir"`
	Methods   []string `mapstructure:"methods"`
}

type CommonConfig struct {
	Log       LogConfig       `mapstructure:"log"`
	Script    ScriptConfig    `mapstructure:"script"`
	Connector ConnectorConfig `mapstructure:"connector"`
}

type ConnectorConfig struct {
	Type string `mapstructure:"type"`
}

// Common 为全局配置 (common.yaml)
var Common CommonConfig

// initCommon 用于初始化全局配置
func initCommon(configDir string) error {
	v := viper.NewWithOptions(viper.KeyDelimiter("::")) // 设置 key 分隔符为 ::，因为默认的 . 会和 IP 地址冲突
	v.AddConfigPath(configDir)
	v.AutomaticEnv() // 读取环境变量
	// 获取配置目录下的所有文件
	files, err := os.ReadDir(configDir)
	if err != nil {
		return fmt.Errorf("读取配置文件失败：%w", err)
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
				return fmt.Errorf("读取配置文件失败 %s: %w", filePath, err)
			}
		}
	}

	// 反序列化到结构体
	if err := v.Unmarshal(&Common); err != nil {
		return fmt.Errorf("反序列化配置失败: %w", err)
	}
	return nil
}
