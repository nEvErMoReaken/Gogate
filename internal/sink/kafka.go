package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/internal/pkg" // 假设 Point 和 PointPackage 类型在这里
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// 初始化时注册 Kafka 策略
func init() {
	Register("kafka", NewKafkaStrategy)
}

// KafkaSinkConfig 包含 Kafka Sink 特定的配置
type KafkaSinkConfig struct {
	Brokers []string `mapstructure:"brokers"`
	Topic   string   `mapstructure:"topic"`
	// 可根据需要添加其他 Kafka 生产者设置
	// 例如 Async, WriteTimeoutSec, RequiredAcks 等
	Async           bool `mapstructure:"async"`
	WriteTimeoutSec int  `mapstructure:"writeTimeoutSec"`
	ReadTimeoutSec  int  `mapstructure:"readTimeoutSec"`
	RequiredAcks    int  `mapstructure:"requiredAcks"` // 使用 int 类型以提高兼容性
}

// KafkaStrategy 实现了 Template 接口，用于将数据发送到 Kafka
type KafkaStrategy struct {
	writer *kafka.Writer
	config KafkaSinkConfig // 存储配置以便潜在的复用（例如，GetType）
	logger *zap.Logger
	ctx    context.Context
	cancel context.CancelFunc
}

// NewKafkaStrategy 是创建 KafkaStrategy 的工厂函数
func NewKafkaStrategy(ctx context.Context) (Template, error) {
	log := pkg.LoggerFromContext(ctx)
	config := pkg.ConfigFromContext(ctx)
	var cfg KafkaSinkConfig
	found := false

	// 查找 kafka 策略配置
	for _, strategyConfig := range config.Strategy {
		if strategyConfig.Enable && strategyConfig.Type == "kafka" {
			// 使用 mapstructure 进行稳健的解码
			decoderConfig := &mapstructure.DecoderConfig{
				Metadata: nil,
				Result:   &cfg,
				TagName:  "mapstructure",
				// 如果需要为缺失字段设置默认值，添加 ZeroFields = true
			}
			decoder, err := mapstructure.NewDecoder(decoderConfig)
			if err != nil {
				log.Error("Failed to create mapstructure decoder for Kafka config", zap.Error(err))
				return nil, fmt.Errorf("failed to create Kafka config decoder: %w", err)
			}

			if err := decoder.Decode(strategyConfig.Para); err != nil {
				log.Error("Error decoding Kafka config", zap.Error(err), zap.Any("config", strategyConfig.Para))
				return nil, fmt.Errorf("error decoding Kafka config: %w", err)
			}
			found = true
			break // 假设只有一个 kafka 配置块
		}
	}

	if !found {
		log.Warn("No enabled Kafka strategy configuration found")
		return nil, fmt.Errorf("no enabled Kafka strategy configuration found")
	}

	// 验证必填字段
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka config validation failed: 'brokers' is required")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("kafka config validation failed: 'topic' is required")
	}

	// 未提供时设置默认值
	if cfg.WriteTimeoutSec == 0 {
		cfg.WriteTimeoutSec = 10
	}
	if cfg.ReadTimeoutSec == 0 {
		cfg.ReadTimeoutSec = 10
	}
	// 如果未指定或值无效，默认为 RequireOne
	acks := kafka.RequireOne
	if cfg.RequiredAcks == -1 { // 所有 ISR
		acks = kafka.RequireAll
	} else if cfg.RequiredAcks == 0 { // 不需要确认
		acks = kafka.RequireNone
	}

	// 配置 Kafka writer
	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafka.LeastBytes{}, // 或者 kafka.RoundRobin, kafka.Hash 等
		WriteTimeout: time.Duration(cfg.WriteTimeoutSec) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeoutSec) * time.Second,
		RequiredAcks: kafka.RequiredAcks(acks),
		Async:        cfg.Async, // 使用配置的值
		// 根据需要考虑添加 Compression, BatchSize, BatchTimeout
	}

	// 使用特定于此策略的可取消上下文
	strategyCtx, cancel := context.WithCancel(ctx)

	ks := &KafkaStrategy{
		writer: writer,
		config: cfg, // 存储解码后的配置
		logger: log.With(zap.String("sink_type", "kafka"), zap.String("topic", cfg.Topic)),
		ctx:    strategyCtx,
		cancel: cancel,
	}

	ks.logger.Info("Kafka strategy initialized",
		zap.Strings("brokers", cfg.Brokers),
		zap.Bool("async", cfg.Async),
		zap.Int("acks", int(acks)),
	)

	// 如果 Async 为 true，考虑启动一个错误处理 goroutine
	// go ks.handleAsyncErrors()

	return ks, nil
}

// GetType 返回策略的类型
func (ks *KafkaStrategy) GetType() string {
	return "kafka"
}

