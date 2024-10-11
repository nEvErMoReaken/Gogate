package pkg

import (
	"fmt"
	"github.com/spf13/viper"
	"io/fs"
	"path/filepath"
	"time"
)

type StrategyConfig struct {
	Type   string                 `mapstructure:"type"`    // 策略类型
	Enable bool                   `mapstructure:"enable"`  // 是否启用
	Filter []string               `mapstructure:"filter"`  // 策略过滤条件
	Config map[string]interface{} `mapstructure:",remain"` // 自定义配置项
}

type ScriptConfig struct {
	ScriptDir string `mapstructure:"dir"`
}

type Config struct {
	Script    ScriptConfig     `mapstructure:"script"`
	Connector ConnectorConfig  `mapstructure:"connector"`
	Strategy  []StrategyConfig `mapstructure:"strategy"`
	Version   string           `mapstructure:"version"`
}

type ConnectorConfig struct {
	Type string `mapstructure:"type"`
}

// MqttConfig 包含 MQTT 配置信息
type MqttConfig struct {
	Broker               string          `mapstructure:"broker"`
	ClientID             string          `mapstructure:"clientID"`
	Username             string          `mapstructure:"username"`
	Password             string          `mapstructure:"password"`
	MaxReconnectInterval time.Duration   `mapstructure:"maxReconnectInterval"`
	Topics               map[string]byte `mapstructure:"topics"` // 主题和 QoS 的 map
}

type ServerConfig struct {
	WhiteList bool              `mapstructure:"whiteList"`
	IPAlias   map[string]string `mapstructure:"ipAlias"`
	Port      string            `mapstructure:"port"`
	Timeout   time.Duration     `mapstructure:"timeout"`
}

type TcpServer struct {
	ProtoFile string       `mapstructure:"protoFile"`
	CheckCRC  bool         `mapstructure:"check_crc"`
	TCPServer ServerConfig `mapstructure:"tcpServer"`
}

type Setting struct {
	Type   string `mapstructure:"type"`
	Length int    `mapstructure:"length"`
	End    []byte `mapstructure:"end"`
}

type JsonParseConfig struct {
	rules JsonConfig `mapstructure:"jsonParseConfig"`
}
type JsonConfig struct {
	Method string `mapstructure:"method"`
}

func UnmarshalJsonParseConfig(v *viper.Viper) (*JsonParseConfig, error) {
	// 反序列化到结构体
	var conversionConfig JsonParseConfig
	if err := v.Unmarshal(&conversionConfig); err != nil {
		return nil, fmt.Errorf("反序列化配置失败: %w", err)
	}

	return &conversionConfig, nil
}

func UnmarshalTCPConfig(v *viper.Viper) (*TcpServer, error) {
	// 反序列化到结构体
	var tcpServer TcpServer
	if err := v.Unmarshal(&tcpServer); err != nil {
		return nil, fmt.Errorf("反序列化配置失败: %w", err)
	}

	return &tcpServer, nil
}

// UnmarshalMqttConfig 是解析配置文件的通用方法
func UnmarshalMqttConfig(v *viper.Viper) (*MqttConfig, error) {
	// 反序列化到结构体
	var config MqttConfig
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("反序列化配置失败: %w", err)
	}
	return &config, nil
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
