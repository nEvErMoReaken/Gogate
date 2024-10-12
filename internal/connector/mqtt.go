package connector

import (
	"fmt"
	json2 "gateway/internal/parser"
	"gateway/internal/parser/json"
	"gateway/internal/pkg"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/viper"
)

// MqttConnector Connector的TcpServer版本实现
type MqttConnector struct {
	MqttConfig         *pkg.MqttConfig         // 配置
	ChDone             chan struct{}           // 停止通道
	v                  *viper.Viper            // 配置文件
	comm               *pkg.Config             // 全局配置
	client             *mqtt.Client            // MQTT 客户端
	conversion         *pkg.JsonParseConfig    // 转换配置
	snapshotCollection *pkg.SnapshotCollection // 快照集合
}

func init() {
	Register("mqtt", NewMqttConnector)
}

func (m *MqttConnector) Listen() error {
	// 检查客户端是否已经连接
	if token := (*m.client).Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT连接失败: %v", token.Error())
	}

	// 订阅多个话题
	token := (*m.client).SubscribeMultiple(m.MqttConfig.Topics, m.messagePubHandler)
	token.Wait() // 等待订阅完成
	if err := token.Error(); err != nil {
		return fmt.Errorf("MQTT订阅失败: %v", err)
	}

	// 持续运行监听消息
	pkg.Log.Infof("MQTT订阅成功，正在监听消息")
	return nil
}

func (m *MqttConnector) Close() error {
	// 关闭MQTT客户端，优雅关闭
	if m.client != nil && (*m.client).IsConnected() {
		(*m.client).Disconnect(250)
		pkg.Log.Infof("MQTT连接已断开")
		return nil
	}

	return fmt.Errorf("MQTT客户端未连接")
}

func NewMqttConnector(comm *pkg.Config, stopChan chan struct{}) Connector {
	// 1. 初始化mqtt配置
	mqttConfig, err := UnmarshalMqttConfig(v)
	if err != nil {
		pkg.Log.Fatalf("初始化MQTT配置失败: %v", err)
	}
	// 2. 初始化转换配置
	conversion, err := json.UnmarshalJsonParseConfig(v)
	if err != nil {
		pkg.Log.Fatalf("初始化转换配置失败: %v", err)
	}
	// 3. 创建一个新的快照集合
	snapshotCollection := make(pkg.SnapshotCollection)
	// 4. 创建一个新的 MQTT 客户端
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

	// 5. 创建一个新的 MqttConnector
	mqttConnector := &MqttConnector{
		ChDone:             stopChan,
		v:                  v,
		comm:               comm,
		client:             &client,
		MqttConfig:         mqttConfig,
		conversion:         conversion,
		snapshotCollection: &snapshotCollection,
	}

	// 检查客户端是否能成功创建并连接
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		pkg.Log.Fatalf("MQTT客户端连接失败: %v", token.Error())
	}
	return mqttConnector
}

func (m *MqttConnector) messagePubHandler(_ mqtt.Client, msg mqtt.Message) {
	pkg.Log.Infof("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())

	// 1. 将 MQTT 消息 Payload 转换为字符串
	js := string(msg.Payload())

	// 2. 调用 ConversionToSnapshot 并传入必要的参数
	json2.ConversionToSnapshot(js, m.conversion, m.snapshotCollection, m.comm)

	// 3. 发射所有快照
	m.snapshotCollection.LaunchALL()
}

// 连接成功回调
var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	pkg.Log.Infof("成功连接至MQTT broker")
}

// 连接丢失回调
var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	pkg.Log.Errorf("Connect lost: %v , 正在执行重连逻辑", err)
	// 这里paho会自动重连，不需要手动重连
}
