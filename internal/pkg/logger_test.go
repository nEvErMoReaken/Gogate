package pkg

import (
	"bytes"
	"github.com/spf13/viper"
	"testing"
)

func TestNewLogger(t *testing.T) {
	// 设置测试配置
	v := viper.New()
	v.SetConfigType("yaml")
	err := v.ReadConfig(bytes.NewBufferString(`
	log:
	  log_path: "test.log"
	  max_size: 10
	  max_backups: 5
	  max_age: 30
	  compress: true
	  level: "info"
	`))
	if err != nil {
		return
	}

	// 初始化 logger
	logger := NewLogger(v)

	// 测试输出
	logger.Info("This is a test log message")

	// 这里可以进一步验证日志是否按照预期格式输出
	// 例如，可以使用 io.Writer 来捕获日志输出并进行验证
}

func TestLogLevel(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.ReadConfig(bytes.NewBufferString(`
log:
  log_path: "test.log"
  max_size: 10
  max_backups: 5
  max_age: 30
  compress: true
  level: "debug"
`))

	logger := NewLogger(v)

	// 测试日志级别
	logger.Debug("This debug message should be logged")
	logger.Info("This info message should be logged")
	logger.Warn("This warn message should be logged")
	logger.Error("This error message should be logged")

	// 在这里可以验证日志输出内容（通常需要借助 mock 或类似的工具）
}

func TestInvalidLogLevel(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")
	err := v.ReadConfig(bytes.NewBufferString(`
log:
  log_path: "test.log"
  max_size: 10
  max_backups: 5
  max_age: 30
  compress: true
  level: "invalid_level"
`))
	if err != nil {
		return
	}

	logger := NewLogger(v)

	// 测试默认日志级别
	logger.Info("This message should be logged at default level")
}
