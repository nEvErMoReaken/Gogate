package connector

import (
	"context"
	"fmt"
	"gateway/internal/parser"
	"gateway/internal/pkg"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

// MQTTClient 定义一个接口，包含需要的 MQTT 客户端方法
type MQTTClient interface {
	Connect() mqtt.Token
	Disconnect(quiesce uint)
	SubscribeMultiple(filters map[string]byte, callback mqtt.MessageHandler) mqtt.Token
	IsConnected() bool
}

// MqttConfig 包含 MQTT 配置信息
type MqttConfig struct {
	Broker               string          `mapstructure:"broker"`
	ClientID             string          `mapstructure:"clientID"`
	Username             string          `mapstructure:"username"`
	Password             string          `mapstructure:"password"`
	MaxReconnectInterval time.Duration   `mapstructure:"maxReconnectInterval"`
	Topics               map[string]byte `mapstructure:"topics"` // 主题和 QoS 的 map
}

// MqttConnector Connector的Mqtt版本实现
type MqttConnector struct {
	ctx      context.Context
	config   *MqttConfig
	Client   MQTTClient // MQTT 客户端
	tempChan chan []byte
	parser   *parser.JParser
}

func init() {
	Register("mqtt", NewMqttConnector)
}

func (m *MqttConnector) GetType() string {
	return "message"
}

func (m *MqttConnector) Start(sink *pkg.Parser2DispatcherChan) error {
	logger := pkg.LoggerFromContext(m.ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	// 检查客户端是否已经连接
	if token := m.Client.Connect(); token.Wait() && token.Error() != nil {
		metrics.IncErrorCount()
		metrics.IncMsgErrors("mqtt_connect")
		pkg.ErrChanFromContext(m.ctx) <- fmt.Errorf("MQTT连接失败: %v", token.Error())
	}

	// 订阅多个话题
	token := m.Client.SubscribeMultiple(m.config.Topics, m.messagePubHandler)
	token.Wait() // 等待订阅完成
	if err := token.Error(); err != nil {
		metrics.IncErrorCount()
		metrics.IncMsgErrors("mqtt_subscribe")
		pkg.ErrChanFromContext(m.ctx) <- fmt.Errorf("MQTT订阅失败: %v", err)
	}

	// 持续运行监听消息
	logger.Info("MQTT订阅成功，正在监听消息")

	go m.parser.Start(m.tempChan, *sink)
	return nil
}

func (m *MqttConnector) Close() error {
	logger := pkg.LoggerFromContext(m.ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	// 关闭MQTT客户端，优雅关闭
	if m.Client != nil && m.Client.IsConnected() {
		m.Client.Disconnect(250)
		logger.Info("MQTT连接已断开")
		return nil
	}
	metrics.IncErrorCount()
	return fmt.Errorf("MQTT客户端未连接")
}

func NewMqttConnector(ctx context.Context) (Template, error) {
	// 1. 初始化配置文件
	config := pkg.ConfigFromContext(ctx)
	// 2. 处理 timeout 字段（从字符串解析为 time.Duration）
	if timeoutStr, ok := config.Connector.Para["maxreconnectinterval"].(string); ok {
		duration, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("解析超时配置失败: %s", err)
		}
		fmt.Println("ok")
		config.Connector.Para["maxreconnectinterval"] = duration // 替换为 time.Duration
	}
	var mqttConfig MqttConfig
	err := mapstructure.Decode(config.Connector.Para, &mqttConfig)
	if err != nil {
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}

	parser, err := parser.NewJsonParser(ctx)
	if err != nil {
		return nil, fmt.Errorf("创建JSON解析器失败: %s", err)
	}
	// 2. 创建 MQTT Template 实例
	mqttConnector := &MqttConnector{
		ctx:      ctx,
		config:   &mqttConfig,
		tempChan: make(chan []byte, 100),
		parser:   parser,
	}

	// 3. 创建一个新的 MQTT 客户端
	opts := mqtt.NewClientOptions()
	opts.AddBroker(mqttConfig.Broker)
	opts.SetClientID(mqttConfig.ClientID)
	opts.SetUsername(mqttConfig.Username) // 如果需要用户名和密码
	opts.SetPassword(mqttConfig.Password) // 如果需要用户名和密码

	// 设置自动重连
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(mqttConfig.MaxReconnectInterval) // 设置重连间隔时间

	opts.OnConnect = mqttConnector.connectHandler
	opts.OnConnectionLost = mqttConnector.connectLostHandler

	// 创建 MQTT 客户端
	client := mqtt.NewClient(opts)
	mqttConnector.Client = client
	return mqttConnector, nil
}

func (m *MqttConnector) messagePubHandler(_ mqtt.Client, msg mqtt.Message) {
	logger := pkg.LoggerFromContext(m.ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	// 记录接收到的消息
	metrics.IncMsgReceived("mqtt")

	logger.Info("Received message", zap.String("payload", string(msg.Payload())), zap.String("topic", msg.Topic()))
	select {
	case m.tempChan <- msg.Payload():
	default:
		logger.Warn("MQTT消息处理通道已满，丢弃消息")
	}

	// 记录成功处理的消息
	metrics.IncMsgProcessed("mqtt")

	logger.Debug("消息处理完成",
		zap.String("topic", msg.Topic()))
}

// 连接成功回调
func (m *MqttConnector) connectHandler(client mqtt.Client) {
	logger := pkg.LoggerFromContext(m.ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	// client编译器不允许使用未使用的参数，所以这里使用下划线忽略
	_ = client
	metrics.IncMsgReceived("mqtt_connect")
	logger.Info("成功连接至MQTT broker")
}

// 连接丢失回调
func (m *MqttConnector) connectLostHandler(client mqtt.Client, err error) {
	logger := pkg.LoggerFromContext(m.ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	// client编译器不允许使用未使用的参数，所以这里使用下划线忽略
	_ = client
	metrics.IncErrorCount()
	metrics.IncMsgErrors("mqtt_connection_lost")
	logger.Error("Connect lost", zap.Error(err))
	// 这里Paho会自动重连，不需要手动重连
}
