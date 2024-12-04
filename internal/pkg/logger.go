package pkg

import (
	"context"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"time"
)

// 定义一个不导出的 key 类型，避免 context key 冲突
type loggerKey struct{}

// WithLogger 将带有模块信息的 zap.Logger 存入 context 中
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// WithLoggerAndModule 将带有模块信息的 zap.Logger 存入 context 中
func WithLoggerAndModule(ctx context.Context, logger *zap.Logger, module string) context.Context {
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

func CustomTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	// 格式化为没有时区信息的时间字符串
	enc.AppendString(t.Format("2006-01-02T15:04:05.000"))
}

// NewLogger initializes the common
func NewLogger(config *LogConfig) *zap.Logger {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   config.LogPath,    // 日志文件路径
		MaxSize:    config.MaxSize,    // megabytes
		MaxBackups: config.MaxBackups, // number of backups
		MaxAge:     config.MaxAge,     // days
		Compress:   config.Compress,   // compress old logs
		LocalTime:  true,
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "log",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "trace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     CustomTimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	encoder := zapcore.NewJSONEncoder(encoderConfig)

	var level zapcore.Level
	if err := level.UnmarshalText([]byte(config.Level)); err != nil {
		level = zap.InfoLevel
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(lumberJackLogger)),
		level,
	)

	// 根据日志级别添加 `zap.AddCaller` 或不添加
	var options []zap.Option
	options = append(options, zap.AddStacktrace(zapcore.ErrorLevel)) // 堆栈跟踪从 ErrorLevel 开始
	if level < zap.InfoLevel {                                       // Info 或更低级别不显示 Caller
		options = append(options, zap.AddCaller())
	}

	logger := zap.New(core, options...)
	return logger
}
