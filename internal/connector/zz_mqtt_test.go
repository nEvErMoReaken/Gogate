package connector

import (
	"context"
	"errors"
	"gateway/internal/pkg"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

type MockMQTTClient struct {
	mock.Mock
}

func (m *MockMQTTClient) Connect() mqtt.Token {
	args := m.Called()
	return args.Get(0).(mqtt.Token)
}

func (m *MockMQTTClient) Disconnect(quiesce uint) {
	m.Called(quiesce)
}

func (m *MockMQTTClient) SubscribeMultiple(filters map[string]byte, callback mqtt.MessageHandler) mqtt.Token {
	args := m.Called(filters, callback)
	return args.Get(0).(mqtt.Token)
}

func (m *MockMQTTClient) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

// MockToken 用于模拟 MQTT Token
type MockToken struct {
	mock.Mock
}

func (t *MockToken) Wait() bool {
	args := t.Called()
	return args.Bool(0)
}

func (t *MockToken) WaitTimeout(timeout time.Duration) bool {
	args := t.Called(timeout)
	return args.Bool(0)
}

func (t *MockToken) Error() error {
	args := t.Called()
	return args.Error(0)
}

type MockMessage struct {
	TopicStr   string
	PayloadStr []byte
}

func (m *MockMessage) Ack() {
	return
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
	return m.TopicStr
}

func (m *MockMessage) MessageID() uint16 {
	return 0
}

func (m *MockMessage) Payload() []byte {
	return m.PayloadStr
}

// Mock Logger for capturing log outputs
var logger, _ = zap.NewDevelopment()

var commonCtx, cancel = context.WithCancel(pkg.WithErrChan(pkg.WithLogger(context.Background(), logger), make(chan error, 5)))

func TestMqttConnector(t *testing.T) {
	Convey("给定一个合法的 ctx 和配置", t, func() {
		ctx := pkg.WithLogger(context.Background(), logger)
		mockClient := new(MockMQTTClient)
		mockToken := new(MockToken)

		// 模拟配置
		mqttConfig := &MqttConfig{
			Broker:               "tcp://localhost:1883",
			ClientID:             "test-client",
			Username:             "user",
			Password:             "password",
			MaxReconnectInterval: 10 * time.Second,
			Topics:               map[string]byte{"test/topic": 0},
		}

		mqttConn := &MqttConnector{
			ctx:    ctx,
			config: mqttConfig,
			Client: mockClient,
			Sink:   pkg.NewMessageDataSource(),
		}

		Convey("当调用 Start 并连接成功时", func() {
			mockClient.On("Connect").Return(mockToken)
			mockToken.On("Wait").Return(true)
			mockToken.On("Error").Return(nil)
			mockClient.On("SubscribeMultiple", mock.Anything, mock.Anything).Return(mockToken)

			sourceChan := make(chan pkg.DataSource)
			go func() {
				err := mqttConn.Start(sourceChan)
				Convey("应该成功启动且没有错误", func() {
					So(err, ShouldBeNil)
					mockClient.AssertCalled(t, "Connect")
					mockClient.AssertCalled(t, "SubscribeMultiple", mock.Anything, mock.Anything)
				})
			}()
		})

		Convey("当调用 Start 并连接失败时", func() {
			mockClient.On("Connect").Return(mockToken)
			mockToken.On("Wait").Return(true)
			mockToken.On("Error").Return(errors.New("连接失败"))

			sourceChan := make(chan pkg.DataSource)
			go func() {
				err := mqttConn.Start(sourceChan)
				Convey("应该将错误写入上下文的 ErrChan", func() {
					So(err, ShouldBeNil)
					// 模拟读取错误通道，验证是否收到错误
					errChan := pkg.ErrChanFromContext(ctx)
					So(<-errChan, ShouldNotBeNil)
				})
			}()
		})

		Convey("当调用 Close 并客户端已连接时", func() {
			mockClient.On("IsConnected").Return(true)
			mockClient.On("Disconnect", uint(250)).Return()

			err := mqttConn.Close()

			Convey("应该成功断开连接且没有错误", func() {
				So(err, ShouldBeNil)
				mockClient.AssertCalled(t, "Disconnect", uint(250))
			})
		})

		Convey("当调用 Close 并客户端未连接时", func() {
			mockClient.On("IsConnected").Return(false)

			err := mqttConn.Close()

			Convey("应该返回未连接的错误", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "MQTT客户端未连接")
			})
		})

		Convey("当调用 messagePubHandler 并消息成功写入时", func() {
			message := &MockMessage{
				TopicStr:   "test/topic",
				PayloadStr: []byte("test message"),
			}

			mqttConn.messagePubHandler(nil, message)

			Convey("应该将消息写入 Sink 且没有错误", func() {
				data, err := mqttConn.Sink.ReadOne()
				So(err, ShouldBeNil)
				So(data, ShouldResemble, []byte("test message"))
			})
		})

	})
}

func TestNewMqttConnector(t *testing.T) {
	Convey("创建一个新的 MQTT 连接器", t, func() {
		ctx := pkg.WithLogger(context.Background(), logger)

		Convey("当配置正确时", func() {
			// 模拟合法的配置
			validConfig := pkg.Config{
				Connector: pkg.ConnectorConfig{
					Para: map[string]interface{}{
						"broker":               "tcp://localhost:1883",
						"clientID":             "test-client",
						"username":             "user",
						"password":             "password",
						"maxreconnectinterval": "10s",
						"topics":               map[string]byte{"test/topic": 0},
					},
				},
			}
			ctx = pkg.WithConfig(ctx, &validConfig)

			connector, err := NewMqttConnector(ctx)

			Convey("应该成功返回一个 MqttConnector 实例", func() {
				So(err, ShouldBeNil)
				So(connector, ShouldNotBeNil)
				mqttConn := connector.(*MqttConnector)
				So(mqttConn.config.Broker, ShouldEqual, "tcp://localhost:1883")
				So(mqttConn.config.ClientID, ShouldEqual, "test-client")
				So(mqttConn.config.Topics, ShouldContainKey, "test/topic")
			})
		})

		Convey("当 maxReconnectInterval 配置错误时", func() {
			// 模拟错误的 maxReconnectInterval 配置
			invalidConfig := pkg.Config{
				Connector: pkg.ConnectorConfig{
					Para: map[string]interface{}{
						"maxreconnectinterval": "invalid_duration",
					},
				},
			}
			ctx = pkg.WithConfig(ctx, &invalidConfig)

			connector, err := NewMqttConnector(ctx)

			Convey("应该返回解析错误", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "解析超时配置失败")
				So(connector, ShouldBeNil)
			})
		})

	})
}
