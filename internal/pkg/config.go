package pkg

import (
	"context"
	"fmt"
	"github.com/spf13/viper"
	"io/fs"
	"os"
	"path/filepath"
)

// Config 根Config
type Config struct {
	Parser    ParserConfig           `mapstructure:"parser"`
	Connector ConnectorConfig        `mapstructure:"connector"`
	Strategy  []StrategyConfig       `mapstructure:"strategy"`
	Version   string                 `mapstructure:"version"`
	Log       LogConfig              `mapstructure:"log"`
	Others    map[string]interface{} `mapstructure:",remain"`
}

type LogConfig struct {
	LogPath    string `mapstructure:"log_path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
	Level      string `mapstructure:"level"`
}

// 定义一个不导出的 key 类型，避免 context key 冲突
type configKey struct{}

// WithConfig 将配置指针存入 context 中
func WithConfig(ctx context.Context, config *Config) context.Context {
	return context.WithValue(ctx, configKey{}, config)
}

// ConfigFromContext 从 context 中提取配置指针
func ConfigFromContext(ctx context.Context) *Config {
	if config, ok := ctx.Value(configKey{}).(*Config); ok {
		return config
	}
	return &Config{}
}

type StrategyConfig struct {
	Type   string                 `mapstructure:"type"`   // 策略类型
	Enable bool                   `mapstructure:"enable"` // 是否启用
	Filter []string               `mapstructure:"filter"` // 策略过滤条件
	Para   map[string]interface{} `mapstructure:"config"` // 自定义配置项
}

type ParserConfig struct {
	Type string                 `mapstructure:"type"`
	Para map[string]interface{} `mapstructure:"config"`
}

type ConnectorConfig struct {
	Type string                 `mapstructure:"type"`
	Para map[string]interface{} `mapstructure:"config"`
}

// InitCommon 用于初始化全局配置
func InitCommon(configDir string) (*Config, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("获取当前工作目录失败: %v\n", err)
	} else {
		fmt.Printf("当前工作目录: %s\n", currentDir)
	}
	v := viper.NewWithOptions(viper.KeyDelimiter("::")) // 设置 key 分隔符为 ::，因为默认的 . 会和 IP 地址冲突
	v.AddConfigPath(configDir)
	// 遍历配置目录及其子目录中的所有文件
	err = filepath.WalkDir(configDir, func(filePath string, d fs.DirEntry, err error) error {
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
			if err = v.MergeInConfig(); err != nil {
				return fmt.Errorf("读取配置文件失败 %s: %w", filePath, err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	// 在配置文件读取之后启用环境变量
	v.AutomaticEnv()
	var common Config
	// 反序列化到结构体
	if err = v.Unmarshal(&common); err != nil {
		return nil, fmt.Errorf("反序列化配置失败: %w", err)
	}
	return &common, nil
}
