package pkg

import (
	"context"
	"fmt"
	"os"
	"time"

	law "github.com/shengyanli1982/law"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
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

// LogErrorCallback 实现 law.Callback 接口，用于处理日志写入错误
type LogErrorCallback struct {
	ErrorLogger *zap.Logger
}

// OnWriteFailed 当日志写入出错时回调
// content: 写入失败的内容
// reason: 失败原因
func (c *LogErrorCallback) OnWriteFailed(content []byte, reason error) {
	if c.ErrorLogger != nil {
		c.ErrorLogger.Error("日志写入失败",
			zap.Error(reason),
			zap.ByteString("content", content))
	} else {
		fmt.Fprintf(os.Stderr, "日志写入失败: %v, 数据: %s\n", reason, string(content))
	}
}

// 存储异步写入器的全局变量，便于程序退出时清理资源
var asyncWriters []*law.WriteAsyncer

// 存储错误回调
var errorCallback *LogErrorCallback

// 初始化错误回调
func initErrorCallback(logger *zap.Logger) *LogErrorCallback {
	if errorCallback == nil {
		errorCallback = &LogErrorCallback{
			ErrorLogger: logger,
		}
	}
	return errorCallback
}

// 关闭所有异步写入器
func CloseAllAsyncWriters() {
	for _, aw := range asyncWriters {
		aw.Stop()
	}
	asyncWriters = nil
}

// 默认配置值
const (
	DefaultBufferSize        = 2048 // 默认缓冲区大小：2KB
	DefaultFlushIntervalSecs = 5    // 默认刷新间隔：5秒
)

// NewLogger initializes the common
func NewLogger(config *LogConfig) *zap.Logger {
	// 创建一个初始的同步logger，用于记录异步写入过程中的错误
	initialEncoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "log",
		MessageKey:     "msg",
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     CustomTimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
	}
	initialEncoder := zapcore.NewJSONEncoder(initialEncoderConfig)
	initialCore := zapcore.NewCore(
		initialEncoder,
		zapcore.AddSync(os.Stderr),
		zap.ErrorLevel,
	)
	initialLogger := zap.New(initialCore)

	// 初始化错误回调
	callback := initErrorCallback(initialLogger)

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

	// 设置缓冲区大小和刷新间隔
	bufferSize := DefaultBufferSize
	if config.BufferSize > 0 {
		bufferSize = config.BufferSize
	}

	flushInterval := DefaultFlushIntervalSecs
	if config.FlushIntervalSecs > 0 {
		flushInterval = config.FlushIntervalSecs
	}

	// 配置异步日志
	lawConfig := law.NewConfig().
		WithCallback(callback).
		WithBufferSize(bufferSize)

	// 创建异步写入器
	stdoutAsyncer := law.NewWriteAsyncer(os.Stdout, lawConfig)
	fileAsyncer := law.NewWriteAsyncer(lumberJackLogger, lawConfig)

	// 记录配置信息
	initialLogger.Info("初始化异步日志配置",
		zap.Int("缓冲区大小", bufferSize),
		zap.Int("刷新间隔(秒)", flushInterval))

	// 保存异步写入器以便后续清理
	asyncWriters = append(asyncWriters, stdoutAsyncer, fileAsyncer)

	core := zapcore.NewCore(
		encoder,
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(stdoutAsyncer), zapcore.AddSync(fileAsyncer)),
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
