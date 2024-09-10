package tcpServer

import (
	"fmt"
	"github.com/spf13/viper"
	"time"
)

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

func UnmarshalTCPConfig(v *viper.Viper) (*TcpServer, error) {
	// 反序列化到结构体
	var tcpServer TcpServer
	if err := v.Unmarshal(&tcpServer); err != nil {
		return nil, fmt.Errorf("反序列化配置失败: %w", err)
	}

	return &tcpServer, nil
}
