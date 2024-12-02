package connector

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"
)

// MockMQTTClient 是模拟的 MQTT 客户端，实现 MQTTClient 接口
type MockMQTTClient struct {
	mock.Mock
}

func (m *MockMQTTClient) IsConnectionOpen() bool {
	//TODO implement me
	panic("implement me")
}

func (m *MockMQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	//TODO implement me
	panic("implement me")
}

func (m *MockMQTTClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	//TODO implement me
	panic("implement me")
}

func (m *MockMQTTClient) Unsubscribe(topics ...string) mqtt.Token {
	//TODO implement me
	panic("implement me")
}

func (m *MockMQTTClient) AddRoute(topic string, callback mqtt.MessageHandler) {
	//TODO implement me
	panic("implement me")
}

func (m *MockMQTTClient) OptionsReader() mqtt.ClientOptionsReader {
	//TODO implement me
	panic("implement me")
}

// Connect 模拟 MQTT 客户端的连接
func (m *MockMQTTClient) Connect() mqtt.Token {
	args := m.Called()
	return args.Get(0).(mqtt.Token)
}

// Disconnect 模拟 MQTT 客户端的断开连接
func (m *MockMQTTClient) Disconnect(quiesce uint) {
	m.Called(quiesce)

}

// SubscribeMultiple 模拟 MQTT 客户端的多主题订阅
func (m *MockMQTTClient) SubscribeMultiple(filters map[string]byte, callback mqtt.MessageHandler) mqtt.Token {
	args := m.Called(filters, callback)
	return args.Get(0).(mqtt.Token)
}

// IsConnected 模拟 MQTT 客户端的连接状态
func (m *MockMQTTClient) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

// MockToken 是模拟的 MQTT Token
type MockToken struct {
	mock.Mock
	mqtt.Token
}

// Wait 模拟 Token 的等待行为
func (m *MockToken) Wait() bool {
	args := m.Called()
	return args.Bool(0)
}

// Error 模拟 Token 的错误
func (m *MockToken) Error() error {
	args := m.Called()
	return args.Error(0)
}

func TestMqttConnectorStart(t *testing.T) {
	ctx := context.Background()
	mockClient := new(MockMQTTClient)
	mockToken := new(MockToken)

	// 模拟配置
	config := &pkg.Config{
		Connector: pkg.ConnectorConfig{
			Para: map[string]interface{}{
				"broker":               "tcp://localhost:1883",
				"clientID":             "test-client",
				"username":             "test-user",
				"password":             "test-pass",
				"maxReconnectInterval": "5s",
				"topics": map[string]byte{
					"test/topic": 1,
				},
			},
		},
	}
	ctx = pkg.WithConfig(ctx, config)
	// 从 NewMqttConnector 返回的 Template 接口进行类型断言
	connector, err := NewMqttConnector(ctx)
	assert.NoError(t, err)

	// 使用类型断言将 connector 转换为 *MqttConnector 类型
	mqttConnector, ok := connector.(*MqttConnector)
	assert.True(t, ok, "转换为 *MqttConnector 失败")
	assert.True(t, ok, "转换为 *MqttConnector 失败")
	assert.NoError(t, err)
	mqttConnector.Client = mockClient

	// 模拟连接成功
	mockClient.On("Connect").Return(mockToken)
	mockToken.On("Wait").Return(true)
	mockToken.On("Error").Return(nil)

	// 模拟订阅成功
	mockClient.On("SubscribeMultiple", mock.Anything, mock.Anything).Return(mockToken)
	mockToken.On("Wait").Return(true)
	mockToken.On("Error").Return(nil)

	// 执行 Start 方法
	mqttConnector.Start()

	// 验证连接和订阅的调用次数
	mockClient.AssertCalled(t, "Connect")
	mockClient.AssertCalled(t, "SubscribeMultiple", mock.Anything, mock.Anything)
}

