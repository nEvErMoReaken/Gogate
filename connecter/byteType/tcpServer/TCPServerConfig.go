package tcpServer

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"time"
)

type TCPServerConfig struct {
	WhiteList bool              `mapstructure:"whiteList"`
	IPAlias   map[string]string `mapstructure:"ipAlias"`
	Port      string            `mapstructure:"port"`
	Timeout   time.Duration     `mapstructure:"timeout"`
}

type StrategyConfig struct {
	Type   string                 `mapstructure:"type"`    // 策略类型
	Enable bool                   `mapstructure:"enable"`  // 是否启用
	Filter []string               `mapstructure:"filter"`  // 策略过滤条件
	Config map[string]interface{} `mapstructure:",remain"` // 自定义配置项
}

type TcpServer struct {
	ProtoFile string           `mapstructure:"protoFile"`
	CheckCRC  bool             `mapstructure:"check_crc"`
	TCPServer TCPServerConfig  `mapstructure:"tcpServer"`
	Strategy  []StrategyConfig `mapstructure:"strategy"`
}

type Setting struct {
	Type   string `mapstructure:"type"`
	Length int    `mapstructure:"length"`
	End    []byte `mapstructure:"end"`
}

func NewConfig(configDir string) (*TcpServer, error) {
	v := viper.NewWithOptions(viper.KeyDelimiter("::")) // 设置 key 分隔符为 ::，因为默认的 . 会和 IP 地址冲突
	v.AddConfigPath(configDir)
	v.AutomaticEnv()
	// 获取配置目录下的所有文件
	files, err := os.ReadDir(configDir)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败：%w", err)
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
				return nil, fmt.Errorf("读取配置文件失败 %s: %w", filePath, err)
			}
		}
	}

	// 反序列化到结构体
	var config TcpServer
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("反序列化配置失败: %w", err)
	}

	return &config, nil
}
