package common

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
)

var Log *zap.SugaredLogger

// InitLogger initializes the common
func InitLogger(filename string, maxSize, maxBackups, maxAge int, compress bool) {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxSize,    // megabytes
		MaxBackups: maxBackups, // number of backups
		MaxAge:     maxAge,     // days
		Compress:   compress,   // compress old logs
		LocalTime:  true,
	}

	// 创建两个 WriteSyncer，一个用于文件输出，一个用于控制台输出
	fileSyncer := zapcore.AddSync(lumberJackLogger)
	consoleSyncer := zapcore.AddSync(os.Stdout)

	// 将文件和控制台输出合并为一个 MultiWriteSyncer
	combinedSyncer := zapcore.NewMultiWriteSyncer(fileSyncer, consoleSyncer)

	// 创建编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "common",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,   // 日志级别的大写编码器
		EncodeTime:     zapcore.ISO8601TimeEncoder,    // ISO8601时间格式
		EncodeDuration: zapcore.StringDurationEncoder, // 持续时间字符串编码器
		EncodeCaller:   zapcore.ShortCallerEncoder,    // 简短的调用者编码器 (文件名和行号)
	}

	// 创建一个控制台编码器，带有自定义的日志格式
	encoder := zapcore.NewConsoleEncoder(encoderConfig)

	// 创建一个核心，它将所有日志写入 combinedSyncer
	core := zapcore.NewCore(encoder, combinedSyncer, zap.InfoLevel)

	// 创建 Logger 并添加调用者信息和堆栈跟踪
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	Log = logger.Sugar()
}
