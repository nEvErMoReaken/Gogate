package mqtt

import (
	"fmt"
	"gateway/common"
	"gateway/model"
	"gateway/parser/jsonType"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/viper"
)

// MqttConnector Connector的TcpServer版本实现
type MqttConnector struct {
	MqttConfig *MqttConfig   // 配置
	ChDone     chan struct{} // 停止通道
	v          *viper.Viper
	comm       *common.Config
	client     *mqtt.Client
	conversion *jsonType.ConversionConfig
}

func init() {
	model.RegisterConn("tcpServer", NewMqttConnector)
}

func (m *MqttConnector) Listen() error {
	// 检查客户端是否已经连接
	if token := (*m.client).Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT连接失败: %v", token.Error())
	}

	// 订阅多个话题
	token := (*m.client).SubscribeMultiple(m.MqttConfig.Topics, messagePubHandler)
	token.Wait() // 等待订阅完成
	if err := token.Error(); err != nil {
		return fmt.Errorf("MQTT订阅失败: %v", err)
	}

	// 持续运行监听消息
	common.Log.Infof("MQTT订阅成功，正在监听消息")
	return nil
}

func (m *MqttConnector) Close() error {
	// 关闭MQTT客户端，优雅关闭
	if m.client != nil && (*m.client).IsConnected() {
		(*m.client).Disconnect(250)
		common.Log.Infof("MQTT连接已断开")
		return nil
	}

	return fmt.Errorf("MQTT客户端未连接")
}

func NewMqttConnector(comm *common.Config, v *viper.Viper, stopChan chan struct{}) model.Connector {
	// 1. 初始化mqtt配置
	mqttConfig, err := UnmarshalMqttConfig(v)
	if err != nil {
		common.Log.Fatalf("初始化MQTT配置失败: %v", err)
	}
	// 2. 初始化转换配置
	conversion, err := jsonType.UnmarshalJsonParseConfig(v)
	if err != nil {
		common.Log.Fatalf("初始化转换配置失败: %v", err)
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(mqttConfig.Broker)
	opts.SetClientID(mqttConfig.ClientID)
	opts.SetUsername(mqttConfig.Username) // 如果需要用户名和密码
	opts.SetPassword(mqttConfig.Password) // 如果需要用户名和密码

	// 设置自动重连
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(mqttConfig.MaxReconnectInterval) // 设置重连间隔时间

	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler

	// 创建 MQTT 客户端
	client := mqtt.NewClient(opts)

	// 2. 创建一个新的 MqttConnector
	mqttConnector := &MqttConnector{
		ChDone:     stopChan,
		v:          v,
		comm:       comm,
		client:     &client,
		MqttConfig: mqttConfig,
		conversion: conversion,
	}

	// 检查客户端是否能成功创建并连接
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		common.Log.Fatalf("MQTT客户端连接失败: %v", token.Error())
	}
	return mqttConnector
}

// 消息处理函数
var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	common.Log.Infof("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
}

// 连接成功回调
var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	common.Log.Infof("成功连接至MQTT broker")
}

// 连接丢失回调
var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	common.Log.Errorf("Connect lost: %v , 正在执行重连逻辑", err)
	// 这里paho会自动重连，不需要手动重连
}
