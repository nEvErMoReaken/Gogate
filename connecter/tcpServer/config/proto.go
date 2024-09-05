package config

type FromConfig struct {
	Byte int `mapstructure:"byte" yaml:"byte"`
}

type DecodingConfig struct {
	Method string `mapstructure:"method,omitempty" yaml:"method,omitempty"`
}

type ToConfig struct {
	Cached bool     `mapstructure:"cached,omitempty" yaml:"cached,omitempty"`
	Stable bool     `mapstructure:"stable,omitempty" yaml:"stable,omitempty"`
	Device string   `mapstructure:"device,omitempty" yaml:"device,omitempty"`
	Type   string   `mapstructure:"type,omitempty" yaml:"type,omitempty"`
	Fields []string `mapstructure:"fields,omitempty" yaml:"fields,omitempty"`
}

type Section struct {
	From     FromConfig     `mapstructure:"from" yaml:"from"`
	Decoding DecodingConfig `mapstructure:"decoding,omitempty" yaml:"decoding,omitempty"`
	For      interface{}    `mapstructure:"for,omitempty" yaml:"for,omitempty"`
	To       ToConfig       `mapstructure:"to,omitempty" yaml:"to,omitempty"`
	Desc     string         `mapstructure:"desc,omitempty" yaml:"desc,omitempty"`
}

type ParsingConfig struct {
	Section []Section `mapstructure:"section"`
	Len     int       `mapstructure:"len,omitempty" yaml:"len,omitempty"`
}

type PreParsingConfig struct {
	Section []Section `mapstructure:"section"`
}

// 预解析和正式解析

type Proto struct {
	PreParsing PreParsingConfig `mapstructure:"preParsing"`
	Parsing    ParsingConfig    `mapstructure:"parsing" yaml:"parsing"`
}
