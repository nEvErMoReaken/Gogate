package test

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/internal"
	"gateway/internal/pkg"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/mochi-co/mqtt/server"
	"github.com/mochi-co/mqtt/server/listeners"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"net"
	"os"
	"strconv"
	"testing"
	"time"
)

type helper struct {
	config  *pkg.Config
	log     *zap.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	errChan chan error
}

func chooseConfig(subject string) (*helper, error) {
	// 1. 初始化common yaml
	var h helper
	var err error
	// 打印当前路径

	h.config, err = pkg.InitCommon("yaml/" + subject)
	if err != nil {
		fmt.Printf("加载配置失败: %s", err)
		return nil, err
	}
	// 2. 初始化log
	h.log = pkg.NewLogger(&h.config.Log)

	h.log.Info("测试程序启动", zap.String("version", h.config.Version))
	h.log.Info("配置信息", zap.Any("common", h.config))
	h.log.Info("*** 初始化流程开始 ***")
	// 3. 创建上下文
	var c context.Context
	c, h.cancel = context.WithCancel(context.Background())
	h.errChan = make(chan error, 10) // 创建一个只写的全局错误通道, 缓存大小为10
	c = pkg.WithErrChan(c, h.errChan)
	// 将config挂载到ctx上
	c = pkg.WithConfig(c, h.config)
	// 将logger挂载到ctx上
	h.ctx = pkg.WithLogger(c, h.log)
	return &h, nil
}

func TestTcpServer2Mqtt(t *testing.T) {
	h, err := chooseConfig("tcpServer2mqtt")
	if err != nil {
		// 触发测试失败
		t.Fatalf("配置初始化失败: %v", err)
		return
	}
	// 启动嵌入式 MQTT Broker
	mqttBroker := startEmbeddedMqttBroker(t)
	defer func(mqttBroker *server.Server) {
		err := mqttBroker.Close()
		if err != nil {
			h.log.Error("关闭 MQTT 服务器失败: %v", zap.Error(err))
		}
	}(mqttBroker) // 测试结束时关闭 Broker

	// 启动流程
	go internal.NewPipeline(h.ctx)

	// 设置 MQTT 订阅
	received := make(chan []byte, 1)
	client, err := subscribeMqtt("gateway/traction_system/status/fields", func(client mqtt.Client, msg mqtt.Message) {
		received <- msg.Payload()
	})
	if err != nil {
		t.Fatalf("MQTT 订阅失败: %v", err)
	}

	defer client.Disconnect(250)
	time.Sleep(1 * time.Second)
	// 模拟 TCP 客户端发送数据
	err = sendTcpData("localhost:8080", []byte{0xFF})

	if err != nil {
		t.Fatalf("TCP 数据发送失败: %v", err)
	}

	// 等待接收 MQTT 消息
	select {
	case bad := <-h.errChan:
		h.log.Error("Error occurred", zap.Error(bad))
		h.cancel() // 取消上下文
		// 等待其他可能的错误
		go func() {
			for err := range h.errChan {
				h.log.Error("Error occurred before shutdown", zap.Error(err))
			}
		}()
		time.Sleep(1 * time.Second) // 确保日志输出完整
		err := h.log.Sync()
		if err != nil {
			h.log.Error("程序退出时同步日志失败: %s", zap.Error(err))
		}
		os.Exit(1)
	case msg := <-received:
		h.log.Info("接收到 MQTT 消息", zap.Any("msg", string(msg)))
		var fieldMap map[string]interface{}
		// 反序列化 string(msg)
		err = json.Unmarshal(msg, &fieldMap)
		if err != nil {
			t.Fatalf("反序列化消息失败: %v", err)
		}
		h.log.Info("接收到 MQTT 消息", zap.Any("point", fieldMap))
		for i := 1; i <= 8; i++ {
			key := fmt.Sprintf("RIOM_sta_%d", i)
			expectedValue := fmt.Sprintf("%d", 1)
			assert.Equal(t, expectedValue, strconv.Itoa(int(fieldMap[key].(float64))), "字段 %s 的值不匹配", key)
		}
		assert.NotNil(t, fieldMap["ts"])
	case <-time.After(5 * time.Second):
		t.Fatal("超时未接收到 MQTT 消息")
	}
	h.cancel()
}

// 模拟 TCP 客户端发送数据
func sendTcpData(address string, data []byte) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("无法连接到服务器: %v", err)
	}
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			fmt.Printf("关闭连接失败: %v", err)
		}
	}(conn)

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("发送数据失败: %v", err)
	}
	return nil
}

// 订阅 MQTT 主题以接收消息
func subscribeMqtt(topic string, messageHandler mqtt.MessageHandler) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().AddBroker("tcp://localhost:1883").SetClientID("testClient")
	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("无法连接到 MQTT 服务器: %v", token.Error())
	}

	token := client.Subscribe(topic, 1, messageHandler)
	if token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("订阅 MQTT 主题失败: %v", token.Error())
	}

	return client, nil
}

// 启动一个嵌入式 MQTT broker
func startEmbeddedMqttBroker(t *testing.T) *server.Server {
	// 创建一个新的 MQTT 服务器
	srv := server.New()

	// 创建一个 TCP 监听器，绑定到端口1883
	tcp := listeners.NewTCP("t1", ":1883")
	err := srv.AddListener(tcp, nil)
	if err != nil {
		t.Fatalf("无法添加 MQTT 监听器: %v", err)
	}

	// 启动 MQTT 服务器
	go func() {
		if err := srv.Serve(); err != nil {
			t.Errorf("MQTT 服务器启动失败: %v", err)
			return
		}
		fmt.Printf("test mqtt broker 启动成功")
	}()

	return srv
}
