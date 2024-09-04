package config

import "time"

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
	URL       string `mapstructure:"url"`
	ORG       string `mapstructure:"org"`
	Bucket    string `mapstructure:"bucket"`
	Token     string `mapstructure:"token"`
	Tips      Tips   `mapstructure:"tips"`
	BatchSize int    `mapstructure:"batch_size"`
}
type Tips struct {
	Interval time.Duration `mapstructure:"interval"`
	Filter   []string      `mapstructure:"filter"`
}
type MqttConfig struct {
	// 在此添加需要的配置字段
	Broker string `mapstructure:"broker"`
	Topic  string `mapstructure:"topic"`
	Bucket Tips   `mapstructure:"bucket"`
}

type FinallyConfig struct {
	InfluxDB InfluxDBConfig `mapstructure:"influxDB"`
	Mqtt     MqttConfig     `mapstructure:"mqtt"`
}

type Common struct {
	ProtoFile string          `mapstructure:"protoFile"`
	CheckCRC  bool            `mapstructure:"check_crc"`
	Log       LogConfig       `mapstructure:"log"`
	TCPServer TCPServerConfig `mapstructure:"tcpServer"`
	Finally   FinallyConfig   `mapstructure:"finally"`
	Script    ScriptConfig    `mapstructure:"script"`
}

type ScriptConfig struct {
	ScriptDir string   `mapstructure:"dir"`
	Methods   []string `mapstructure:"methods"`
}
