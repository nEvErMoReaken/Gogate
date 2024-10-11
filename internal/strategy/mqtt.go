package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/internal/pkg"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/mitchellh/mapstructure"
)

// 初始化函数，注册 IoTDB 策略
func init() {
	// 注册发送策略
	RegisterStrategy("mqtt", NewMqttStrategy)
}

// MqttStrategy 实现将数据发布到 MQTT 的逻辑
type MqttStrategy struct {
	client    MQTT.Client
	pointChan chan pkg.Point
	stopChan  chan struct{}
	info      MQTTInfo
}

// MQTTInfo MQTT的专属配置
type MQTTInfo struct {
	URL       string `mapstructure:"url"`
	ClientID  string `mapstructure:"clientID"`
	UserName  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
	WillTopic string `mapstructure:"willTopic"`
}

// GetChan Step.2
func (m *MqttStrategy) GetChan() chan pkg.Point {
	return m.pointChan
}

// Start Step.3
func (m *MqttStrategy) Start() {
	defer m.client.Disconnect(250)
	pkg.Log.Info("MqttStrategy started")
	// 发布网关上线的状态
	m.client.Publish(m.info.WillTopic, 1, true, "online")
	for {
		select {
		case <-m.stopChan:
			m.Stop()
			return
		case point := <-m.pointChan:
			m.Publish(point)
		}
	}
}

func (m *MqttStrategy) Publish(point pkg.Point) {
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
		pkg.Log.Errorf("序列化 JSON 失败: %+v", err)
		return
	}
	topic := fmt.Sprintf("gateway/%s/%s/fields", point.DeviceType, point.DeviceName)
	m.client.Publish(topic, 0, true, jsonData)
	pkg.Log.Infof("[MqttStrategy]发布消息到 %s: %s", topic, string(jsonData))
}
func NewMqttStrategy(ctx context.Context) pkg.SendStrategy {
	var info MQTTInfo
	// 将 map 转换为结构体
	if err := mapstructure.Decode(dbConfig.Config, &info); err != nil {
		pkg.Log.Fatalf("[NewInfluxDbStrategy] Error decoding map to struct: %v", err)
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

	// 创建 MQTT 客户端
	mqCLi := MQTT.NewClient(opts)

	// 连接到 MQTT Broker
	if token := mqCLi.Connect(); token.Wait() && token.Error() != nil {
		pkg.Log.Errorf("连接到 MQTT Broker 失败:%+v", token.Error())

	}
	return &MqttStrategy{
		client:    mqCLi,
		pointChan: make(chan pkg.Point, 200),
		stopChan:  stopChan,
		info:      info,
	}
}

// Stop 停止 MQTTStrategy
func (m *MqttStrategy) Stop() {
	m.client.Publish(m.info.WillTopic, 1, true, "offline")
}