// Start 开始监听通道并将数据发送到 Kafka
func (ks *KafkaStrategy) Start(pointChan chan *pkg.PointPackage) {
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例
	ks.logger.Info("===KafkaStrategy Started===")

	defer func() {
		err := ks.writer.Close()
		if err != nil {
			ks.logger.Error("Failed to close Kafka writer cleanly", zap.Error(err))
		}
		ks.logger.Info("Kafka writer closed")
	}()

OuterLoop:
	for {
		select {
		case <-ks.ctx.Done():
			ks.logger.Info("===KafkaStrategy Stopping===")
			break OuterLoop
		case pointPackage, ok := <-pointChan:
			if !ok {
				ks.logger.Info("Input channel closed, stopping KafkaStrategy")
				break OuterLoop
			}

			if pointPackage == nil || len(pointPackage.Points) == 0 {
				ks.logger.Debug("Skipping nil or empty point package")
				continue
			}

			// 记录接收到的点计数（使用包中点的数量）
			metrics.IncMsgReceived("kafka_strategy")

			// 为整个发送操作创建计时器
			sendTimer := metrics.NewTimer("kafka_strategy_send_batch")

			// 准备 Kafka 批处理的消息
			messages := make([]kafka.Message, 0, len(pointPackage.Points))
			var firstPointTags map[string]any // For logging context on error

			for _, point := range pointPackage.Points {
				if point == nil {
					ks.logger.Warn("Skipping nil point within package", zap.String("frameId", pointPackage.FrameId))
					continue
				}
				if firstPointTags == nil {
					firstPointTags = point.Tag // Store tags of the first valid point for logging
				}

				// New Payload Structure
				payload := map[string]interface{}{
					"tags":   point.Tag,                  // Include all tags (map[string]any)
					"fields": point.Field,                // Include all fields (map[string]interface{})
					"ts":     pointPackage.Ts.UnixNano(), // Timestamp from package
				}

				jsonData, err := json.Marshal(payload)
				if err != nil {
					metrics.IncErrorCount()
					metrics.IncMsgErrors("kafka_strategy_json_marshal")
					// Log with point tags for context if available
					ks.logger.Error("Failed to marshal point to JSON in batch", zap.Error(err), zap.Any("point_tags", point.Tag), zap.String("frameId", pointPackage.FrameId))
					continue
				}

				// Determine Kafka Message Key (e.g., from 'id' tag)
				var messageKey []byte
				if idVal, ok := point.Tag["id"]; ok {
					if idStr, isStr := idVal.(string); isStr {
						messageKey = []byte(idStr)
					}
				}

				messages = append(messages, kafka.Message{
					Key:   messageKey, // Use tag 'id' as key if available and string
					Value: jsonData,
					Time:  pointPackage.Ts,
				})
			}

			if len(messages) == 0 {
				ks.logger.Warn("No valid messages generated from point package", zap.String("frameId", pointPackage.FrameId), zap.Any("first_point_tags", firstPointTags))
				sendTimer.Stop() // Stop timer even if nothing sent
				continue
			}

			// 发送批次
			err := ks.writer.WriteMessages(ks.ctx, messages...)

			sendDuration := sendTimer.StopAndLog(ks.logger) // 停止计时器并记录持续时间

			if err != nil {
				if ks.ctx.Err() != nil {
					// 上下文取消，可能是在关闭过程中，记录为警告或信息
					ks.logger.Warn("Kafka write context canceled, likely during shutdown", zap.Error(ks.ctx.Err()), zap.String("frameId", pointPackage.FrameId))
					// 对于正常关闭不增加错误计数
					continue // 如果上下文已取消，则跳过增加错误计数
				}
				metrics.IncErrorCount()
				metrics.IncMsgErrors("kafka_strategy_write_messages")
				ks.logger.Error("Failed to write message batch to Kafka", zap.Error(err), zap.Int("batch_size", len(messages)), zap.Duration("duration", sendDuration), zap.String("frameId", pointPackage.FrameId), zap.Any("first_point_tags", firstPointTags))
			} else {
				// 记录成功处理的点计数
				metrics.IncMsgProcessed("kafka_strategy")
				ks.logger.Debug("Batch sent to Kafka successfully", zap.Int("count", len(messages)), zap.Duration("duration", sendDuration), zap.String("frameId", pointPackage.FrameId))
			}
		}
	}
	ks.logger.Info("===KafkaStrategy Finished===")
}

// Stop 优雅地停止 KafkaStrategy（通过上下文取消调用）
// 实际清理在 Start 中的 defer 函数中完成
func (ks *KafkaStrategy) Stop() {
	ks.logger.Info("Requesting stop for KafkaStrategy via context cancel")
	ks.cancel()
}
