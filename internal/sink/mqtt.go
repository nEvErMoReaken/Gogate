package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/internal/pkg"
	"strings"
	"time"

	"go.uber.org/zap"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/mitchellh/mapstructure"
)

// 初始化函数，注册 IoTDB 策略
func init() {
	// 注册发送策略
	Register("mqtt", NewMqttStrategy)
}

// MQTTClientInterface 定义了我们需要的 MQTT 客户端方法
type MQTTClientInterface interface {
	Connect() mqtt.Token
	Disconnect(quiesce uint)
	Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token
	// 根据需要添加其他方法
}

// MqttInfo MQTT's specific configuration
type MqttInfo struct {
	Broker         string `mapstructure:"broker"`
	Port           int    `mapstructure:"port"`
	Username       string `mapstructure:"username"`
	Password       string `mapstructure:"password"`
	ClientID       string `mapstructure:"clientID"`
	Topic          string `mapstructure:"topic"` // Base topic
	QoS            byte   `mapstructure:"qos"`
	Retained       bool   `mapstructure:"retained"`
	KeepAliveSec   uint   `mapstructure:"keepAliveSec"`
	PingTimeoutSec uint   `mapstructure:"pingTimeoutSec"`
}

// MqttStrategy 实现将数据发布到 MQTT 的逻辑
type MqttStrategy struct {
	client mqtt.Client
	info   MqttInfo
	ctx    context.Context
	cancel context.CancelFunc
	logger *zap.Logger
}

// NewMqttStrategy Step.0 Constructor
func NewMqttStrategy(ctx context.Context) (Template, error) {
	log := pkg.LoggerFromContext(ctx)
	config := pkg.ConfigFromContext(ctx)
	var info MqttInfo
	found := false

	for _, strategyConfig := range config.Strategy {
		if strategyConfig.Enable && strategyConfig.Type == "mqtt" {
			if err := mapstructure.Decode(strategyConfig.Para, &info); err != nil {
				log.Error("Failed to decode MQTT config", zap.Error(err), zap.Any("config", strategyConfig.Para))
				return nil, fmt.Errorf("failed to decode MQTT config: %w", err)
			}
			found = true
			break
		}
	}

	if !found {
		log.Warn("No enabled MQTT strategy configuration found")
		return nil, fmt.Errorf("no enabled MQTT strategy configuration found")
	}

	if info.Broker == "" {
		return nil, fmt.Errorf("mqtt config validation failed: 'broker' is required")
	}
	if info.Topic == "" {
		return nil, fmt.Errorf("mqtt config validation failed: 'topic' is required")
	}
	if info.Port == 0 {
		info.Port = 1883 // Default MQTT port
	}
	if info.ClientID == "" {
		info.ClientID = fmt.Sprintf("gateway-mqtt-%d", time.Now().UnixNano())
		log.Info("MQTT ClientID not set, generated default", zap.String("clientID", info.ClientID))
	}
	if info.KeepAliveSec == 0 {
		info.KeepAliveSec = 60
	}
	if info.PingTimeoutSec == 0 {
		info.PingTimeoutSec = 2
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", info.Broker, info.Port))
	opts.SetClientID(info.ClientID)
	opts.SetUsername(info.Username)
	opts.SetPassword(info.Password)
	opts.SetKeepAlive(time.Duration(info.KeepAliveSec) * time.Second)
	opts.SetPingTimeout(time.Duration(info.PingTimeoutSec) * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)

	// Connection logging
	opts.OnConnect = func(client mqtt.Client) {
		log.Info("MQTT connected", zap.String("broker", info.Broker))
	}
	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		log.Error("MQTT connection lost", zap.Error(err), zap.String("broker", info.Broker))
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Error("MQTT connection failed", zap.Error(token.Error()), zap.String("broker", info.Broker))
		return nil, fmt.Errorf("mqtt connection failed for %s: %w", info.Broker, token.Error())
	}

	strategyCtx, cancel := context.WithCancel(ctx)

	return &MqttStrategy{
		client: client,
		info:   info,
		ctx:    strategyCtx,
		cancel: cancel,
		logger: log.With(zap.String("sink_type", "mqtt"), zap.String("broker", info.Broker), zap.String("base_topic", info.Topic)),
	}, nil
}

// GetType Step.1
func (b *MqttStrategy) GetType() string {
	return "mqtt"
}

