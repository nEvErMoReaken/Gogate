package common

import (
	"fmt"
	"github.com/spf13/viper"
	"io/fs"
	"path/filepath"
)

type StrategyConfig struct {
	Type   string                 `mapstructure:"type"`    // 策略类型
	Enable bool                   `mapstructure:"enable"`  // 是否启用
	Filter []string               `mapstructure:"filter"`  // 策略过滤条件
	Config map[string]interface{} `mapstructure:",remain"` // 自定义配置项
}

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

type Config struct {
	Log       LogConfig        `mapstructure:"log"`
	Script    ScriptConfig     `mapstructure:"script"`
	Connector ConnectorConfig  `mapstructure:"connector"`
	Strategy  []StrategyConfig `mapstructure:"strategy"`
}

type ConnectorConfig struct {
	Type string `mapstructure:"type"`
}

// InitCommon 用于初始化全局配置
func InitCommon(configDir string) (*Config, *viper.Viper, error) {
	v := viper.NewWithOptions(viper.KeyDelimiter("::")) // 设置 key 分隔符为 ::，因为默认的 . 会和 IP 地址冲突
	v.AddConfigPath(configDir)
	v.AutomaticEnv() // 读取环境变量
	// 遍历配置目录及其子目录中的所有文件
	_ = filepath.WalkDir(configDir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("访问路径 %s 失败: %w", filePath, err)
		}

		// 如果是目录则跳过，继续遍历
		if d.IsDir() {
			return nil
		}

		// 获取文件的扩展名
		ext := filepath.Ext(filePath)

		// 只处理 .yaml 或 .yml 文件
		if ext == ".yaml" || ext == ".yml" {
			// 设置配置文件的名称（不包括扩展名）
			baseName := filepath.Base(filePath)
			configName := baseName[0 : len(baseName)-len(ext)]
			v.SetConfigName(configName)

			// 设置配置文件的路径（不需要再使用 AddConfigPath，因为我们已经有完整路径）
			v.SetConfigFile(filePath)

			// 读取并合并配置文件 (会覆盖之前的配置)
			if err := v.MergeInConfig(); err != nil {
				return fmt.Errorf("读取配置文件失败 %s: %w", filePath, err)
			}
		}

		return nil
	})
	var common Config
	// 反序列化到结构体
	if err := v.Unmarshal(&common); err != nil {
		return nil, nil, fmt.Errorf("反序列化配置失败: %w", err)
	}
	return &common, v, nil
}
