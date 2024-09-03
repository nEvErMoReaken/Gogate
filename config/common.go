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

type InfluxDBConfig struct {
	URL      string   `mapstructure:"url"`
	Password string   `mapstructure:"password"`
	Filter   []string `mapstructure:"filter"`
}

type MqttConfig struct {
	// 在此添加需要的配置字段
}

type FinallyConfig struct {
	InfluxDB InfluxDBConfig `mapstructure:"InfluxDB"`
	Mqtt     MqttConfig     `mapstructure:"mqtt"`
}

type Common struct {
	ProtoFile string          `mapstructure:"protoFile"`
	CheckCRC  bool            `mapstructure:"check_crc"`
	Log       LogConfig       `mapstructure:"log"`
	TCPServer TCPServerConfig `mapstructure:"tcpServer"`
	Finally   FinallyConfig   `mapstructure:"finally"`
}
