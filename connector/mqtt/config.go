package mqtt

import (
	"fmt"
	"github.com/spf13/viper"
	"time"
)

// MqttConfig 包含 MQTT 配置信息
type MqttConfig struct {
	Broker               string          `mapstructure:"broker"`
	ClientID             string          `mapstructure:"clientID"`
	Username             string          `mapstructure:"username"`
	Password             string          `mapstructure:"password"`
	MaxReconnectInterval time.Duration   `mapstructure:"maxReconnectInterval"`
	Topics               map[string]byte `mapstructure:"topics"` // 主题和 QoS 的 map
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
