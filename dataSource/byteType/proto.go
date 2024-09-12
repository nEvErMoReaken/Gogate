package byteType

type FixedChunkConfig struct {
	Type     string          `mapstructure:"type"`
	Length   interface{}     `mapstructure:"length"`
	Sections []SectionConfig `mapstructure:"sections"`
}

// SectionConfig 定义
type SectionConfig struct {
	From     FromConfig `mapstructure:"from"` // 偏移量
	Decoding Decoding   `mapstructure:"decoding"`
	For      ForConfig  `mapstructure:"for"`  // 赋值变量
	To       ToConfig   `mapstructure:"to"`   // 字段转换配置
	Desc     string     `mapstructure:"desc"` // 字段说明
}

type ForConfig struct {
	VarName []string `mapstructure:"varName"`
}
type FromConfig struct {
	Byte   int         `mapstructure:"byte"`
	Repeat interface{} `mapstructure:"repeat"`
}

type Decoding struct {
	Method string `mapstructure:"method"`
}

type ToConfig struct {
	Cached     bool     `mapstructure:"cached"`
	Stable     bool     `mapstructure:"stable"`
	DeviceName string   `mapstructure:"device"`
	DeviceType string   `mapstructure:"type"`
	Fields     []string `mapstructure:"fields"`
}
