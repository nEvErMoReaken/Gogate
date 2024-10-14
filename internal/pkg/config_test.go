package pkg

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestInitCommon 测试 InitCommon 函数
func TestInitCommon(t *testing.T) {
	// 创建一个临时的配置文件目录
	tempDir := t.TempDir()

	// 在临时目录中创建一个测试用的配置文件
	configFilePath := filepath.Join(tempDir, "test_config.yaml")
	configContent := `
version: "1.0.0"
my_custom_config: "custom_value"
log:
  log_path: ./logs
  # MaxSize：在进行切割之前，日志文件的最大大小（以MB为单位）
  max_size: 512
  # MaxBackups：保留旧文件的最大个数
  max_backups: 1000
  # MaxAge：保留旧文件的最大天数
  max_age: 365
  # Compress：是否压缩/归档旧文件
  compress: true
  level: debug
# 连接器相关配置
connector:
  type: mqtt  # mqtt|tcpserver
  config:
    broker: "tcp://broker.hivemq.com:1883"
    topics:
      test/topic1: 0  # 主题1, QoS = 0
      test/topic2: 1  # 主题2, QoS = 1
      test/topic3: 2  # 主题3, QoS = 2
      #    QoS 级别
      #    0：最多一次交付（At most once）。消息可能丢失，不会有重试。
      #    1：至少一次交付（At least once）。消息会至少发送一次，可能会有重复消息。
      #    2：仅一次交付（Exactly once）。消息保证只会到达一次，最安全的 QoS。
      #    发布端和订阅端的 QoS 可以是不同的。消息的实际传递 QoS 取决于 发布者设置的 QoS 和 订阅者设置的 QoS 中的 最小值。
    clientID: "gateway"
    username:
    password:
    maxReconnectInterval: 10s

# 解析器相关配置
parser:
  type: ioReader # ioReader|json
  config:
    dir: ./script
    protoFile: test
#    method: "ConvertOldGatewayTelemetry"


# 后处理策略相关配置 可以有多个
strategy:
  - type: influxdb
    enable: true
    filter: # 格式<设备类型>:<设备名称>:<遥测名称>
       - ".*:.*:.*"
#       - "vobc\\.info:vobc.*:RIOM_sta_3"
    config:
  #    以下是自定义配置项
      url: http://10.17.191.107:8086
      token: mK_0NkLVPW8THIYkn52eqr7enL6IinGp8d5xbXizO1mVxAEk_EuOFxZ9OKWYcwVgi2XmogD6iPcO9KQ8ToVvtQ==
      org: "byd"
      bucket: "test"
      batch_size: 2000
      tags:
        - "data_source"
  - type: iotdb
    enable: false
    config:
      url: "127.0.0.1:6667,127.0.0.1:6668,127.0.0.1:6669"
      username:
      password:
      batch_size:
  - type: mqtt
    enable: false
    filter: # 格式<设备类型>:<设备名称>:<遥测名称>
      - ".*:.*:.*"
    config:
      url: tcp://
      clientID:
      username:
      password:
      willTopic: "status/gateway"

`
	err := os.WriteFile(configFilePath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("创建配置文件失败: %v", err)
	}

	// 调用 InitCommon 进行初始化
	config, err := InitCommon(tempDir)
	if err != nil {
		t.Fatalf("InitCommon 函数调用失败: %v", err)
	}

	// 验证配置项是否正确解析
	if config.Parser.Type != "ioReader" {
		t.Errorf("期望解析器类型为 'ioReader'，但得到的是 %s", config.Parser.Type)
	}
	if config.Connector.Type != "mqtt" {
		t.Errorf("期望连接器类型为 'mqtt'，但得到的是 %s", config.Connector.Type)
	}
	if config.Log.LogPath != "./logs" {
		t.Errorf("期望日志路径为 './logs'，但得到的是 %s", config.Log.LogPath)
	}
	if config.Log.MaxSize != 512 {
		t.Errorf("期望日志文件大小为 512 MB，但得到的是 %d", config.Log.MaxSize)
	}
	if config.Log.Level != "debug" {
		t.Errorf("期望日志级别为 'debug'，但得到的是 %s", config.Log.Level)
	}
	if config.Others["my_custom_config"] != "custom_value" {
		t.Errorf("期望自定义配置项 my_custom_config 为 'custom_value'，但得到的是 %s", config.Others["my_custom_config"])
	}
}

