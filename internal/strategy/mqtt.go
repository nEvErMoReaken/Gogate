package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/internal/pkg"
	"gateway/util"
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

// MqttStrategy 实现将数据发布到 MQTT 的逻辑
type MqttStrategy struct {
	client MQTT.Client
	info   MQTTInfo
	core   Core
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
func NewMqttStrategy(ctx context.Context) (Strategy, error) {
	logger := pkg.LoggerFromContext(ctx)
	config := pkg.ConfigFromContext(ctx)
	var info MQTTInfo
	for _, strategyConfig := range config.Strategy {
		if strategyConfig.Enable && strategyConfig.Type == "influxDB" {
			// 将 map 转换为结构体
			if err := mapstructure.Decode(strategyConfig, &info); err != nil {
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
		logger.Error("MQTT connection lost", zap.Error(err))
	})

	reconnectAttempts := 0 // 重连尝试计数器

	// 重连中的回调函数
	opts.SetReconnectingHandler(func(client MQTT.Client, opts *MQTT.ClientOptions) {
		reconnectAttempts++
		logger.Info("Attempting to reconnect...", zap.Int("attempt", reconnectAttempts))

		// 如果超过最大重连次数
		if reconnectAttempts >= maxReconnectAttempts {
			// 断开连接
			client.Disconnect(250)
			// 将错误传递到错误通道，通知上层程序
			errChan := util.ErrChanFromContext(ctx)
			if errChan != nil {
				errChan <- fmt.Errorf("[MqttStrategy]MQTT reached max reconnect attempts, stopping client")
			}
		}
	})

	// 成功连接时重置计数器
	opts.SetOnConnectHandler(func(client MQTT.Client) {
		reconnectAttempts = 0
		logger.Info("Successfully connected to MQTT Broker.")
	})

	// 创建 MQTT 客户端
	mqCLi := MQTT.NewClient(opts)
	// 连接到 MQTT Broker
	if token := mqCLi.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("连接到 MQTT Broker 失败:%+v", token.Error())
	}

	return &MqttStrategy{
		client: mqCLi,
		info:   info,
		core:   Core{StrategyType: "mqtt", pointChan: make(chan pkg.Point, 200), ctx: ctx},
		logger: logger,
	}, nil
}

// GetCore Step.1
func (m *MqttStrategy) GetCore() Core {
	return m.core
}

// GetChan Step.2
func (m *MqttStrategy) GetChan() chan pkg.Point {
	return m.core.pointChan
}

// Start Step.3
func (m *MqttStrategy) Start() {
	defer m.client.Disconnect(250)
	m.logger.Info("===MqttStrategy started===")
	// 发布网关上线的状态
	m.client.Publish(m.info.WillTopic, 1, true, "online")
	for {
		select {
		case <-m.core.ctx.Done():
			m.Stop()
			pkg.LoggerFromContext(m.core.ctx).Info("===MqttStrategy stopped===")
		case point := <-m.core.pointChan:
			err := m.Publish(point)
			if err != nil {
				util.ErrChanFromContext(m.core.ctx) <- fmt.Errorf("MqttStrategy error occurred: %w", err)
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