func TestMqttConnectorClose(t *testing.T) {
	ctx := context.Background()
	mockClient := new(MockMQTTClient)

	// 模拟 MQTT 连接状态
	mockClient.On("IsConnected").Return(true)
	mockClient.On("Disconnect", uint(250)).Return()

	// 创建 MqttConnector 并设置 mockClient
	mqttConnector := &MqttConnector{
		ctx:    ctx,
		Client: mockClient,
	}

	// 执行 Close 方法
	err := mqttConnector.Close()
	assert.NoError(t, err, "关闭 MqttConnector 不应出错")

	// 验证 Disconnect 是否被调用
	mockClient.AssertCalled(t, "Disconnect", uint(250))
}

// 测试未连接状态下的 Close
func TestMqttConnectorClose_NotConnected(t *testing.T) {
	ctx := context.Background()
	mockClient := new(MockMQTTClient)

	// 模拟 MQTT 未连接状态
	mockClient.On("IsConnected").Return(false)

	// 创建 MqttConnector 并设置 mockClient
	mqttConnector := &MqttConnector{
		ctx:    ctx,
		Client: mockClient,
	}

	// 执行 Close 方法
	err := mqttConnector.Close()
	assert.EqualError(t, err, "MQTT客户端未连接")
}

func TestMqttConnectorGetDataSource(t *testing.T) {
	ctx := context.Background()
	dataChan := make(chan string, 1)

	// 创建 MqttConnector
	mqttConnector := &MqttConnector{
		ctx:      ctx,
		dataChan: dataChan,
	}

	// 调用 GetDataSource
	dataSource, err := mqttConnector.GetDataSource()
	assert.NoError(t, err, "GetDataSource 不应出错")
	assert.Equal(t, dataChan, dataSource.Source, "数据源的 Source 应该与 dataChan 相同")
	assert.Nil(t, dataSource.MetaData, "MetaData 应为 nil")
}

// MockMessage 实现 mqtt.Message 接口
type MockMessage struct {
	topic   string
	payload []byte
}

func (m *MockMessage) Duplicate() bool {
	return false
}

func (m *MockMessage) Qos() byte {
	return 0
}

func (m *MockMessage) Retained() bool {
	return false
}

func (m *MockMessage) Topic() string {
	return m.topic
}

func (m *MockMessage) MessageID() uint16 {
	return 0
}

func (m *MockMessage) Payload() []byte {
	return m.payload
}

// 实现 Ack 方法
func (m *MockMessage) Ack() {
	// 模拟 Ack 操作，这里可以是一个空的实现
}

// 测试 messagePubHandler 方法
func TestMqttConnectorMessagePubHandler(t *testing.T) {
	ctx := context.Background()
	dataChan := make(chan string, 1)

	// 创建 MqttConnector
	mqttConnector := &MqttConnector{
		ctx:      ctx,
		dataChan: dataChan,
	}

	// 创建模拟的 MQTT 消息
	msg := &MockMessage{
		topic:   "test/topic",
		payload: []byte("test-message"),
	}

	// 执行消息处理函数
	mqttConnector.messagePubHandler(nil, msg)

	// 验证消息是否被发送到数据通道
	select {
	case data := <-dataChan:
		assert.Equal(t, "test-message", data)
	case <-time.After(1 * time.Second):
		t.Fatal("消息未发送到数据通道")
	}
}

func TestMqttConnectorConnectHandler(t *testing.T) {

	ctx := pkg.WithLogger(context.Background(), logger)
	// 创建 MqttConnector 实例
	mqttConnector := &MqttConnector{
		ctx: ctx,
	}

	// 模拟 MQTT 客户端
	mockClient := new(MockMQTTClient)

	// 执行连接成功的回调函数
	mqttConnector.connectHandler(mockClient)
}

func TestMqttConnectorConnectLostHandler(t *testing.T) {
	ctx := pkg.WithLogger(context.Background(), logger)
	// 创建 MqttConnector 实例
	mqttConnector := &MqttConnector{
		ctx: ctx,
	}

	// 模拟连接丢失的错误
	err := fmt.Errorf("测试：connection lost")

	// 执行连接丢失的回调函数
	mqttConnector.connectLostHandler(nil, err)

}
