package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/internal/pkg"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/mochi-co/mqtt/server"
	"github.com/mochi-co/mqtt/server/listeners"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// 测试 NewMqttStrategy 成功情况
func TestNewMqttStrategy_Success(t *testing.T) {

	ctx := pkg.WithLogger(context.Background(), logger)

	// 启动存中的 MQTT broker
	srv := server.New()

	// 创建一个 TCP 监听器，监听本地 1883 端口
	tcp := listeners.NewTCP("t1", "localhost:1883")
	err := srv.AddListener(tcp, nil)
	assert.NoError(t, err)

	// 启动 MQTT broker
	go func() {
		if err = srv.Serve(); err != nil {
			t.Log("MQTT 服务器错误:", err)
		}
	}()
	defer func(srv *server.Server) {
		err = srv.Close()
		if err != nil {

		}
	}(srv)

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 模拟配置
	config := &pkg.Config{
		Strategy: []pkg.StrategyConfig{
			{
				Enable: true,
				Type:   "mqtt",
				Para: map[string]interface{}{
					"url":       "tcp://localhost:1883",
					"clientID":  "testClient",
					"username":  "user",
					"password":  "pass",
					"willTopic": "gateway/status",
				},
			},
		},
	}

	// 创建错误通道并添加到上下文中
	errChan := make(chan error, 1)
	ctx = pkg.WithConfig(ctx, config)
	ctx = pkg.WithLogger(ctx, logger)
	ctx = pkg.WithErrChan(ctx, errChan)

	// 创建 MqttStrategy 实例
	strategyInterface, err := NewMqttStrategy(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, strategyInterface)

	// 类型断言
	strategy, ok := strategyInterface.(*MqttStrategy)
	assert.True(t, ok)

	// 测试策略的属性
	assert.Equal(t, "tcp://localhost:1883", strategy.info.URL)
	assert.Equal(t, "testClient", strategy.info.ClientID)
	assert.Equal(t, "user", strategy.info.UserName)
	assert.Equal(t, "pass", strategy.info.Password)
	assert.Equal(t, "gateway/status", strategy.info.WillTopic)
	ph := make(chan pkg.Point)
	// 启动策略
	go strategy.Start(ph)
	defer strategy.Stop()

	// 等待策略连接
	time.Sleep(100 * time.Millisecond)

	// 准备一个要发布的点
	point := pkg.Point{
		Device: "device.vobc",
		Field: map[string]interface{}{
			"temperature": 25.5,
			"humidity":    60.0,
		},
	}

	// 将点发送到策略的通道
	ph <- point

	// 设置一个 MQTT 客户端来订阅并验证发布的消息
	subscriberOpts := MQTT.NewClientOptions()
	subscriberOpts.AddBroker("tcp://localhost:1883")
	subscriberOpts.SetClientID("testSubscriber")

	// 创建一个通道来接收消息
	received := make(chan []byte, 1)

	// 消息处理函数
	var messageHandler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		received <- msg.Payload()
	}

	// 创建并启动订阅者客户端
	subscriber := MQTT.NewClient(subscriberOpts)
	if token := subscriber.Connect(); token.Wait() && token.Error() != nil {
		t.Fatalf("订阅者连接失败: %v", token.Error())
	}
	defer subscriber.Disconnect(250)

	// 订阅主题
	topic := fmt.Sprintf("gateway/device/vobc/fields")
	if token := subscriber.Subscribe(topic, 0, messageHandler); token.Wait() && token.Error() != nil {
		t.Fatalf("订阅者订阅失败: %v", token.Error())
	}

	// 等待接收消息或超时
	select {
	case msg := <-received:
		// 反序列化 JSON 消息
		var data map[string]interface{}
		err = json.Unmarshal(msg, &data)
		assert.NoError(t, err)
		assert.Equal(t, point.Field["temperature"], data["temperature"])
		assert.Equal(t, point.Field["humidity"], data["humidity"])
	case <-time.After(2 * time.Second):
		t.Fatalf("未及时从%s topic收到消息", topic)
	}
}
