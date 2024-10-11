package pkg

import (
	"context"
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

// 定义一个不导出的 key 类型，避免 context key 冲突
type loggerKey struct{}

// WithLogger 将带有模块信息的 zap.Logger 存入 context 中
func WithLogger(ctx context.Context, logger *zap.Logger, module string) context.Context {
	loggerWithModule := logger.With(zap.String("module", module))
	return context.WithValue(ctx, loggerKey{}, loggerWithModule)
}

// LoggerFromContext 从 context 中提取 zap.Logger
func LoggerFromContext(ctx context.Context) *zap.Logger {
	// 尝试从 context 中获取 logger，如果没有则返回一个默认的 logger
	if logger, ok := ctx.Value(loggerKey{}).(*zap.Logger); ok {
		return logger
	}
	return zap.NewNop() // 返回一个 no-op logger，避免 nil pointer 错误
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
