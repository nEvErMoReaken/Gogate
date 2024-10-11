package pkg

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"log"
	"os"
)

type config struct {
	para struct {
		LogPath    string `mapstructure:"log_path"`
		MaxSize    int    `mapstructure:"max_size"`
		MaxBackups int    `mapstructure:"max_backups"`
		MaxAge     int    `mapstructure:"max_age"`
		Compress   bool   `mapstructure:"compress"`
		Level      string `mapstructure:"level"`
	} `mapstructure:"log"`
}

// NewLogger initializes the common
func NewLogger(v *viper.Viper) *zap.Logger {
	var logConfig config
	if err := v.Unmarshal(&logConfig); err != nil {
		log.Fatalf("Unable to decode into struct, %v", err)
	}
	lumberJackLogger := &lumberjack.Logger{
		Filename:   logConfig.para.LogPath,    // 日志文件路径
		MaxSize:    logConfig.para.MaxSize,    // megabytes
		MaxBackups: logConfig.para.MaxBackups, // number of backups
		MaxAge:     logConfig.para.MaxAge,     // days
		Compress:   logConfig.para.Compress,   // compress old logs
		LocalTime:  true,
	}

	// 创建编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "log",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "trace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder, // 带颜色
		EncodeTime:     zapcore.ISO8601TimeEncoder,       // ISO8601时间格式
		EncodeDuration: zapcore.SecondsDurationEncoder,   // 时间格式
		EncodeCaller:   zapcore.ShortCallerEncoder,       // 简短的调用者编码器 (文件名和行号)
	}

	// 创建一个控制台编码器，带有自定义的日志格式
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// 通过level参数创建zapcore
	// 解析日志级别
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(logConfig.para.Level)); err != nil {
		level = zap.InfoLevel // 默认日志级别为 InfoLevel
	}
	// 创建一个核心，它将所有日志写入 combinedSyncer
	core := zapcore.NewCore(
		encoder,
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(lumberJackLogger)),
		level,
	)
	// 创建 Logger 并添加调用者信息和堆栈跟踪
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	return logger
}
