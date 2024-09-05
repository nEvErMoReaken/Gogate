package config

import "time"

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