// Start Step.2
func (b *MqttStrategy) Start(sink chan *pkg.PointPackage) {
	metrics := pkg.GetPerformanceMetrics()
	b.logger.Info("===MQTTStrategy Started===")

OuterLoop:
	for {
		select {
		case <-b.ctx.Done():
			b.logger.Info("===MQTTStrategy Stopping===")
			break OuterLoop
		case pointPackage, ok := <-sink:
			if !ok {
				b.logger.Info("Input channel closed, stopping MQTTStrategy")
				break OuterLoop
			}
			if pointPackage == nil || len(pointPackage.Points) == 0 {
				b.logger.Debug("Skipping nil or empty point package")
				continue
			}

			metrics.IncMsgReceived("mqtt_strategy")
			publishTimer := metrics.NewTimer("mqtt_strategy_publish_batch")
			pointsPublished := 0

			for _, point := range pointPackage.Points {
				if point == nil {
					b.logger.Warn("Skipping nil point in package", zap.String("frameId", pointPackage.FrameId))
					continue
				}

				// Determine device ID for topic (e.g., from 'id' tag)
				deviceID := "unknown_device"
				if idVal, ok := point.Tag["id"]; ok {
					if idStr, isStr := idVal.(string); isStr && idStr != "" {
						deviceID = idStr
					} else {
						b.logger.Warn("Tag 'id' is not a non-empty string, using default deviceID for topic", zap.Any("tag_id", idVal), zap.String("frameId", pointPackage.FrameId))
					}
				} else {
					b.logger.Warn("Tag 'id' not found, using default deviceID for topic", zap.String("frameId", pointPackage.FrameId))
				}

				// Construct topic: base_topic/deviceID
				topic := strings.TrimSuffix(b.info.Topic, "/") + "/" + deviceID

				// Create payload similar to Kafka for consistency
				payloadMap := map[string]interface{}{
					"tags":   point.Tag,
					"fields": point.Field,
					"ts":     pointPackage.Ts.UnixNano(),
				}

				jsonData, err := json.Marshal(payloadMap)
				if err != nil {
					metrics.IncErrorCount()
					metrics.IncMsgErrors("mqtt_strategy_json_marshal")
					b.logger.Error("Failed to marshal point to JSON for MQTT",
						zap.Error(err),
						zap.String("topic", topic),
						zap.Any("point_tags", point.Tag),
						zap.String("frameId", pointPackage.FrameId))
					continue // Skip this point
				}

				token := b.client.Publish(topic, b.info.QoS, b.info.Retained, jsonData)
				// For QoS > 0, Wait can be used, but for performance, we might not wait here for each message.
				// Consider adding a timeout for the wait if used: token.WaitTimeout(duration)
				// For now, we are fire-and-forget for simplicity, relying on AutoReconnect.
				// If token.Error() is checked, it must be after token.Wait() or token.WaitTimeout().
				_ = token // Explicitly ignore the token for now to avoid build warnings
				// A more robust implementation might handle errors here, e.g., if the client is disconnected.
				// However, paho library handles offline buffering to some extent.
				b.logger.Debug("Message prepared for MQTT", zap.String("topic", topic), zap.Int("payload_size", len(jsonData)))
				pointsPublished++
			}

			duration := publishTimer.StopAndLog(b.logger)
			if pointsPublished > 0 {
				metrics.IncMsgProcessed("mqtt_strategy") // Count based on successfully prepared points
				b.logger.Debug("Points processed for MQTT publishing in batch",
					zap.Int("published_count", pointsPublished),
					zap.Int("total_in_package", len(pointPackage.Points)),
					zap.Duration("duration", duration),
					zap.String("frameId", pointPackage.FrameId))
			} else if len(pointPackage.Points) > 0 {
				b.logger.Warn("No points were published from the package due to errors or empty data",
					zap.Int("total_in_package", len(pointPackage.Points)),
					zap.Duration("duration", duration),
					zap.String("frameId", pointPackage.FrameId))
			}
		}
	}
	b.logger.Info("===MQTTStrategy Finished===")
	if b.client.IsConnected() {
		b.client.Disconnect(250) // Graceful disconnect with 250ms timeout
	}
}

// Stop 停止 MqttStrategy
func (b *MqttStrategy) Stop() {
	b.logger.Info("Requesting stop for MQTTStrategy via context cancel")
	if b.cancel != nil {
		b.cancel()
	}
	// Disconnect is now handled at the end of Start's loop to ensure messages are processed.
}
