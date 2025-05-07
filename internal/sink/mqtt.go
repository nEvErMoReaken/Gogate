package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/internal/pkg"
	"strings"
	"time"

	"go.uber.org/zap"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/mitchellh/mapstructure"
)

// 初始化函数，注册 IoTDB 策略
func init() {
	// 注册发送策略
	Register("mqtt", NewMqttStrategy)
}

// MQTTClientInterface 定义了我们需要的 MQTT 客户端方法
type MQTTClientInterface interface {
	Connect() MQTT.Token
	Disconnect(quiesce uint)
	Publish(topic string, qos byte, retained bool, payload interface{}) MQTT.Token
	// 根据需要添加其他方法
}

// MqttStrategy 实现将数据发布到 MQTT 的逻辑
type MqttStrategy struct {
	client MQTTClientInterface
	info   MQTTInfo
	ctx    context.Context
	logger *zap.Logger
}

// maxReconnectAttempts 最大重连次数
const maxReconnectAttempts = 5

// MQTTInfo MQTT的专属配置
type MQTTInfo struct {
	URL       string `mapstructure:"url"`
	ClientID  string `mapstructure:"clientID"`
	UserName  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
	WillTopic string `mapstructure:"willTopic"`
}

// NewMqttStrategy Step.0 构造函数
func NewMqttStrategy(ctx context.Context) (Template, error) {
	log := pkg.LoggerFromContext(ctx)
	config := pkg.ConfigFromContext(ctx)
	var info MQTTInfo

	for _, strategyConfig := range config.Strategy {
		if strategyConfig.Enable && strategyConfig.Type == "mqtt" {
			// 将 map 转换为结构体
			if err := mapstructure.Decode(strategyConfig.Para, &info); err != nil {
				log.Error("Error decoding map to struct", zap.Error(err))
				return nil, fmt.Errorf("[NewMqttStrategy] Error decoding map to struct: %v", err)
			}
		}
	}

	// 定义 MQTT 客户端的选项
	opts := MQTT.NewClientOptions().AddBroker(info.URL)
	opts.SetClientID(info.ClientID) // 设置客户端 ID
	opts.SetUsername(info.UserName) // 可选：如果 Broker 需要认证
	opts.SetPassword(info.Password) // 可选：如果 Broker 需要认证
	opts.SetWill(info.WillTopic, "offline", 1, true)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(5 * time.Minute)
	opts.SetConnectRetryInterval(10 * time.Second)
	// 添加连接丢失的回调函数，打印连接丢失日志
	opts.SetConnectionLostHandler(func(client MQTT.Client, err error) {
		log.Error("MQTT connection lost", zap.Error(err))
	})

	reconnectAttempts := 0 // 重连尝试计数器

	// 重连中的回调函数
	opts.SetReconnectingHandler(func(client MQTT.Client, opts *MQTT.ClientOptions) {
		reconnectAttempts++
		log.Info("Attempting to reconnect...", zap.Int("attempt", reconnectAttempts))

		// 如果超过最大重连次数
		if reconnectAttempts >= maxReconnectAttempts {
			// 断开连接
			client.Disconnect(250)
			// 将错误传递到错误通道，通知上层程序
			errChan := pkg.ErrChanFromContext(ctx)
			if errChan != nil {
				log.Error("MQTT reached max reconnect attempts, stopping client")
				errChan <- fmt.Errorf("[MqttStrategy]MQTT reached max reconnect attempts, stopping client")
			}
		}
	})

	// 成功连接时重置计数器
	opts.SetOnConnectHandler(func(client MQTT.Client) {
		reconnectAttempts = 0
		log.Info("Successfully connected to MQTT Broker.")
	})

	// 创建 MQTT 客户端
	mqCLi := MQTT.NewClient(opts)
	// 连接到 MQTT Broker
	if token := mqCLi.Connect(); token.Wait() && token.Error() != nil {
		log.Error("Failed to connect to MQTT Broker", zap.Error(token.Error()))
		return nil, fmt.Errorf("连接到 MQTT Broker 失败:%+v", token.Error())
	}

	return &MqttStrategy{
		client: mqCLi,
		info:   info,
		ctx:    ctx,
		logger: log,
	}, nil
}

// GetType Step.1
func (m *MqttStrategy) GetType() string {
	return "mqtt"
}

// Start Step.2
func (m *MqttStrategy) Start(pointChan chan *pkg.PointPackage) {
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	defer m.client.Disconnect(250)
	m.logger.Info("===MqttStrategy started===")
	// 发布网关上线的状态
	m.client.Publish(m.info.WillTopic, 1, true, "online")
OuterLoop:
	for {
		select {
		case <-m.ctx.Done():
			m.Stop()
			m.logger.Info("===MqttStrategy stopped===")
			break OuterLoop
		case pointPackage := <-pointChan:
			// 创建发布计时器
			publishTimer := metrics.NewTimer("mqtt_strategy_publish")

			// 记录接收的点
			metrics.IncMsgReceived("mqtt_strategy")

			err := m.Publish(pointPackage)

			if err != nil {
				metrics.IncErrorCount()
				metrics.IncMsgErrors("mqtt_strategy")
				m.logger.Error("MqttStrategy error occurred", zap.Error(err))
			} else {
				// 记录成功处理的点
				metrics.IncMsgProcessed("mqtt_strategy")
			}

			publishTimer.StopAndLog(m.logger)
		}
	}
}

func (m *MqttStrategy) Publish(pointPackage *pkg.PointPackage) error {
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	// 创建JSON转换计时器
	jsonTimer := metrics.NewTimer("mqtt_strategy_json")

	// 创建一个新的 map[string]interface{} 来存储解引用的字段
	decodedFields := make(map[string]interface{})
	for _, point := range pointPackage.Points {
		for key, valuePtr := range point.Field {
			if valuePtr == nil {
				continue // 跳过 nil 值
			}

			decodedFields[key] = valuePtr
		}
		decodedFields["ts"] = pointPackage.Ts

		// 将 map 序列化为 JSON
		jsonData, err := json.Marshal(decodedFields)

		jsonTimer.Stop()

		if err != nil {
			metrics.IncErrorCount()
			metrics.IncMsgErrors("mqtt_strategy_json")
			return fmt.Errorf("序列化 JSON 失败: %+v", err)
		}

		// 创建发送计时器
		sendTimer := metrics.NewTimer("mqtt_strategy_send")

		// 分割device.vobc.123.1.1这样的设备名称，填入mqtt话题格式中
		split := strings.Split(point.Device, ".")
		topic := "gateway"
		for i := 0; i < len(split); i++ {
			topic += "/"
			topic += split[i]
		}
		topic += "/fields"
		// final topic like that: /gateway/device/vobc/123/1/1
		token := m.client.Publish(topic, 0, true, jsonData)

		// 等待发布完成
		if token.Wait() && token.Error() != nil {
			metrics.IncErrorCount()
			metrics.IncMsgErrors("mqtt_strategy_publish")
			sendTimer.Stop()
			return fmt.Errorf("MQTT发布失败: %v", token.Error())
		}

		sendDuration := sendTimer.Stop()

		m.logger.Debug("[MqttStrategy]发布消息",
			zap.String("topic", topic),
			zap.String("data", string(jsonData)),
			zap.Duration("duration", sendDuration))
	}
	return nil
}

// Stop 停止 MQTTStrategy
func (m *MqttStrategy) Stop() {
	m.client.Publish(m.info.WillTopic, 1, true, "offline")
	m.client.Disconnect(250)
}
