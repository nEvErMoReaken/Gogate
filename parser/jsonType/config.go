package jsonType

import (
	"fmt"
	"github.com/spf13/viper"
)

// ConversionRule 描述从 JSON 到 DeviceSnapshot 的字段映射规则
type ConversionRule struct {
	Path   string `yaml:"path"`   // JSON 中的字段路径
	Type   string `yaml:"type"`   // 目标数据类型 (例如 "datetime")
	Format string `yaml:"format"` // 用于时间字段的格式（可选）
}

// ConversionConfig 描述所有字段的转换规则
type ConversionConfig struct {
	DeviceName ConversionRule `yaml:"device_name"`
	DeviceType ConversionRule `yaml:"device_type"`
	Fields     ConversionRule `yaml:"fields"`
	Ts         ConversionRule `yaml:"ts"`
}

func UnmarshalJsonParseConfig(v *viper.Viper) (*ConversionConfig, error) {
	// 反序列化到结构体
	var conversionConfig ConversionConfig
	if err := v.Unmarshal(&conversionConfig); err != nil {
		return nil, fmt.Errorf("反序列化配置失败: %w", err)
	}

	return &conversionConfig, nil
}
