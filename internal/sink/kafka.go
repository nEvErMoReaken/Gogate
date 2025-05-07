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
				log.Error("为 Kafka 配置创建 mapstructure 解码器失败", zap.Error(err))
				return nil, fmt.Errorf("创建 Kafka 配置解码器失败: %w", err)
			}

			if err := decoder.Decode(strategyConfig.Para); err != nil {
				log.Error("解码 Kafka 配置出错", zap.Error(err), zap.Any("config", strategyConfig.Para))
				return nil, fmt.Errorf("解码 Kafka 配置出错: %w", err)
			}
			found = true
			break // 假设只有一个 kafka 配置块
		}
	}

	if !found {
		log.Warn("未找到或未启用 Kafka 策略配置")
		return nil, fmt.Errorf("未找到或未启用 Kafka 策略配置")
	}

	// 验证必填字段
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("Kafka 配置验证失败: 'brokers' 是必需的")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("Kafka 配置验证失败: 'topic' 是必需的")
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

	ks.logger.Info("Kafka 策略已初始化",
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
	ks.logger.Info("===KafkaStrategy 已启动===")

	defer func() {
		err := ks.writer.Close()
		if err != nil {
			ks.logger.Error("无法干净地关闭 Kafka writer", zap.Error(err))
		}
		ks.logger.Info("Kafka writer 已关闭")
	}()

OuterLoop:
	for {
		select {
		case <-ks.ctx.Done():
			ks.logger.Info("===KafkaStrategy 正在停止===")
			break OuterLoop
		case pointPackage, ok := <-pointChan:
			if !ok {
				ks.logger.Info("输入通道已关闭，正在停止 KafkaStrategy")
				break OuterLoop
			}

			if pointPackage == nil || len(pointPackage.Points) == 0 {
				continue // 跳过空的包
			}

			// 记录接收到的点计数（使用包中点的数量）
			metrics.IncMsgReceived("kafka_strategy")

			// 为整个发送操作创建计时器
			sendTimer := metrics.NewTimer("kafka_strategy_send_batch")

			// 准备 Kafka 批处理的消息
			messages := make([]kafka.Message, 0, len(pointPackage.Points))
			for _, point := range pointPackage.Points {
				if point == nil {
					continue
				}

				// 创建包含时间戳和字段的 map 用于 JSON 序列化
				payload := map[string]interface{}{
					"device": point.Device,               // 包含设备名称
					"ts":     pointPackage.Ts.UnixNano(), // 使用包中的时间戳
					"fields": point.Field,                // 字段已经是 map[string]interface{}
				}

				jsonData, err := json.Marshal(payload)
				if err != nil {
					metrics.IncErrorCount()
					metrics.IncMsgErrors("kafka_strategy_json_marshal")
					ks.logger.Error("在批处理中将点序列化为 JSON 失败", zap.Error(err), zap.Any("point_device", point.Device))
					continue // 跳过此点
				}
				messages = append(messages, kafka.Message{
					// 可选地设置 Key 用于分区（例如，point.Device）
					// Key:   []byte(point.Device),
					Value: jsonData,
					Time:  pointPackage.Ts, // 使用包中的时间戳
				})
			}

			if len(messages) == 0 {
				sendTimer.Stop() // 即使没有发送消息也停止计时器
				continue         // 所有点序列化失败
			}

			// 发送批次
			err := ks.writer.WriteMessages(ks.ctx, messages...)

			sendDuration := sendTimer.StopAndLog(ks.logger) // 停止计时器并记录持续时间

			if err != nil {
				if ks.ctx.Err() != nil {
					// 上下文取消，可能是在关闭过程中，记录为警告或信息
					ks.logger.Warn("Kafka 写入上下文已取消", zap.Error(ks.ctx.Err()))
					// 对于正常关闭不增加错误计数
					continue // 如果上下文已取消，则跳过增加错误计数
				}
				metrics.IncErrorCount()
				metrics.IncMsgErrors("kafka_strategy_write_messages")
				ks.logger.Error("向 Kafka 写入批消息失败", zap.Error(err), zap.Int("batch_size", len(messages)), zap.Duration("duration", sendDuration))
			} else {
				// 记录成功处理的点计数
				metrics.IncMsgProcessed("kafka_strategy")
				ks.logger.Debug("批次已发送到 Kafka", zap.Int("count", len(messages)), zap.Duration("duration", sendDuration))
			}
		}
	}
	ks.logger.Info("===KafkaStrategy 已完成===")
}

// Stop 优雅地停止 KafkaStrategy（通过上下文取消调用）
// 实际清理在 Start 中的 defer 函数中完成
func (ks *KafkaStrategy) Stop() {
	ks.logger.Info("通过上下文取消请求停止 KafkaStrategy")
	ks.cancel()
}
