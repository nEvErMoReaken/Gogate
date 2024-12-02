package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/internal/pkg"
	"go.uber.org/zap"
	"time"

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

// GetCore Step.1
func (m *MqttStrategy) GetType() string {
	return "mqtt"
}

// Start Step.2
func (m *MqttStrategy) Start(pointChan chan pkg.Point) {
	defer m.client.Disconnect(250)
	m.logger.Info("===MqttStrategy started===")
	// 发布网关上线的状态
	m.client.Publish(m.info.WillTopic, 1, true, "online")
	for {
		select {
		case <-m.ctx.Done():
			m.Stop()
			pkg.LoggerFromContext(m.ctx).Info("===MqttStrategy stopped===")
		case point := <-pointChan:
			err := m.Publish(point)
			if err != nil {
				pkg.ErrChanFromContext(m.ctx) <- fmt.Errorf("MqttStrategy error occurred: %w", err)
			}
		}
	}
}

func (m *MqttStrategy) Publish(point pkg.Point) error {
	// 创建一个新的 map[string]interface{} 来存储解引用的字段
	decodedFields := make(map[string]interface{})
	for key, valuePtr := range point.Field {
		if valuePtr == nil {
			continue // 跳过 nil 值
		}

		decodedFields[key] = valuePtr
	}
	decodedFields["ts"] = point.Ts
	// 将 map 序列化为 JSON
	jsonData, err := json.Marshal(decodedFields)
	if err != nil {
		return fmt.Errorf("序列化 JSON 失败: %+v", err)
	}
	topic := fmt.Sprintf("gateway/%s/%s/fields", point.DeviceType, point.DeviceName)
	m.client.Publish(topic, 0, true, jsonData)
	m.logger.Debug("[MqttStrategy]发布消息到 %s: %s", zap.String("topic", topic), zap.String("data", string(jsonData)))
	return nil
}

// Stop 停止 MQTTStrategy
func (m *MqttStrategy) Stop() {
	m.client.Publish(m.info.WillTopic, 1, true, "offline")
	m.client.Disconnect(250)
}
