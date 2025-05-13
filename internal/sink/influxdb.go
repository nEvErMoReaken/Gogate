package sink

import (
	"context"
	"fmt"
	"gateway/internal/pkg"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

// 拓展数据源步骤
func init() {
	// 注册发送策略
	Register("influxdb", NewInfluxDbStrategy)
}

// InfluxDbInfo InfluxDB的专属配置
type InfluxDbInfo struct {
	URL         string   `mapstructure:"url"`
	Org         string   `mapstructure:"org"`
	Token       string   `mapstructure:"token"`
	Bucket      string   `mapstructure:"bucket"`
	Measurement string   `mapstructure:"measurement"`
	BatchSize   uint     `mapstructure:"batch_size"`
	Tags        []string `mapstructure:"tags"`
}

// InfluxDbStrategy 实现将数据发布到 InfluxDB 的逻辑
type InfluxDbStrategy struct {
	client   influxdb2.Client
	writeAPI api.WriteAPI
	info     InfluxDbInfo
	ctx      context.Context
	logger   *zap.Logger
}

// NewInfluxDbStrategy Step.0 构造函数
func NewInfluxDbStrategy(ctx context.Context) (Template, error) {
	config := pkg.ConfigFromContext(ctx)
	logger := pkg.LoggerFromContext(ctx)

	var info InfluxDbInfo
	foundConfig := false
	for _, strategyConfig := range config.Strategy {
		if strategyConfig.Enable && strategyConfig.Type == "influxdb" {
			// 将 map 转换为结构体
			if err := mapstructure.Decode(strategyConfig.Para, &info); err != nil {
				return nil, fmt.Errorf("[NewInfluxDbStrategy] Error decoding map to struct: %v", err)
			}
			foundConfig = true
			break // Assuming only one influxdb config section
		}
	}

	if !foundConfig {
		return nil, fmt.Errorf("[NewInfluxDbStrategy] No enabled influxdb strategy configuration found")
	}

	// 检查 BatchSize 是否为零或未设置，如果是，使用默认值
	if info.BatchSize == 0 {
		info.BatchSize = 100 // 使用默认的批处理大小
		logger.Info("InfluxDB batch size not configured, using default", zap.Uint("batch_size", info.BatchSize))
	}
	// 新增：检查 Measurement 是否为空，并设置默认值
	if info.Measurement == "" {
		info.Measurement = "gateway_points" // 默认 measurement 名称
		logger.Info("InfluxDB measurement not configured, using default", zap.String("measurement", info.Measurement))
	}

	logger.Debug("InfluxDB 配置加载完毕", zap.Any("info", info))
	client := influxdb2.NewClientWithOptions(info.URL, info.Token, influxdb2.DefaultOptions().SetBatchSize(info.BatchSize))
	// 获取写入 API
	writeAPI := client.WriteAPI(info.Org, info.Bucket)
	// Get errors channel
	errorsCh := writeAPI.Errors()
	// Create go proc for reading and logging errors
	go func() {
		for err := range errorsCh {
			logger.Error("InfluxDB write error", zap.Error(err)) // Updated log message
			// 不清楚Influxdb通道内的错误是否会导致程序退出，所以这里不直接返回错误
		}
	}()
	return &InfluxDbStrategy{
		logger:   logger,
		client:   client,
		writeAPI: writeAPI,
		info:     info,
		ctx:      ctx,
	}, nil
}

// GetType Step.1
func (b *InfluxDbStrategy) GetType() string {
	return "influxdb"
}

// Start Step.2
func (b *InfluxDbStrategy) Start(sink chan *pkg.PointPackage) {
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	defer b.client.Close()
	b.logger.Info("InfluxDB Strategy started", zap.String("measurement", b.info.Measurement), zap.String("bucket", b.info.Bucket))
	for {
		select {
		case <-b.ctx.Done():
			b.Stop()
			b.logger.Info("InfluxDB Strategy stopped")
			return
		case pointPackage := <-sink:
			if pointPackage == nil {
				b.logger.Warn("Received nil point package, skipping")
				continue
			}
			// 创建发布计时器
			publishTimer := metrics.NewTimer("influxdb_publish")

			// 记录接收的点包
			metrics.IncMsgReceived("influxdb")

			err := b.Publish(pointPackage)

			if err != nil {
				metrics.IncErrorCount()
				metrics.IncMsgErrors("influxdb")
				// Log the error, but don't push to ErrChan as Publish handles logging internally
				b.logger.Error("Error publishing to InfluxDB", zap.String("frameId", pointPackage.FrameId), zap.Error(err))
			} else {
				// 记录成功处理的点包 (Note: metrics track packages now, not individual points)
				metrics.IncMsgProcessed("influxdb")
			}

			publishTimer.StopAndLog(b.logger)
		}
	}
}

func (b *InfluxDbStrategy) Publish(pointPackage *pkg.PointPackage) error {
	if pointPackage == nil || len(pointPackage.Points) == 0 {
		b.logger.Warn("Publish called with nil or empty point package")
		return nil // Nothing to publish
	}

	b.logger.Debug("Publishing point package to InfluxDB", zap.String("frameId", pointPackage.FrameId), zap.Int("pointCount", len(pointPackage.Points)))

	pointsWritten := 0
	for _, point := range pointPackage.Points {
		if point == nil {
			b.logger.Warn("Skipping nil point within package", zap.String("frameId", pointPackage.FrameId))
			continue
		}

		tagsMap := make(map[string]string)
		fieldsMap := make(map[string]interface{})

		// Populate tags from point.Tag
		if point.Tag != nil {
			for key, value := range point.Tag {
				tagsMap[key] = fmt.Sprintf("%v", value) // Convert tag value to string
			}
		} else {
			b.logger.Warn("Point is missing Tag map", zap.String("frameId", pointPackage.FrameId))
			tagsMap["error"] = "missing_tags" // Add an error tag if needed
		}

		// Populate fields from point.Field
		if point.Field != nil {
			for key, valuePtr := range point.Field {
				if valuePtr != nil { // Check for nil pointers/values in fields
					fieldsMap[key] = valuePtr
				}
			}
		} else {
			b.logger.Warn("Point is missing Field map", zap.String("frameId", pointPackage.FrameId), zap.Any("tags", tagsMap))
		}

		// Skip writing if there are no fields
		if len(fieldsMap) == 0 {
			b.logger.Warn("Skipping point write to InfluxDB due to empty fields", zap.String("frameId", pointPackage.FrameId), zap.Any("tags", tagsMap))
			continue
		}

		// Create InfluxDB point
		p := influxdb2.NewPoint(
			b.info.Measurement, // Use configured measurement
			tagsMap,            // Tags from point.Tag
			fieldsMap,          // Fields from point.Field
			pointPackage.Ts,    // Timestamp from the package
		)

		// Write point (asynchronously via batching API)
		b.writeAPI.WritePoint(p)
		pointsWritten++

		// Reduce log verbosity, maybe log only first point or summary later
		// b.logger.Info("InfluxDB point prepared",
		// 	zap.String("measurement", b.info.Measurement),
		// 	zap.Any("tags", tagsMap),
		// 	zap.Int("fieldCount", len(fieldsMap)),
		// 	zap.String("frameId", pointPackage.FrameId))
	}

	if pointsWritten > 0 {
		b.logger.Debug("Points prepared for InfluxDB batch write",
			zap.Int("pointsWritten", pointsWritten),
			zap.Int("totalInPackage", len(pointPackage.Points)),
			zap.String("measurement", b.info.Measurement),
			zap.String("frameId", pointPackage.FrameId))
	} else {
		b.logger.Warn("No valid points were written from the package", zap.String("frameId", pointPackage.FrameId))
	}

	return nil
}

// Stop 停止 InfluxDBStrategy
func (b *InfluxDbStrategy) Stop() {
	b.writeAPI.Flush() // 确保所有数据被写入
	b.client.Close()   // 关闭 InfluxDB 客户端
}
