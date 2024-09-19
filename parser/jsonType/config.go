package jsonType

import (
	"fmt"
	"github.com/spf13/viper"
)

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