// TestWithConfigAndConfigFromContext 测试 WithConfig 和 ConfigFromContext 函数
func TestWithConfigAndConfigFromContext(t *testing.T) {
	// 创建一个测试配置
	testConfig := &Config{
		Version: "1.0.0",
		Parser: ParserConfig{
			Type: "json",
		},
	}

	// 创建一个基础上下文
	ctx := context.Background()

	// 将配置存入上下文
	ctxWithConfig := WithConfig(ctx, testConfig)

	// 从上下文中提取配置
	extractedConfig := ConfigFromContext(ctxWithConfig)

	// 检查提取出来的配置是否与原始配置相同
	if extractedConfig.Version != "1.0.0" {
		t.Errorf("期望提取到的配置版本为 '1.0.0'，但得到的是 %s", extractedConfig.Version)
	}

	if extractedConfig.Parser.Type != "json" {
		t.Errorf("期望解析器类型为 'json'，但得到的是 %s", extractedConfig.Parser.Type)
	}
}

// TestInitCommonConfigFileNotFound 测试 InitCommon 函数当配置文件不存在时的错误处理
func TestInitCommonConfigFileNotFound(t *testing.T) {
	// 使用一个不存在的目录调用 InitCommon
	_, err := InitCommon("/invalid/path")
	if err == nil {
		t.Fatal("期望出现错误，但未得到错误")
	}

	// 只检查错误信息的前缀
	expectedErrPrefix := "访问路径 /invalid/path"
	if err.Error()[:len(expectedErrPrefix)] != expectedErrPrefix {
		t.Errorf("期望错误信息以 '%s' 开头，但得到的是 '%s'", expectedErrPrefix, err.Error())
	}
}
func TestUnmarshalConfig(t *testing.T) {
	// 创建一个临时的配置文件目录
	tempDir := t.TempDir()

	// 在临时目录中创建一个与config不匹配的配置文件
	configFilePath := filepath.Join(tempDir, "invalid_config.yaml")
	invalidConfigContent := `
parser:
  type:
    -"json"
  config:
    key:
      - "json"
connector:
  type: "mqtt"
  config:
    broker: "tcp://localhost:1883"
    clientID: "test_client"
log:
  log_path: "/var/log/test.log"
  max_size: 10
  max_backups: 3
  max_age: "not_a_number"
  compress: true
  level: "info"
my_custom_config: "custom_value"
` // 无效的数据类型
	err := os.WriteFile(configFilePath, []byte(invalidConfigContent), 0644)
	if err != nil {
		t.Fatalf("创建配置文件失败: %v", err)
	}

	// 调用 InitCommon 进行初始化，预期会出错
	_, err = InitCommon(tempDir)
	if err == nil {
		t.Fatal("期望出现错误，但未得到错误")
	}

	expectedErr := "反序列化配置失败"
	if err.Error()[:len(expectedErr)] != expectedErr {
		t.Errorf("期望错误信息为 '%s'，但得到的是 '%s'", expectedErr, err.Error())
	}
}

// TestInitCommonInvalidConfigFormat 测试 InitCommon 函数当配置文件格式错误时的处理
func TestInitCommonInvalidConfigFormat(t *testing.T) {
	// 创建一个临时的配置文件目录
	tempDir := t.TempDir()

	// 在临时目录中创建一个格式错误的配置文件
	configFilePath := filepath.Join(tempDir, "invalid_config.yaml")
	invalidConfigContent := `
parser 
  type: "json"
  config:
    key: "value"
connector:
  type: "mqtt"
  config:
    broker: "tcp://localhost:1883"
    clientID: "test_client"
` // 缺少冒号
	err := os.WriteFile(configFilePath, []byte(invalidConfigContent), 0644)
	if err != nil {
		t.Fatalf("创建配置文件失败: %v", err)
	}

	// 调用 InitCommon 进行初始化，预期会出错
	_, err = InitCommon(tempDir)
	if err == nil {
		t.Fatal("期望出现错误，但未得到错误")
	}

	expectedErr := "读取配置文件失败"
	if err.Error()[:len(expectedErr)] != expectedErr {
		t.Errorf("期望错误信息为 '%s'，但得到的是 '%s'", expectedErr, err.Error())
	}
}

// TestConfigFromContextWithoutConfig 测试在上下文中没有配置时的情况
func TestConfigFromContextWithoutConfig(t *testing.T) {
	// 创建一个不包含配置的上下文
	ctx := context.Background()

	// 从上下文中提取配置
	extractedConfig := ConfigFromContext(ctx)

	// 检查提取到的配置是否为默认值（空配置）
	if extractedConfig.Version != "" {
		t.Errorf("期望提取到的版本为空字符串，但得到的是 %s", extractedConfig.Version)
	}
	if extractedConfig.Parser.Type != "" {
		t.Errorf("期望解析器类型为空字符串，但得到的是 %s", extractedConfig.Parser.Type)
	}
}
