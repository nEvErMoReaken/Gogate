package config

type LogConfig struct {
	LogPath    string `mapstructure:"log_path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
	Level      string `mapstructure:"level"`
}

type TCPServerConfig struct {
	WhiteList bool              `mapstructure:"whiteList"`
	IPAlias   map[string]string `mapstructure:"ipAlias"`
}

type StrategyConfig struct {
	Type   string                 `mapstructure:"type"`    // 策略类型
	Enable bool                   `mapstructure:"enable"`  // 是否启用
	Filter []string               `mapstructure:"filter"`  // 策略过滤条件
	Config map[string]interface{} `mapstructure:",remain"` // 自定义配置项
}

type Common struct {
	ProtoFile string           `mapstructure:"protoFile"`
	CheckCRC  bool             `mapstructure:"check_crc"`
	Log       LogConfig        `mapstructure:"log"`
	TCPServer TCPServerConfig  `mapstructure:"tcpServer"`
	Strategy  []StrategyConfig `mapstructure:"strategy"`
	Script    ScriptConfig     `mapstructure:"script"`
}

type ScriptConfig struct {
	ScriptDir string   `mapstructure:"dir"`
	Methods   []string `mapstructure:"methods"`
}
